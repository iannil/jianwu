# 某钦 (mouqin.com) — 部署指南

## 架构概览

```
用户 ──POST──→ mouqin.com/api/waitlist ──→ Resend API ──→ hi@mouqin.com
                      │
                  Cloudflare Pages
                  (Hugo 静态站 + Functions)
```

- **静态站：** Hugo 构建，部署到 Cloudflare Pages
- **表单后端：** Cloudflare Pages Functions（无服务器函数，零运维）
- **邮件通知：** Resend API 发送通知到 hi@mouqin.com

---

## 1. 注册 Resend 并获取 API 密钥

https://resend.com 注册 → 添加域名 `mouqin.com` → 按指引配置 DNS TXT 记录验证域名所有权 → 创建 API 密钥

> **注意：** Resend 需要验证域名才能从 `waitlist@mouqin.com` 发信。免费版每天 100 封，足够 waitlist 使用。

---

## 2. 部署到 Cloudflare Pages

### 2.1 连接 GitHub 仓库

1. 登录 [Cloudflare Dashboard](https://dash.cloudflare.com/)
2. **Workers & Pages** → **Pages** → **连接到 Git**
3. 选择 `zhurongshuo/jianwu` 仓库
4. 配置构建设置：

| 设置 | 值 |
|------|-----|
| **项目名称** | `mouqin` |
| **生产分支** | `main`（或你用的主分支） |
| **框架预设** | `Hugo` |
| **构建命令** | `hugo` |
| **构建输出目录** | `public` |
| **根目录** | `website` |

### 2.2 添加环境变量

在 Cloudflare Pages → `mouqin` → **设置** → **环境变量** → **生产环境**：

| 变量名 | 必填 | 值 |
|--------|------|-----|
| `RESEND_API_KEY` | ✅ | `re_xxxxxxxxxxxx`（你的 Resend API 密钥） |
| `TURNSTILE_SECRET_KEY` | ✅ | Cloudflare Turnstile 站点密钥（**未配置时 waitlist 返回 503 fail-closed**） |
| `WAITLIST_EMAIL` | – | 通知接收邮箱（默认 `hi@mouqin.com`） |
| `WAITLIST_FROM` | – | 发件人地址（默认 `waitlist@mouqin.com`） |

> **⚠️ Turnstile fail-closed：** v0.3 审计后，waitlist 函数在 `TURNSTILE_SECRET_KEY` 未配置时**直接返回 503**（不再静默跳过验证）。上线前必检此项。Turnstile 注册：https://dash.cloudflare.com → Turnstile → 添加站点（域名填 `mouqin.com`，模式选 Managed）→ 复制 Secret Key。

### 2.3 绑定 KV Namespace（限流）

为防止 Resend 配额被刷光（免费版 100/天），waitlist 函数用 KV 做 10 分钟 / IP / 最多 3 次的限流。

1. Cloudflare Dashboard → **Workers & Pages** → **KV** → 创建 namespace `WAITLIST_KV`
2. Pages → `mouqin` → **设置** → **Functions** → **KV namespace bindings**：
   - 变量名：`WAITLIST_KV`
   - KV namespace：选 `WAITLIST_KV`

> **未绑定 KV 时：** 函数记录 warn 日志、跳过限流、继续处理提交。**不**fail-closed（限流是滥用防控，不是安全防线）。

### 2.4 Functions 兼容性

Cloudflare Pages Functions 默认兼容最新 Workers 运行时。无需额外配置。

---

## 3. 绑定域名

1. Cloudflare Pages → `mouqin` → **自定义域**
2. 添加 `mouqin.com`
3. Cloudflare 会自动添加 DNS 记录并签发 SSL 证书（约 1 分钟生效）

> **前提：** `mouqin.com` 的 DNS 已托管在 Cloudflare（你已注册，确认 nameservers 指向 Cloudflare）。

---

## 4. Cloudflare Email Routing（可选 — 接收回复）

如果你希望 `hi@mouqin.com` 能收到邮件（不仅仅是 waitlist 通知）：

1. Cloudflare Dashboard → **Email** → **Email Routing**
2. 添加目标邮箱（如你的 Gmail/Outlook）
3. 创建路由规则：`hi@mouqin.com` → 你的个人邮箱

这样 `hi@mouqin.com` 可以接收 waitlist 通知的回复。

---

## 5. 验证部署

部署完成后：

1. 访问 https://mouqin.com — 应正确显示首页
2. 在 waitlist 表单输入邮箱并提交 — 应显示"感谢加入！"
3. 检查 `hi@mouqin.com` 收件箱 — 应收到通知邮件

---

## 文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `website/functions/api/waitlist.js` | **新增** | Cloudflare Pages Function — 处理表单提交，调 Resend API 发邮件 |
| `website/layouts/index.html` | **修改** | 表单加 `action="/api/waitlist"`，加 JS 异步提交和反馈 |
| `website/assets/css/main.css` | **修改** | 加 `.mq-form-feedback`、`.mq-form-row--loading` 等状态样式 |
| `website/package.json` | **新增** | 标记 Functions 目录给 Cloudflare Pages |

---

## 本地开发

```bash
# 构建静态站
cd website && hugo

# 本地预览 Pages Functions（需要 wrangler）
npx wrangler pages dev public --kv --compatibility-date=2025-01-01
```
