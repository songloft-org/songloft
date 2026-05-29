# Songloft v2.0 Release Playbook

> 面向**项目维护者**的 Phase 6 切换操作手册。读者：你自己（hanxi）。
> 本文档假设 Phase 0/1/2/3/4 的代码改造已经全部 commit 在各仓库的 `v2.0` 分支（本地未 push）。

## 现状盘点

| 仓库（当前名） | v2.0 分支已 commit | 目标名 |
|--------------|------------------|------|
| `mimusic-org/mimusic` | ✅ | `songloft-org/songloft` |
| `mimusic-org/mimusic-player` | ✅ | `songloft-org/songloft-player` |
| `mimusic-org/plugin-toolchain` | ✅ | `songloft-org/plugin-toolchain`（仅 transfer，不改名）|
| `mimusic-org/jsplugin-musicsdk` | ✅ | `songloft-org/jsplugin-musicsdk`（仅 transfer，不改名）|
| `mimusic-org/mimusic-jsplugin-miot` | ✅ | `songloft-org/songloft-plugin-miot` |
| ~~`mimusic-org/jsplugins`~~ | ~~~~ | ~~`songloft-org/jsplugins`~~ —— 已删除。Songloft v2.0 起取消聚合仓库，每个插件在自己仓库分发 release |
| `github.com/hanxi/tag` (pkg/tag) | ✅ | 保持不变（hanxi 个人 fork，不归 mimusic-org 也不 transfer 到 songloft-org）|

⚠️ **冲突警告**：`songloft-org/songloft` 已经作为 placeholder 仓库存在（你之前抢名时建的，描述 "Coming in MiMusic v2.0 as Songloft"）。直接 transfer-rename 会冲突，必须先 rename / archive placeholder。

---

## Step 1 — 处理 placeholder 冲突

```bash
# 把已建的 placeholder 改名让出位置（不删除，作历史保留）
gh repo rename songloft -R songloft-org/songloft songloft-placeholder-2026
```

或者用 web UI：`https://github.com/songloft-org/songloft/settings` → Rename。

---

## Step 2 — 在 `mimusic-org` 下 rename 那些需要改名的仓库

在 `mimusic-org` 下先 rename，让远端仓库名与本地子模块名一致。Rename 后 GitHub 自动设置 redirect。

```bash
gh repo rename songloft -R mimusic-org/mimusic
gh repo rename songloft-player -R mimusic-org/mimusic-player
gh repo rename songloft-plugin-miot -R mimusic-org/mimusic-jsplugin-miot
```

---

## Step 3 — Transfer 6 个仓库到 `songloft-org`

> ⚠️ **transfer 是高敏感的不可逆操作**：所有 issue / PR / star / fork / wiki 都会跟着 transfer。GitHub 会自动设置老地址 → 新地址的 redirect（保留至少几年），所以现有 clone / curl URL 都会继续 work。
>
> 操作权限：你必须同时是 `mimusic-org` 的 owner 和 `songloft-org` 的 owner。

`gh repo transfer` 子命令在 `gh` CLI 里**不存在**，只能走 REST API：

```bash
# 直接用项目根脚本，按"先小后大"顺序自动跑
bash scripts/transfer-repos-to-songloft-org.sh           # dry-run
bash scripts/transfer-repos-to-songloft-org.sh apply     # 实际 transfer
```

如果想手工跑某一个：

```bash
gh api -X POST repos/mimusic-org/plugin-toolchain/transfer -f new_owner=songloft-org
```

**也可以直接用 web UI**（一个一个手工做 6 次）：每个仓库的 Settings → "Danger Zone" → "Transfer ownership"，输入 `songloft-org` + 仓库名确认。Web UI 多一道二次确认，比脚本更难误操作。

---

## Step 4 — 本地 git remote 切到新地址

仓库地址变了，本地 clone 必须切 remote。用项目根的 helper 脚本：

```bash
bash scripts/update-remotes-for-v2.sh
```

脚本会：
1. 更新主仓库 `origin` → `git@github.com:songloft-org/songloft.git`
2. 更新每个子模块 `origin` 到对应的 `songloft-org/*` 新地址
3. 跑 `git remote -v` 列出所有 remote 验证

⚠️ `pkg/tag` 的 remote 不动（它是 hanxi/tag fork，不 transfer）。

---

## Step 5 — Push v2.0 分支到新远端

**严格按子模块在前 / 主仓库在后**的顺序：

```bash
# 5.1 - 各子模块的 v2.0 分支（顺序无关，可并行）
(cd plugin-toolchain && git push -u origin v2.0)
(cd jsplugins-src/jsplugin-musicsdk && git push -u origin v2.0)
(cd songloft-player && git push -u origin v2.0)
(cd jsplugins-src/songloft-plugin-miot && git push -u origin v2.0)
(cd pkg/tag && git push -u origin v2.0)    # pkg/tag 推到 hanxi/tag 远端

# 5.2 - 主仓库 v2.0（必须在所有子模块 push 之后）
git push -u origin v2.0
```

---

## Step 6 — 老仓库 deprecation banner

GitHub redirect 已经自动生效，但搜索引擎 + 收藏的链接会继续指向老 URL。在老地址留一个明显的 deprecation banner 让访问者知道项目搬家了。

由于仓库已被 transfer，访问 `https://github.com/mimusic-org/mimusic` 会自动 redirect 到 `https://github.com/songloft-org/songloft`。**所以这一步不必做** —— 除非你想做 SEO 优化，可以在 songloft-org/songloft 的 README 里加一句"前身是 mimusic-org/mimusic"提示。

---

## Step 7 — Docker Hub 切换

```bash
# 7.1 - Songloft 镜像首次 build + push
make docker-build VERSION=2.0.0-alpha.1
docker tag songloft:2.0.0-alpha.1 songloft/songloft:2.0.0-alpha.1
docker push songloft/songloft:2.0.0-alpha.1

# 7.2 - 老 hanxi/mimusic 镜像加 deprecation
# 在 Docker Hub web UI: https://hub.docker.com/repository/docker/hanxi/mimusic/general
# 在 Description 顶部加：
#   ⚠️ Deprecated: This image has been renamed to songloft/songloft.
#   See https://github.com/songloft-org/songloft for the v2.0 migration guide.
```

---

## Step 8 — npm publish 链路 (Phase 5)

只有 plugin-toolchain v2.0 push 之后，才能跑 npm publish：

```bash
cd plugin-toolchain
git checkout v2.0
pnpm install
pnpm -r build
# 用 changesets 或 scripts/release.mjs 发 alpha
node scripts/release.mjs major --tag=alpha   # 第一次发 2.0.0-alpha.1
# 确认无误后实际 publish
pnpm -r publish --tag alpha --access public
```

⚠️ 旧的 `@mimusic/*` 系列发 deprecation 提示（不要 unpublish，npm 政策上 unpublish 受限且会破坏老插件）：

```bash
npm deprecate '@mimusic/plugin-sdk@>=0.9.0' \
  "Package renamed to @songloft/plugin-sdk. See https://github.com/songloft-org/songloft/blob/main/MIGRATION.md"
npm deprecate '@mimusic/plugin-builder@>=0.9.0' \
  "Package renamed to @songloft/plugin-builder. See https://github.com/songloft-org/songloft/blob/main/MIGRATION.md"
npm deprecate 'create-mimusic-plugin@>=0.9.0' \
  "Package renamed to create-songloft-plugin. See https://github.com/songloft-org/songloft/blob/main/MIGRATION.md"
npm deprecate '@mimusic/musicsdk@>=1.1.0' \
  "Package renamed to @songloft/musicsdk. See https://github.com/songloft-org/songloft/blob/main/MIGRATION.md"
# 还有 jsc 系列 6 个 platform 子包，按需 deprecate
```

---

## Step 9 — 合 v2.0 到 main

经过 RC 公测稳定之后：

```bash
# 在主仓库
git checkout main
git merge --no-ff v2.0 -m "feat: Songloft v2.0 (formerly MiMusic)"
git push origin main

# 各子模块同样
```

或者**重命名分支**：`git branch -m main main-v1 && git branch -m v2.0 main`，然后强制 update 默认分支（GitHub 仓库 Settings → Branches → Default branch）。

---

## 校验清单（每步做完都过一遍）

- [ ] `gh repo view songloft-org/songloft` 显示新仓库 + 描述正确
- [ ] `gh repo view mimusic-org/mimusic` 显示 "This repository has been transferred" 或 404 redirect
- [ ] `git remote -v` 在主仓库和所有子模块都显示 `songloft-org/*` URL
- [ ] `git push -u origin v2.0` 全部成功，无 `permission denied` / `not found`
- [ ] `gh release list -R songloft-org/songloft` 看不到任何泄漏的 v1.x tag（如果有，作为 archive 保留即可）
- [ ] `docker pull songloft/songloft:2.0.0-alpha.1` 拉得到
- [ ] `npm view @songloft/plugin-sdk@2.0.0-alpha.1` 拉得到
- [ ] 主仓库 README 顶部 v2.0 banner 移除（已经是 v2.0 自身了，不再需要 "未来更名" 提示）

---

## 回滚预案

如果 Phase 6 切换后发现致命问题：

1. **GitHub transfer 反向回去**：transfer 可以反向走，重新 transfer 到 mimusic-org。redirect 会反向。
2. **Docker 镜像**：停推 songloft/songloft，继续在 hanxi/mimusic 上推修复版本。
3. **npm**：unpublish 24 小时内可做（72 小时窗口内 npm 还允许撤回），超过则只能发新 patch 修复。
4. **本地 git**：所有改动都在 `v2.0` 分支，main 不动；废弃 v2.0 分支即可回到 v1.x 状态。

⚠️ **Phase 6 切换务必在至少 2 周 RC 稳定期之后进行**。Phase 5 的 alpha/beta/rc 是用来发现这种致命问题的安全网。
