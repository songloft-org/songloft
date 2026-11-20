# 文档站开发说明

本站使用 VitePress 自定义主题。页面内容来源分为手工文档、落地页结构化数据和构建时同步 / 生成的页面。修改前先确认来源，并同步中英文。

## 修改前判断

1. 首页 / 落地页 → 改 `.vitepress/data/*.ts`
2. `quick-start` / `changelog` → 改根 `README` / `CHANGELOG`
3. 子模块同步页 → 改子模块源仓库，再回主仓库 bump 指针
4. `repowiki` → 直接改入库的 Markdown
5. 其余手工页 → 改对应中英文文件

## 本地预览与构建

```bash
cd docs
npm ci
npm run docs:dev

# 验证生产构建后预览产物
npm run docs:build
npm run docs:preview
```

`docs:dev` 默认监听 `http://localhost:3030`。`docs:dev` / `docs:build` 依次执行 `sync`、`fetch-issues` 和 `swagger-to-md`，再启动 VitePress。预览已构建页面使用 `docs:preview`；不要同时运行开发服务和生产构建，让开发侧文件监听处理 `.vitepress/dist` 的大批变化。

## 内容来源

| 页面 / 文件 | 来源 | 修改位置 |
| --- | --- | --- |
| `index.md / en/index.md` | 自定义落地页 | `.vitepress/data/*.ts`、`.vitepress/theme/components/landing/*.vue` |
| `faq.md / architecture*.md / clients.md / repowiki/**` | 手工维护 Markdown | 对应中英文文件 / Corresponding Chinese and English files |
| `quick-start.md / en/quick-start.md` | 同步 | 根 README.md / README.en.md |
| `changelog.md` | 同步 | 根 CHANGELOG.md |
| `NOTICE.md / PRIVACY.md / TERMS.md 及英文版` | 同步 | 根目录对应声明 / Corresponding root statements |
| `player/** / en/player/** / player-lynx/** / en/player-lynx/**` | 客户端子模块同步 | Flutter 的 `docs/cn`、`docs/en` 及 Lynx 公共指南，映射见 `sync-docs.mjs` |
| `addon/** / plugin-toolchain/**` | 子模块同步 | `sync-docs.mjs` 中的源文件 |
| `swagger.json / swagger.yaml / docs.go` | swag 生成，入库 | handler 注释 + make swagger / Handler annotations + make swagger |
| `swagger-api/**` | 构建时生成，忽略 | docs/swagger.json → scripts/swagger-to-md.mjs |
| `issues/**` | GitHub 读取生成，忽略 | 带「文档」标签的 Issue / Issues labeled 文档 |

落地页 `index.md` 仅挂载 `<Landing />`，安装入口在 `downloads.ts`，功能在 `features.ts`，公共文案在 `landing-i18n.ts`；数据的 `{zh,en}` 字段需同步。

`repowiki` 是入库且手工维护的 Markdown，仓内没有重新生成它的脚本。代码变更后直接更新相关页面；不要把它与忽略的 Swagger API 页面混为一类。

子模块文档应改源仓库，再更新主仓库指针。`sync-docs.mjs` 的 `to:` 目标路径与源路径**刻意解耦**（如 `integrations/home-assistant/README.md` → `docs/addon/index.md`），因为 `/addon/` 等对外 URL 已进 sitemap，不能跟随源仓库改名。未 checkout 子模块时同步脚本仅警告，相关页面会**静默消失**；CI 的子模块初始化清单见 `.github/workflows/static.yml`，必须包含这些子模块。

## 同步与生成脚本

- `scripts/sync-docs.mjs`：同步根文档和子模块文档，并重写相对链接；公开目标 URL 与源路径可不同。
- `scripts/fetch-issues.mjs`：读取 GitHub Issue；本地未设置 `VITE_GITHUB_ISSUES_TOKEN` 时清空旧生成页面并跳过。
- `scripts/swagger-to-md.mjs`：将入库的 Swagger JSON 转为忽略的 API Markdown；新增接口先改 handler 注释并运行 `make swagger`。

这些预处理步骤必须在 VitePress 扫描页面之前执行。具体忽略范围以 `docs/.gitignore` 为准，不能把所有生成文件都视为不入库。
