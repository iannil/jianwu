/**
 * waitlist API 测试 — 模拟 Cloudflare Pages Function 环境
 *
 * 用法: node test-waitlist.mjs
 *
 * 测试覆盖：
 *   ✓ 无效邮箱 → 400
 *   ✓ 缺少邮箱 → 400
 *   ✓ 无效 JSON → 400
 *   ✓ 非 POST → 405
 *   ✓ RESEND_API_KEY 缺失 → 500
 *   ✓ 有效请求 → 调用 Resend API（没有真实密钥时走到 catch）
 *   ✓ CORS preflight → 204
 */

import { onRequest } from './functions/api/waitlist.js';

// ── 模拟 Request / env ──

function mockEnv(overrides = {}) {
  return {
    RESEND_API_KEY: overrides.RESEND_API_KEY ?? 're_test_key',
    TURNSTILE_SECRET_KEY: overrides.TURNSTILE_SECRET_KEY ?? '1x00000000000000000000BB',
    WAITLIST_EMAIL: overrides.WAITLIST_EMAIL,
    WAITLIST_FROM: overrides.WAITLIST_FROM,
  };
}

function mockRequest(method, body) {
  return {
    method,
    headers: new Map([['Content-Type', 'application/json']]),
    json: async () => body,
  };
}

// ── 辅助 ──

let passed = 0;
let failed = 0;

// 保存原始 fetch
const originalFetch = globalThis.fetch;

async function test(name, fn) {
  try {
    // 每次测试恢复原始 fetch（防止 mock 泄漏）
    globalThis.fetch = originalFetch;
    await fn();
    passed++;
    console.log(`  ✅ ${name}`);
  } catch (e) {
    failed++;
    console.log(`  ❌ ${name}: ${e.message}`);
  }
}

function assertStatus(resp, expected) {
  if (resp.status !== expected) {
    throw new Error(`期望状态 ${expected}，实际 ${resp.status}`);
  }
}

async function assertJson(resp, checks) {
  const data = await resp.json();
  for (const [key, val] of Object.entries(checks)) {
    if (data[key] !== val) {
      throw new Error(`期望 ${JSON.stringify(checks)}，实际 ${JSON.stringify(data)}`);
    }
  }
}

// ── 测试用例 ──

console.log('\n📋 waitlist API 测试\n');

// 1. CORS preflight
await test('OPTIONS → 204', async () => {
  const resp = await onRequest({ request: mockRequest('OPTIONS', {}), env: mockEnv() });
  assertStatus(resp, 204);
});

// 2. 非 POST → 405
await test('GET → 405', async () => {
  const resp = await onRequest({ request: mockRequest('GET', {}), env: mockEnv() });
  assertStatus(resp, 405);
  await assertJson(resp, { ok: false });
});

// 3. 无效 JSON → 400
await test('无效 JSON → 400', async () => {
  const req = {
    method: 'POST',
    headers: new Map([['Content-Type', 'application/json']]),
    json: async () => { throw new Error('invalid json'); },
  };
  const resp = await onRequest({ request: req, env: mockEnv() });
  assertStatus(resp, 400);
  await assertJson(resp, { ok: false });
});

// 4. 缺少邮箱 → 400
await test('缺少 email → 400', async () => {
  const resp = await onRequest({
    request: mockRequest('POST', {}),
    env: mockEnv(),
  });
  assertStatus(resp, 400);
  await assertJson(resp, { ok: false, error: '请提供有效的邮箱地址' });
});

// 5. 空邮箱 → 400
await test('空 email → 400', async () => {
  const resp = await onRequest({
    request: mockRequest('POST', { email: '' }),
    env: mockEnv(),
  });
  assertStatus(resp, 400);
  await assertJson(resp, { ok: false });
});

// 6. 无效邮箱格式 → 400
await test('无效邮箱格式 → 400', async () => {
  const resp = await onRequest({
    request: mockRequest('POST', { email: 'not-an-email' }),
    env: mockEnv(),
  });
  assertStatus(resp, 400);
  await assertJson(resp, { ok: false });
});

// 7. RESEND_API_KEY 缺失 → 500
await test('缺少 RESEND_API_KEY → 500', async () => {
  // Mock Turnstile 校验为通过
  globalThis.fetch = async (url) => {
    if (url === 'https://challenges.cloudflare.com/turnstile/v0/siteverify') {
      return new Response(JSON.stringify({ success: true }));
    }
    return originalFetch(url);
  };
  const resp = await onRequest({
    request: mockRequest('POST', { email: 'test@example.com', turnstileToken: 'mock-token' }),
    env: mockEnv({ RESEND_API_KEY: '' }),
  });
  assertStatus(resp, 500);
  await assertJson(resp, { ok: false });
});

// 8. Turnstile token 缺失 → 400
await test('缺少 turnstileToken → 400', async () => {
  const resp = await onRequest({
    request: mockRequest('POST', { email: 'test@example.com' }),
    env: mockEnv(),
  });
  assertStatus(resp, 400);
  await assertJson(resp, { ok: false, error: '请完成安全验证' });
});

// 9. TURNSTILE_SECRET_KEY 未配置 → 跳过 Turnstile 校验
await test('未配置 TURNSTILE_SECRET_KEY → 跳过校验，尝试发送', async () => {
  const resp = await onRequest({
    request: mockRequest('POST', { email: 'test@example.com' }),
    env: mockEnv({ TURNSTILE_SECRET_KEY: '' }),
  });
  // 没有 TURNSTILE_SECRET_KEY → 跳过校验 → 走到 Resend（没有真实 key 所以 502）
  assertStatus(resp, 502);
  await assertJson(resp, { ok: false });
});

// 10. 有效请求 + Turnstile → 调用 Resend API
await test('有效请求 + Turnstile → 尝试调用 Resend API', async () => {
  // Mock Turnstile 校验为通过
  globalThis.fetch = async (url) => {
    if (url === 'https://challenges.cloudflare.com/turnstile/v0/siteverify') {
      return new Response(JSON.stringify({ success: true }));
    }
    return originalFetch(url);
  };
  const resp = await onRequest({
    request: mockRequest('POST', { email: 'test@example.com', turnstileToken: 'mock-token' }),
    env: mockEnv({ RESEND_API_KEY: 're_test_key' }),
  });
  // Turnstile 通过 → Resend 401 → 502
  assertStatus(resp, 502);
  await assertJson(resp, { ok: false });
});

// ── 汇总 ──

console.log(`\n📊 结果: ${passed} 通过, ${failed} 失败, 共 ${passed + failed} 项\n`);
process.exit(failed > 0 ? 1 : 0);
