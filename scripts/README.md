# Songloft 脚本目录

本目录包含 Songloft 后端的版本管理、构建辅助、子模块同步等自动化脚本。多平台二进制构建、Docker 镜像打包、GitHub Release 创建已交由 [`.github/workflows/release.yml`](../.github/workflows/release.yml) 完成。

## 📋 脚本列表

| 脚本 | 作用 |
|------|------|
| `bump-version.sh` | 升级 `Makefile` 的 `VERSION` 与 `main.go` 的 Swagger `@version`，并 commit + tag + push（push tag 后由 release workflow 接管发布） |
| `submodule-update.sh` | 批量同步所有 git 子模块到最新 main |
| `docker-entrypoint.sh` | Docker 镜像内的启动入口（不直接调用） |
| `plugin-build.sh` | 构建单个 JS 插件，输出 `.jsplugin.zip` |
| `plugin-release.sh` | 把 `.jsplugin.zip` 上传到对应 GitHub Release |
| `sync-repowiki.sh` | 同步 Qoder repowiki 到 `docs/repowiki/` |
| `fetch-issues.mjs` / `sync-docs.mjs` | 文档同步辅助 |
| `test_tag.sh` | `pkg/tag` 命令行工具的手工冒烟脚本 |

## 🚀 发布流程

```bash
# 1. 本地升版号 + 打 tag + push
./scripts/bump-version.sh patch        # 1.4.1 → 1.4.2
# 或：make bump TYPE=patch
```

push tag 后，[`release.yml`](../.github/workflows/release.yml) 会自动：

1. 构建 7 平台二进制（Linux amd64/arm64/armv7、macOS amd64/arm64、Windows amd64/arm64）的 lite + full 版本
2. 构建并推送 Docker 多架构镜像（linux/amd64、linux/arm64、linux/arm/v7）到 Docker Hub
3. 生成 sha256 校验和
4. 创建 GitHub Release 并上传所有产物
5. 用 [`requarks/changelog-action`](https://github.com/requarks/changelog-action) 基于 Conventional Commits 生成 changelog，作为 Release Notes，并把内容追加到 `CHANGELOG.md` 顶部 commit 回 `main`

> 注意：当前 `release.yml` 默认仅 `workflow_dispatch` 触发（生成 prerelease，不写 `CHANGELOG.md`）。要让 push tag 自动跑全套 stable 流程并回写 CHANGELOG，需取消文件顶部 `push: tags: - 'v*'` 段的注释。

## 🔁 submodule-update.sh

批量同步所有子模块（`songloft-player` / `plugin-toolchain` / `pkg/tag` / `jsplugins-src/*` / `jsplugins` 等）到各自的 main：

```bash
./scripts/submodule-update.sh
```

## 🔌 JS 插件构建脚本

| 脚本 | 作用 |
|------|------|
| `plugin-build.sh <plugin-name>` | 进入 `jsplugins-src/<plugin-name>/`，运行 `pnpm install && pnpm run build`，把产物拷贝到 `jsplugins/` |
| `plugin-release.sh <plugin-name>` | 把对应的 `.jsplugin.zip` 上传到该插件子模块的 GitHub Release |

详细插件开发流程见 `plugin-toolchain/README.md`。

## 🧪 test_tag.sh

`pkg/tag` 提供的 `cmd/tag`、`cmd/sum`、`cmd/check` 命令行工具的人工冒烟测试，运行前需先 `go install ./pkg/tag/cmd/...`。

## 📍 仓库说明

- **代码 & 发布仓库**：https://github.com/songloft-org/songloft
- **Docker 镜像**：[songloft/songloft](https://hub.docker.com/r/songloft/songloft)（保持原命名空间）
