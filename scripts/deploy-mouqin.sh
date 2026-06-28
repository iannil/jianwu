#!/usr/bin/env bash
# =============================================================================
# scripts/deploy-mouqin.sh — 部署某钦 (mouqin.com) 到 Cloudflare Pages
#
# 用法:
#   ./scripts/deploy-mouqin.sh                # 部署到生产
#   ./scripts/deploy-mouqin.sh --branch=preview  # 部署到预览分支
#   ./scripts/deploy-mouqin.sh --dry-run         # 仅构建，不部署
#
# 前置条件:
#   - Hugo CLI         (brew install hugo)
#   - Wrangler CLI     (npm install -g wrangler)
#   - 环境变量 RESEND_API_KEY 已在 Cloudflare Pages Dashboard 设定
#     （首次部署后通过 wrangler secret put 或 Dashboard 设置）
#
# 首次部署需要:
#   1. Cloudflare Dashboard → Pages → 连接 GitHub 仓库
#   2. 按 docs/DEPLOY_MOUQIN.md 配置构建设置和域名
#   3. 之后可用此脚本快速部署
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
WEBSITE_DIR="$PROJECT_ROOT/website"

# ── 颜色 ──
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# ── 参数 ──
DRY_RUN=false
ENVIRONMENT="production"
PROJECT_NAME="mouqin"
BRANCH=""  # 延迟检测：用 --branch 传参则覆盖，否则取当前 git 分支

for arg in "$@"; do
  case "$arg" in
    --branch=*) BRANCH="${arg#*=}" ;;
    --dry-run)  DRY_RUN=true ;;
    --help|-h)
      echo "用法: $0 [--branch=<name>] [--dry-run]"
      echo ""
      echo "  默认：部署当前 git 分支"
      echo "    main     → 生产 (production)"
      echo "    其他分支 → 预览 (preview)"
      echo "  --branch=<name>  强制指定分支"
      echo "  --dry-run        仅构建 Hugo，不部署"
      exit 0
      ;;
  esac
done

# 检测当前 git 分支
if [ -z "$BRANCH" ]; then
  BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "main")"
fi

if [ "$BRANCH" != "main" ]; then
  ENVIRONMENT="preview"
fi

# ── 工具检查 ──
check_tool() {
  if ! command -v "$1" &>/dev/null; then
    echo -e "${RED}❌ 未找到 $1${NC}"
    echo "   请安装: $2"
    exit 1
  fi
}

echo -e "${CYAN}🔍 检查前置工具...${NC}"
check_tool "hugo"  "brew install hugo  或  https://gohugo.io/installation/"
check_tool "wrangler" "npm install -g wrangler"
check_tool "node"  "https://nodejs.org/ 或 nvm"

echo -e "${GREEN}✅ 前置工具就绪${NC}"
echo ""

# ── Hugo 构建 ──
echo -e "${CYAN}📦 构建 Hugo 静态站...${NC}"
cd "$WEBSITE_DIR"

# 加载环境变量供 Hugo 构建使用（如 HUGO_TURNSTILE_SITE_KEY）
if [ -f "$PROJECT_ROOT/.env.mouqin" ]; then
  set -a
  source "$PROJECT_ROOT/.env.mouqin"
  set +a
fi

hugo --cleanDestinationDir

echo -e "${GREEN}✅ Hugo 构建完成${NC}"
echo ""

# ── 环境变量检查（仅生产环境检查） ──
if [ "$ENVIRONMENT" = "production" ]; then
  echo -e "${YELLOW}ℹ️  确保 Cloudflare Pages 已设置环境变量:${NC}"
  echo "   - RESEND_API_KEY (必填)"
  echo "   - TURNSTILE_SECRET_KEY (推荐 — 反爬虫)"
  echo "   - WAITLIST_EMAIL (可选, 默认 hi@mouqin.com)"
  echo "   - WAITLIST_FROM  (可选, 默认 waitlist@mouqin.com)"
  echo ""

  # 检查本地是否有 .env 文件用于校验
  if [ -f "$PROJECT_ROOT/.env.mouqin" ]; then
    source "$PROJECT_ROOT/.env.mouqin"
    if [ -z "${RESEND_API_KEY:-}" ]; then
      echo -e "${RED}❌ .env.mouqin 中未设置 RESEND_API_KEY${NC}"
      echo "   请创建文件: echo 'RESEND_API_KEY=re_xxxx' > .env.mouqin"
      exit 1
    fi
    echo -e "${GREEN}✅ 本地环境变量检查通过${NC}"
  else
    echo -e "${YELLOW}⚠️  未找到 .env.mouqin 文件${NC}"
    echo "   首次部署请先通过 Cloudflare Dashboard 设置 RESEND_API_KEY"
    echo "   或创建本地 .env.mouqin 用于校验"
    echo ""
  fi
fi

# ── Dry-run: 只构建不部署 ──
if [ "$DRY_RUN" = true ]; then
  echo -e "${YELLOW}🏁 Dry-run 模式 — 构建完成，跳过部署${NC}"
  echo "   构建输出: $WEBSITE_DIR/public/"
  exit 0
fi

# ── 部署确认（生产环境） ──
if [ "$ENVIRONMENT" = "production" ] && [ "$DRY_RUN" = false ]; then
  echo ""
  echo -e "${YELLOW}⚠️  即将部署到生产环境 (mouqin.com)${NC}"
  echo -e "   分支: ${CYAN}$BRANCH${NC} @ $(git rev-parse --short HEAD 2>/dev/null)"
  echo -n "   确认部署？(y/N) "
  read -r CONFIRM
  if [ "$CONFIRM" != "y" ] && [ "$CONFIRM" != "Y" ]; then
    echo -e "${RED}❌ 取消部署${NC}"
    exit 0
  fi
  echo ""
fi

# ── 部署到 Cloudflare Pages ──
echo -e "${CYAN}🚀 部署到 Cloudflare Pages...${NC}"
echo "   项目:  $PROJECT_NAME"
echo "   分支:  $BRANCH $(git rev-parse --short HEAD 2>/dev/null)"
echo "   环境:  $ENVIRONMENT"
echo "   目录:  $WEBSITE_DIR/public"
echo ""

cd "$WEBSITE_DIR"

# 使用 wrangler pages deploy 直接部署
# 首次部署后，后续只需更新 public/ 目录
DEPLOY_OUTPUT=$(wrangler pages deploy public \
  --project-name="$PROJECT_NAME" \
  --branch="$BRANCH" \
  --commit-dirty=true \
  2>&1)

echo "$DEPLOY_OUTPUT"

# ── 提取部署 URL ──
DEPLOY_URL=$(echo "$DEPLOY_OUTPUT" | grep -oE 'https://[a-z0-9]+\.mouqin\.pages\.dev' | head -1)
if [ -n "$DEPLOY_URL" ]; then
  echo ""
  echo -e "${GREEN}✅ 部署成功!${NC}"
  echo -e "   预览地址: ${CYAN}$DEPLOY_URL${NC}"
  if [ "$ENVIRONMENT" = "production" ]; then
    echo -e "   生产域名: ${CYAN}https://mouqin.com${NC}（需配置 CNAME）"
  fi
fi

echo ""
echo -e "${GREEN}✅ 部署流程完成${NC}"
