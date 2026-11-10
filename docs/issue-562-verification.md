# #562 实施与验证记录

2026-11-11，Linux amd64，Go 1.26.0、Node 22.17.1、Flutter 3.41.5，Docker browserless Chrome 121。范围为 [songloft-org/songloft#562](https://github.com/songloft-org/songloft/issues/562) 的来源分类、同名、封面与布局。[English](en/issue-562-verification.md)。

## 验证对象

以下验证最初针对未提交工作树；随后用户明确授权提交、推送、更新 gitlink 及回复 Issue。下表记录测试基线与源码指纹，交付提交见文末。指纹依文件名和内容排序计算，只包含已变更的非 Markdown 普通文件，排除目录/gitlink；完整对象记录为 `/tmp/songloft-562-implementation/worktrees.json`。

下表为同日审核修复后的源码指纹。首轮全量测试与浏览器证据对应审核前对象，保存在 `worktrees-before-review.json`；审核修复后的验证单列于文末。

| 仓库路径 | HEAD | 源码差异 SHA-256 |
| --- | --- | --- |
| `.` | `22cd87edf308b53ad6301e0b8a418bd8a6e784a2` | `22b06392a880a41b75de04ef89efbe284d0fd755cb55c671667b39836708ed2e` |
| `clients/player-lynx` | `2a18bb57819886a2c9b2c0476bdf51a127f09ae1` | `6cbe92b0516e9ab5c7383be751bb78609485262f8baa703414216054b8bf3716` |
| `clients/player` | `59bc9c7def13e73ced81df045cf77e3ce32f0426` | `3930d76bc9d5aba547d6d6981e70cbf7efbd9136d0f52c9e0a95b7ef97b62920` |
| `plugins/src/songloft-plugin-navidrome` | `04d87b0d79c880d1e648d056375a680e97d78802` | `0d75bf56ef73feedc160d9dda9c396653aea3355a064634ab398701c5ff1cec9` |
| `plugins/toolchain` | `0380621b2a7072b7d2b9c931c37f418d1ab77668` | `33b88431a426ea604bb8de4ecfd337d7eb80398f946ec889042a639e67cc5d51` |

## 实现

- 歌曲复用 `song_tags/song_tag_links`；歌单复用 `labels` 的 `tag:<id>`，无新关联表。GET/PUT 歌单标签、按标签分页过滤、独立可见歌单计数、插件桥接与 SDK 已补齐。全量替换保留系统标记；普通元数据更新保留当前标签引用。
- 删除标签与歌单引用、播放历史在同一 UnitOfWork 清理，歌曲关联外键级联。测试覆盖无效批次回滚、空歌单、隐藏计数、两类计数不相乘、ID 复用及旧对象写回。备份 v2 携带标签定义并按名称重映射 ID，接受 v1，保留原歌单同名合并语义。
- Navidrome 每轮按名解析来源及可选连接标签，同时标记歌单和导入歌曲。原名冲突时分配 `收藏-Navidrome`、`收藏-Navidrome (2)`；名称稳定，显式冲突报错，旧默认名仅在归属验证后迁移，手动名称保留，预览使用同一规则。
- 非空歌单提供封面端点；优先独立封面，再查询最多 20 首有封面线索的成员；远程请求共用 5 秒预算，支持插件相对地址和内部认证。成员封面强制重验证，失败尝试后续候选。
- Lynx/Flutter 的曲库、首页与详情接入共享标签；标签内分别浏览歌单和歌曲。歌单长标题复用滚动组件，纯机器同步标记只在展示层隐藏。手机封面 96px，数量文字增大，歌曲行收紧序号和两侧空白，时长进入副标题，保留操作菜单及点击区域。

## 首轮检查与边界

| 仓库 | 命令/场景 | 结果 |
| --- | --- | --- |
| 后端 | `go test -tags dev,lite ./...`，最终受影响四包重跑；`go vet -tags dev,lite ./...` | 通过 |
| 后端生成物 | `make sqlc`、`make swagger VERSION=2.13.9` | 通过；歌单 tags 路由和 `playlist_count` 已生成，未改宿主版本 |
| Lynx | `pnpm exec tsc -b`、`pnpm test`、`pnpm run build` | 通过；3,781 项；Lynx/Web 双产物 |
| Flutter | `flutter analyze --no-pub`、`flutter test --no-pub` | 通过；最终 821 项全量，首页补齐后 119 项、最终数量字号调整后 48 项相关检查亦通过 |
| Flutter Web | embedded Web，禁用 CDN，JavaScript 目标 | 通过；`--no-wasm-dry-run --no-web-resources-cdn`，真实 Chrome 首页／详情封面和来源标签通过 |
| Navidrome | format/typecheck/test/validate/check:format | 通过；29 项，包含命名稳定性、冲突、迁移及共享标签 |
| SDK | `pnpm --filter @songloft/plugin-sdk build`、`pnpm --filter './examples/*' build` | 通过；首次递归构建遇到 CLI bin 未链接，离线重装依赖后示例重跑通过 |
| 文档 | `node docs/node_modules/vitepress/bin/vitepress.js build docs` | 通过；中英设计、验证及 API 说明同步 |

最终 Flutter 全量采用 `--concurrency=2`。此前与多个构建并行时，音频中断测试有一次 30 秒超时；单独 17 项重跑及最终全量均通过。首轮中间 Web 构建的可选 Wasm 探测失败，JavaScript 构建成功；最终明确禁用 Wasm 探测，不声称已验证 Wasm 目标。

反向回归：用 Go overlay 仅去掉删除标签时的歌单引用清理，不改当前工作树；`TestPlaylistTagsPreserveFlagsRollbackAndDeleteReferences` 如预期失败，报告旧 `tag:1` 被写回。恢复实现后通过。日志 `/tmp/songloft-562-negative-tags.log`。

Lynx 真浏览器连接临时真实 SQLite/HTTP 后端：320px/390px 的封面均为 96px、数量 15px；歌曲标题可用宽度分别 142px/212px，收藏与更多均为 44px；1200px 保持 160px 封面。长标题 transform 随时间变化，无页面异常。实际打开管理标签、取消勾选、保存后用 GET 回读为空。Navidrome 构建页面的可选连接标签输入、提交与回填通过，API 为受控 fixture。

Flutter 最终 Web 产物在同一真实后端完成首页、详情及标签浏览：从曲库标签页打开 Navidrome，默认进入歌单分栏，实际发出 `GET /playlists?limit=30&offset=0&tag_id=1`，显示关联歌单及成员封面。三条流程均无页面异常，截图分别为 `flutter-home.png`、`flutter-detail.png`、`flutter-tags.png`。

证据位于 `/tmp/songloft-562-implementation/`：`lynx-mobile.png`、`lynx-320.png`、`lynx-1200.png`、`lynx-tags-browser.json`、`nav-mobile.png/json`、Flutter 截图与日志。浏览器证据归档前剔除临时 token；临时认证文件不作为证据。

## 同日代码审核与修复

确认并修复 4 项 P2 问题；均有修复前失败、修复后通过的行为回归：

1. **远程超时阻断本地回退**：独立远程封面或前序成员耗尽 5 秒预算时，原实现取消后续数据库读取／中断候选遍历，尽管存在本地图片仍返回 404。现在远程请求使用独立预算，本地读取保留原请求上下文并继续遍历。测试覆盖独立封面和成员两条路径。
2. **更换候选误返回 304**：移除首个成员后，如果下一幅图片文件修改时间更早，原实现会用旧 `If-Modified-Since` 判为未变化，客户端继续显示旧图。动态回退移除日期校验，保留包含文件身份的缩略图 ETag。
3. **删除标签残留客户端歌单缓存**：数据库已清理引用，但两端只刷新标签目录，缓存的 `tag:<id>` 可能在 ID 复用后解析成其他来源。Flutter 同时失效列表、详情、绑定缓存；Lynx 删除 mutation 刷新标签与歌单查询。实际删除 Widget 测试及真实 QueryClient 测试在修复前均读到旧 `tag:1`。
4. **歌单删除／显隐未刷新标签计数**：标签缓存中的可见歌单数不会随单个删除、批量删除、隐藏或恢复而更新，可能影响分类默认分栏。两端相应动作现同步失效标签列表与共享目录；四种操作在两端修复前均读取到旧计数，修复后通过。

修复前证据：`/tmp/songloft-562-review-before.log`（两条超时路径 404、候选切换 304）、`/tmp/songloft-562-review-flutter-before.log`、`/tmp/songloft-562-review-lynx-before.log`（旧缓存引用）。

计数回归的修复前日志为 `/tmp/songloft-562-review-counts-flutter-before.log`、`/tmp/songloft-562-review-counts-lynx-before.log`，每端四种操作均按预期失败。

审核后验证：`go test -tags dev,lite ./...` 和同标签 `go vet` 通过；Lynx 全量 357 文件、3,786 项测试及 `tsc -b` 通过；Flutter 曲库／歌单／首页网格 94 项测试及 analyze 通过。日志统一为 `/tmp/songloft-562-review-*.log`。Navidrome 与 SDK 源码未在审核中修改，沿用同指纹验证。

审核后 Lynx/Web 双产物和 Flutter embedded JavaScript Web 构建通过（后者 204.8 秒，输出 `/tmp/songloft-562-implementation/flutter-web-review`）。未重跑首轮整套浏览器流程；审核新增行为分别由真实 HTTP Handler、Widget 删除交互及 QueryClient 回归覆盖。

最终 `pnpm test --maxWorkers=4` 全量通过：357 文件、3,786 项，耗时 258.59 秒，日志 `/tmp/songloft-562-review-lynx-all-final.log`。审核文档构建、五仓库差异与 UTF-8 编码检查通过，最终源码指纹与表格一致。

Not run：报告人的荣耀 30 Pro / HarmonyOS 4 Bundle 真机、本地 Android/iOS/HarmonyOS 原生构建与设备交互、新一轮真实 Navidrome + QuickJS 全链路同步。现有自动化与浏览器结果不替代这些边界，不关闭 Issue。

## 提交与交付

2026-11-11，用户明确授权「提交推送回复 issue」。四个子仓库已提交并推送 main，父仓库本记录所在提交同步以下指针；未手动创建正式宿主版本。Navidrome main 推送会触发其既有自动插件发布，其余 CI/dev 构建按各仓库工作流执行。远端检查及可用产物以对应 SHA 的 GitHub Actions 和 Issue 回复为准，CI 构建不替代真机验收。

| 仓库 | 提交 |
| --- | --- |
| SDK | [75d22c4](https://github.com/songloft-org/plugin-toolchain/commit/75d22c4c655bf86f2c2417b0ae2fac4ed3953b69) |
| Navidrome | 源码 [a34e0b6](https://github.com/songloft-org/songloft-plugin-navidrome/commit/a34e0b6acebf20acdcb75ad6e69bc2a76570372c)，自动发布元数据／父指针 [f271903](https://github.com/songloft-org/songloft-plugin-navidrome/commit/f271903dd3cd5b688c627f61d4e7824673ff9230) |
| Flutter | [87a7699](https://github.com/songloft-org/songloft-player/commit/87a7699d2580b8c4463fa43e159d7bff18f6df29) |
| Lynx | [bc692c1](https://github.com/songloft-org/songloft-player-lynx/commit/bc692c14be23190aef4d3468e428dc292fd456af) |

提交前重新按测试基线计算源码指纹，全部与上表验证对象一致。Flutter main 额外保留 `65b7145` 的既有纯 CHANGELOG 提交，业务源码与测试对象一致。子仓库记录中的「未提交」描述验证时快照，当前交付以本节和 GitHub 为准。

Navidrome [发布 CI](https://github.com/songloft-org/songloft-plugin-navidrome/actions/runs/42876372384) 已通过并发布 [v2026.11.11.0](https://github.com/songloft-org/songloft-plugin-navidrome/releases/tag/v2026.11.11.0)。已核对 `releaseSource=a34e0b6…`；自动提交仅调整 `plugin.json`、`package.json`、锁文件的版本、下载地址和产物哈希，父指针采用该元数据提交，业务源码与已测源码相同。
