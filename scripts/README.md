# Songloft 脚本目录

本目录包含 Songloft 后端的版本管理、构建辅助、子模块同步等自动化脚本。多平台二进制构建、Docker 镜像打包、GitHub Release 创建已交由 [`.github/workflows/release.yml`](../.github/workflows/release.yml) 完成。

## 📋 脚本列表

| 脚本 | 作用 |
|------|------|
| `build-embedded-web.mjs` | 构建完整版 Lynx Web，隔离父/子仓库版本，应用引擎补丁并生成 Brotli/gzip 资源 |
| `release-all.sh` | 一键发布：Flutter 客户端 → Lynx 客户端 → 提交子模块引用 → 父仓库后端（同一组参数透传给三个 bump 脚本） |
| `bump-version.sh` | 升级 `Makefile` 的 `VERSION` 与 `main.go` 的 Swagger `@version`，commit + tag + 原子 push（push tag 后由 release workflow 接管发布）。与两个客户端仓库的 bump 脚本共享**统一发布契约** |
| `submodule-update.sh` | 批量同步所有 git 子模块到最新 main |
| `submodule-push.sh` | 批量推送所有有未推送 commit 的子模块到远程 |
| `docker-entrypoint.sh` | Docker 镜像内的启动入口（不直接调用） |
| `plugin-build.sh` | 构建单个 JS 插件，输出 `.jsplugin.zip` |
| `plugin-release.sh` | 把 `.jsplugin.zip` 上传到对应 GitHub Release |
| `generate-icons.sh` | 从 `favicon.svg` 生成项目所需的所有 PNG/ICO 图标（依赖 Node.js 18+） |
| `fetch-issues.mjs` / `sync-docs.mjs` | 文档同步辅助 |
| `test_tag.sh` | `pkg/tag` 命令行工具的手工冒烟脚本 |
| `ai-review-context.mjs` | 只读收集主仓库和递归子模块的清单、开放 Issues、提交与工作树标识；不完整时写报告并退出 1 |

## AI 开发取证

需要 Node.js 22+、Git；Issues 模式需要已登录的 `gh`。在父仓库执行，输出用工作树外的唯一文件名，已有文件不会覆盖：

```bash
node scripts/ai-review-context.mjs inventory --output /tmp/songloft-inventory-20261013-01.json
node scripts/ai-review-context.mjs issues --output /tmp/songloft-issues-20261013-01.json
node scripts/ai-review-context.mjs commits --since 2026-10-12T00:00:00+08:00 --until 2026-10-13T00:00:00+08:00 --output /tmp/songloft-review-20261013-01.json
node --test scripts/tests/ai-review-context.test.mjs
```

Issues 完整分页、排除 PR、保留负责人，按仓库去重。未初始化子模块仍查 Issues，但其嵌套清单未知时报告缺口。`complete=false` / 退出码 1 表示未全覆盖，不能当零待办。commits 只读本地历史，不证明远端同步；收集结果也不自动判定修复、更新经验或调度 AI。完整流程见 [AI 开发与复盘](../docs/ai-development.md)。

## 🚀 发布流程

### 统一发布契约

父仓库 `scripts/bump-version.sh`、`clients/player-flutter/scripts/bump-version.sh`、`clients/player-lynx/scripts/bump-version.sh` 三者参数与行为完全一致：

```bash
./scripts/bump-version.sh [major|minor|patch|release|X.Y.Z[-prerelease]] [--dry-run] [--yes] [--no-push]
```

- 预检先于任何写操作：`main` 分支、工作树干净、新版本高于当前版本、`v<新>` tag 本地与远程均不存在、本地包含 `origin/main`；任一失败即退出。
- `--dry-run` 只打印计划（零写入零联网）；`--yes` 跳过交互确认；`--no-push` 只留本地 commit/tag。
- commit → annotated tag → `git push --atomic` 一次推送 main 与该 tag。**已发布的 tag 永不覆盖**；失败后保留现场，不自动回滚。
- stdout 末行输出 `v<版本>`（日志走 stderr），方便链式调用。

### 一键发布（三个仓库）

```bash
./scripts/release-all.sh patch            # Flutter → Lynx → gitlink commit → 父仓库
./scripts/release-all.sh 2.14.0 --dry-run # 预览全部计划，零写入
```

顺序约束：子仓库脚本各自 commit+tag+push（触发各自 CI）→ 父仓库用独立 commit 收录两个 gitlink（pathspec 限定，不裹带无关改动）→ 父仓库 bump 脚本（要求干净树，故 gitlink 必须先落地）推送后端 tag。失败即停、不回滚。

版本计划与确认提示实时显示在 stderr。任一仓库取消确认（包括直接回车）会立即停止整条流程，不会跳过该仓库继续发布；此前已完成的步骤保留。子脚本未返回有效的 `v<版本>` 时也会中止。

### 仅发布父仓库

```bash
# 本地升版号 + 打 tag + push
./scripts/bump-version.sh patch        # x.y.z → x.y.(z+1)
# 或：make bump TYPE=patch
```

push tag 后，[`release.yml`](../.github/workflows/release.yml) 会自动：

1. 构建 7 平台二进制（Linux amd64/arm64/armv7、macOS amd64/arm64、Windows amd64/arm64）的 lite + full 版本
2. 构建并推送 Docker 多架构镜像（linux/amd64、linux/arm64、linux/arm/v7）到 Docker Hub
3. 生成 sha256 校验和
4. 创建 GitHub Release 并上传所有产物
5. 用 [`requarks/changelog-action`](https://github.com/requarks/changelog-action) 基于 Conventional Commits 生成 changelog，作为 Release Notes，并把内容追加到 `CHANGELOG.md` 顶部 commit 回 `main`

## 🔁 submodule-update.sh

批量同步所有子模块（`clients/player-flutter` / `plugins/toolchain` / `pkg/tag` / `plugins/src/*` / `jsplugins` 等）到各自的 main：

```bash
./scripts/submodule-update.sh
```

## 🔌 JS 插件构建脚本

| 脚本 | 作用 |
|------|------|
| `plugin-build.sh <plugin-name>` | 进入 `plugins/src/<plugin-name>/`，运行 `pnpm install && pnpm run build`，把产物拷贝到 `jsplugins/` |
| `plugin-release.sh <plugin-name>` | 把对应的 `.jsplugin.zip` 上传到该插件子模块的 GitHub Release |

详细插件开发流程见 `plugins/toolchain/README.md`。

## 🧪 test_tag.sh

`pkg/tag` 提供的 `cmd/tag`、`cmd/sum`、`cmd/check` 命令行工具的人工冒烟测试，运行前需先 `go install ./pkg/tag/cmd/...`。

## 📍 仓库说明

- **代码 & 发布仓库**：https://github.com/songloft-org/songloft
- **Docker 镜像**：[songloft/songloft](https://hub.docker.com/r/songloft/songloft)（保持原命名空间）
