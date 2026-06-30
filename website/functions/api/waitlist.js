/**
 * 某钦 waitlist — Cloudflare Pages Function
 *
 * POST /api/waitlist { "email": "...", "turnstileToken": "..." }
 *   → 通过 Resend API 发送通知邮件到 hi@mouqin.com
 *
 * 环境变量（在 Cloudflare Pages Dashboard 中设置）:
 *   RESEND_API_KEY        — Resend API 密钥（必填）
 *   TURNSTILE_SECRET_KEY  — Cloudflare Turnstile 密钥（**必填**，未配置则 503 fail-closed）
 *   WAITLIST_KV           — KV namespace 绑定（**推荐**，未绑定则跳过限流）
 *   WAITLIST_EMAIL        — 通知接收邮箱（可选，默认 hi@mouqin.com）
 *   WAITLIST_FROM         — 发件人地址（可选，默认 waitlist@mouqin.com）
 *
 * 安全防线（按顺序）：
 *   1. 邮箱格式校验
 *   2. KV 限流（10 分钟 / IP / 最多 3 次，防 Resend 配额烧光）
 *   3. Turnstile 人机校验（fail-closed：未配置 secret 直接 503）
 *   4. Resend API 调用
 *   5. 所有用户输入在 HTML 邮件体中 HTML-escape，防邮件 HTML 注入
 */

// 简单的邮箱格式校验
function isValidEmail(email) {
  if (typeof email !== 'string') return false;
  if (email.length > 320) return false;
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
}

// HTML 转义 — 防止用户输入注入到 HTML 邮件体
function escapeHtml(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

// CORS 头 — 允许同站表单提交
const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Methods': 'POST, OPTIONS',
  'Access-Control-Allow-Headers': 'Content-Type',
};

function jsonResponse(body, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      'Content-Type': 'application/json; charset=utf-8',
      ...corsHeaders,
    },
  });
}

// 10 分钟窗口内单 IP 最多提交次数
const RATE_LIMIT_MAX = 3;
const RATE_LIMIT_WINDOW_SECONDS = 600;

export async function onRequest(context) {
  const { request, env } = context;

  // OPTIONS → CORS preflight
  if (request.method === 'OPTIONS') {
    return new Response(null, { status: 204, headers: corsHeaders });
  }

  // 仅接受 POST
  if (request.method !== 'POST') {
    return jsonResponse({ ok: false, error: '仅接受 POST 请求' }, 405);
  }

  // 解析请求体
  let body;
  try {
    body = await request.json();
  } catch {
    return jsonResponse({ ok: false, error: '请求体必须是有效的 JSON' }, 400);
  }

  const { email, turnstileToken } = body;

  // 校验邮箱
  if (!email || !isValidEmail(email)) {
    return jsonResponse({ ok: false, error: '请提供有效的邮箱地址' }, 400);
  }

  // ── KV 限流（如绑定）──
  // 未绑定 WAITLIST_KV 时跳过（仅记录警告），不阻断服务
  if (env.WAITLIST_KV) {
    const ip = request.headers.get('CF-Connecting-IP') || 'unknown';
    const key = `waitlist:${ip}`;
    try {
      const raw = await env.WAITLIST_KV.get(key);
      const count = parseInt(raw || '0', 10);
      if (count >= RATE_LIMIT_MAX) {
        return jsonResponse(
          { ok: false, error: '提交过于频繁，请稍后再试' },
          429,
        );
      }
      await env.WAITLIST_KV.put(key, String(count + 1), {
        expirationTtl: RATE_LIMIT_WINDOW_SECONDS,
      });
    } catch (kvErr) {
      // KV 故障不应阻断正常提交，记录后继续
      console.error('KV 限流检查失败，跳过限流:', kvErr);
    }
  } else {
    console.warn('WAITLIST_KV 未绑定，跳过限流（建议在 Dashboard 绑定 KV namespace）');
  }

  // ── Turnstile 校验（fail-closed）──
  const turnstileSecret = env.TURNSTILE_SECRET_KEY;
  if (!turnstileSecret) {
    // fail-closed：未配置 secret 直接 503，避免静默退化为无验证
    console.error('TURNSTILE_SECRET_KEY 未配置 — fail closed');
    return jsonResponse(
      { ok: false, error: '安全验证未配置，请联系管理员' },
      503,
    );
  }

  if (!turnstileToken) {
    return jsonResponse({ ok: false, error: '请完成安全验证' }, 400);
  }

  const verifyRes = await fetch('https://challenges.cloudflare.com/turnstile/v0/siteverify', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      secret: turnstileSecret,
      response: turnstileToken,
    }),
  });

  const verifyData = await verifyRes.json();
  if (!verifyData.success) {
    console.error('Turnstile 验证失败:', verifyData['error-codes']);
    return jsonResponse({ ok: false, error: '安全验证未通过，请刷新后重试' }, 403);
  }

  // 获取配置
  const apiKey = env.RESEND_API_KEY;
  if (!apiKey) {
    console.error('RESEND_API_KEY 未配置');
    return jsonResponse({ ok: false, error: '服务器配置错误，请稍后再试' }, 500);
  }

  const toEmail = env.WAITLIST_EMAIL || 'hi@mouqin.com';
  const fromEmail = env.WAITLIST_FROM || 'waitlist@mouqin.com';

  // HTML-escape 所有用户输入字段后再拼到邮件体
  const safeEmail = escapeHtml(email);
  const mailtoUrl = encodeURIComponent(email);

  try {
    // 通过 Resend API 发送通知邮件
    const resendRes = await fetch('https://api.resend.com/emails', {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${apiKey}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        from: `某钦 <${fromEmail}>`,
        to: [toEmail],
        subject: '🎉 新 waitlist 注册',
        html: `
          <!DOCTYPE html>
          <html>
          <head><meta charset="utf-8"></head>
          <body style="font-family: system-ui, sans-serif; max-width: 480px; margin: 2rem auto; padding: 0 1rem;">
            <h1 style="font-size: 1.25rem; font-weight: 600; margin-bottom: 0.5rem;">🎉 新的 Waitlist 注册</h1>
            <p style="color: #374151; margin-bottom: 1.5rem;">
              有新的用户加入了 <strong>某钦 (mouqin)</strong> 的 waitlist：
            </p>

            <table style="border-collapse: collapse; width: 100%;">
              <tr>
                <td style="padding: 0.5rem 1rem 0.5rem 0; font-weight: 500; color: #6B7280; white-space: nowrap;">邮箱</td>
                <td style="padding: 0.5rem 0; font-family: monospace; color: #1F2937;">
                  <a href="mailto:${mailtoUrl}" style="color: #2563EB;">${safeEmail}</a>
                </td>
              </tr>
              <tr>
                <td style="padding: 0.5rem 1rem 0.5rem 0; font-weight: 500; color: #6B7280; white-space: nowrap;">时间</td>
                <td style="padding: 0.5rem 0; color: #1F2937;">${new Date().toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai' })}</td>
              </tr>
            </table>

            <hr style="margin: 2rem 0; border: none; border-top: 1px solid #E5E7EB;">

            <p style="font-size: 0.75rem; color: #9CA3AF;">
              此邮件由 某钦 waitlist 系统自动发送 · ${new Date().toISOString()}
            </p>
          </body>
          </html>
        `,
      }),
    });

    if (!resendRes.ok) {
      const errText = await resendRes.text();
      console.error('Resend API 错误:', resendRes.status, errText);
      return jsonResponse({ ok: false, error: '邮件发送失败，请稍后再试' }, 502);
    }

    return jsonResponse({ ok: true, message: '感谢加入！我们会尽快联系你。' });
  } catch (err) {
    console.error('waitlist 处理异常:', err);
    return jsonResponse({ ok: false, error: '服务器内部错误，请稍后再试' }, 500);
  }
}
