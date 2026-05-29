# 文档站开发说明

本文档面向希望在本地预览或修改 Songloft 文档站的开发者。

## 本地预览

```bash
cd docs
npm install
npm run docs:dev
```

默认监听 `http://localhost:3030`。`docs:dev` 会先执行 `sync`（把仓库根 `README.md` / `CHANGELOG.md` 同步为 `docs/quick-start.md` / `docs/changelog.md`）和 `fetch-issues`（从 GitHub 拉取带「文档」标签的 issues 生成 `docs/issues/*.md`），再启动 VitePress。

## 目录分类

`docs/` 下的 Markdown 文件分三类，请在修改前先判断文件类型：

| 文件 | 类型 | 如何修改 |
|------|------|---------|
| `index.md` | 源文件 | 直接编辑 |
| `faq.md` | 源文件 | 直接编辑 |
| `js-plugin-development-guide.md` | 源文件 | 直接编辑 |
| `swagger.json` | 手动维护 | 从主仓 `songloft/docs/swagger.json` 复制 |
| `quick-start.md` | 构建时生成 | 改仓库根的 `README.md` |
| `changelog.md` | 构建时生成 | 改仓库根的 `CHANGELOG.md` |
| `issues/*.md` | 运行时生成 | 改 GitHub issues（带「文档」标签） |

生成类文件全部被 `docs/.gitignore` 忽略，不要提交，也不要手动编辑。

## 同步脚本

- `scripts/sync-docs.mjs`：仓库根 Markdown → `docs/`。新增同步项在 `syncItems` 数组追加一行即可。
- `scripts/fetch-issues.mjs`：调用 GitHub API 抓取带「文档」标签的 issues，写入 `docs/issues/*.md`。依赖环境变量 `VITE_GITHUB_ISSUES_TOKEN`（CI 中由 secrets 注入）。本地若未设置该变量，脚本会清空 `docs/issues/` 后直接跳过，站点仍可构建但 issues 页面为空。

两个脚本都由 `docs/package.json` 串联到 `docs:dev` / `docs:build` 里，必须在 `vitepress build` **之前**执行，否则 VitePress 扫描不到新生成的页面。

