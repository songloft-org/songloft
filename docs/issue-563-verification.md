# #563 歌单成员变更事件验证

目标：[songloft-org/songloft#563](https://github.com/songloft-org/songloft/issues/563)。2026-11-11 完成实现与本地验证；下方保留提交前对象，交付状态见文末。

## 验证对象

- 主仓库基线：`c686994c13cb4a6611a29ef410b3f13236516158` + 本次工作树。
- 客户端基线：Flutter `87a7699d2580b8c4463fa43e159d7bff18f6df29`，Lynx `bc692c14be23190aef4d3468e428dc292fd456af`，均为验证时的基线加工作树；最终子提交见文末。
- SDK 仓库 `songloft-org/plugin-toolchain`：`75d22c4c655bf86f2c2417b0ae2fac4ed3953b69` + `packages/plugin-sdk/src/{global.d.ts,index.ts}` 提交前验证改动；最终发布提交见文末。
- 最终文件 SHA-256 清单：`/tmp/songloft-563-extra/worktree-final.json`；已有 `songloft-562-implementation/` 未修改，不纳入本次改动指纹。
- 环境：Linux amd64、Go 1.26.0、Node 22.17.1、pnpm 11.15.1；真实内存 SQLite 迁移、QuickJS、Chi HTTP 测试服务器。

## 行为与范围

新增 `songloft.events.onPlaylistSongsChange` / `offPlaylistSongsChange`，需要 `playlists.read`。事件含歌单 ID、歌曲 ID、add/remove、Unix 毫秒和歌名／艺术家快照；收藏与普通歌单共用。覆盖歌单服务的单首／批量增删及原子替换，客户端 HTTP 和插件桥接共用服务。事务回滚、跳过项、重复项和纯排序不发事件；部分批次提交后出错仍通知已提交部分。

订阅保留空闲唤醒能力，新 VM 重新校验权限并恢复处理器；显式取消、禁用和删除清理身份。单次操作按整批入队，覆盖成员数超过 actor 队列容量的情况。分发不等待插件网络回调；属于有界队列的尽力实时通知，不持久化、不重放。

接口用法与更完整边界见[插件指南](js-plugin-development-guide.md#songloftevents--歌单成员变更)。补充实现已增加认证长轮询端点 `/playlists/favorites/changes`、独立 `playlist_added_at` 字段及两客户端自动刷新，SQLC 与 Swagger 已重新生成；数据库 schema 不变。歌单／曲库删除的级联清理、备份恢复及本地转换内部关联替换不广播本事件。

## 核心事件首次检查（附加需求实现前）

| 命令／场景 | 结果与证据 |
| --- | --- |
| `CGO_ENABLED=0 go test -tags dev,lite ./...` | 通过；`/tmp/songloft-563/go-tests-delivery.log` |
| `CGO_ENABLED=0 go vet -tags dev,lite ./...` | 通过；`/tmp/songloft-563/go-vet-final.log` |
| 目标回归 race 检查（完整命令见下） | 四包三轮通过；`/tmp/songloft-563/events-race-final.log` |
| HTTP 收藏／普通歌单添加、移除 | Chi 真实 handler 验证事件和响应；重复输入仅一个 add |
| QuickJS 生命周期 | 真实 JS 注册、重复注册、取消、自身 HTTP 回调写歌单、顺序增删、空闲唤醒、禁用、删除清理、重载撤销权限、272 成员整批投递通过 |
| SQLite 事务 | 差集通知、空数组清空、回滚静默、部分批次失败及已提交 ID／计数通过 |
| 反向验证 | 去掉通知后缺少 add；恢复旧批量方法后错误返回 501，实际仅 500 行提交；去掉重载校验后继承撤销的权限。三个隔离 overlay 均按预期失败，见 `/tmp/songloft-563/negative-{notification,batch-baseline,reload}.log` |
| `node --test scripts/tests/*.test.mjs` | 59 项通过；`/tmp/songloft-563/scripts-tests.log` |
| SDK `pnpm --filter @songloft/plugin-sdk typecheck` / `build` | 均通过；另用 `/tmp/songloft-563/sdk-consumer.ts` 编译实际生成的导出声明，通过；`/tmp/songloft-563/sdk-consumer.log` |
| `npm run docs:build`（`docs/`） | 通过；`/tmp/songloft-563/docs-build-final.log` |
| `gofmt -w .`、两仓库 `git diff --check` | 通过 |

```bash
CGO_ENABLED=1 go test -race -tags dev,lite ./internal/jsplugin ./internal/services ./internal/database ./internal/handlers -run '^TestPlaylist(Membership|BatchChanges|Events)' -count=3
```

初次 race 检查因测试夹具每次持久化完整历史，处理大批量时发生二次方序列化，导致排队 HTTP 读超过 10 秒；没有报告数据竞争。夹具仅将短生命周期历史持久化，大批量投递在内存中断言；修正后四包三轮 race 通过。早期测试还修正了测试包循环导入及换包夹具错误重置哈希的问题，最终结果以以上命令为准。

## 未执行与交付

- 本次附加实现修改了 Flutter / Lynx 客户端；新增检查与实际 Web 操作见下方补充记录。Not run：移动／桌面真机、完整嵌入发行包。
- Not run：实际 Last.fm 账号的 love/unlove、npm 发布、远端 CI；未修改同步插件，也未获发布或推送授权。
- 建议提交：SDK `feat(sdk): 添加歌单成员变更事件类型（Ref songloft-org/songloft#563）`，主仓库 `feat(jsplugin): 广播已提交的歌单成员变更（Fixes #563）`。子仓库先提交，父仓库再更新指针。

## 2026-11-11 附加需求

- 歌单全量、分页、筛选、HTTP 与 QuickJS 完整／brief 成员结果返回 `playlist_added_at`，`Song.added_at` 保留曲库时间。原子替换保留已有成员行，只增删差集并重排位置，避免改写保留成员的加入时间。
- 认证 `GET /api/v1/playlists/favorites/changes` 接口首次／旧游标立即返回 `changed=true`；相同游标挂起最多 25 秒，收藏或电台收藏写入提交后立即唤醒所有等待客户端。游标含进程身份，重启后自动失效；断开、停止服务释放等待，无需事件持久日志。
- Flutter 在收藏 Provider 接入，Lynx 在应用级接入（覆盖全屏播放器）；收通知后读取完整 ID、更新红心，并失效内置歌单视图。会话变更拒绝迟到结果，后台暂停、恢复重新对账，网络失败退避、旧宿主 404 停止订阅。Flutter 还防止会话重置前的初始化恢复旧集合。
- 主仓库检查与证据：`/tmp/songloft-563-extra/go-tests.log`、`go-vet.log`、`go-race.log`；独立成员时间、保留成员时间、认证 401、多个等待者、取消、重启和关闭回归，Go 目标 race 三轮通过。
- 反向：将原子替换恢复为清空／重建，保留成员时间回归按预期失败；去掉收藏通知后内置收藏回归按预期失败。日志 `negative-time-final.log`、`negative-notify.log`。早期时间 overlay 范围错误导致编译失败，修正后才计作行为反向证据。
- SDK 类型检查与构建：`sdk-types.log`、`sdk-build.log`。SQLC / Swagger 生成及新字段、路由检索通过。所有本节日志位于 `/tmp/songloft-563-extra/`。
- 客户端最终验证：Flutter 3.44.8 / Dart 3.12.2，`flutter analyze` 无问题、`flutter test` 831 项通过；Web 使用 `flutter build web --profile --no-wasm-dry-run --no-web-resources-cdn --dart-define=DEPLOY_MODE=embedded --output=/tmp/songloft-563-extra/flutter-web` 构建通过。`flutter-{analyze-delivery,tests,build-delivery}.log`。Profile Web 验证不等于发行包／原生平台编译。
- Lynx：`pnpm exec tsc -b`、`pnpm test` 359 文件 / 3793 项通过；分别 `pnpm exec rspeedy build --environment lynx` 和 `pnpm run build:web` 完成 Lynx / Web 两目标。`lynx-{types-delivery,tests-delivery,native-build,web}.log`。
- Docker Chrome 121，1280×900，临时后端、临时认证与真实 `verify563` QuickJS 插件。插件 HTTP handler 实际调用 `playlists.addSongs/removeSongs`；同一个已打开页面不点红心、不刷新，Lynx 为 Add→Remove→Add，Flutter 为 Favorite→Unfavorite→Favorite。最终后端二进制为 `songloft-delivery`，UI JSON 证据 `lynx-ui-evidence-delivery.json`、`flutter-ui-evidence-delivery.json`；加入／移除截图 `{lynx,flutter}-delivery-{addedShot,removedShot}.png`，请求链接中的凭证已脱敏。最终文件及验证产物哈希见本批 `worktree-final.json`。
- 受影响模块的 Go 全量测试、vet 与目标四包 race 三轮均通过；SDK 类型／构建、SQLC / Swagger 与文档站构建通过。最终文档日志 `docs-build-delivery.log`。
- 中间失败如实保留：首次并行 Flutter / Lynx 构建内存压力下 Lynx 进程退出 137，Flutter 构建主动终止后改 Profile 串行验证；首次 Lynx 全量套件因失败构建缺少 `dist/main.lynx.bundle` 失败一项，成功构建后完整重跑 3793 项通过。首次 Flutter 真实页面脚本在默认不展示心形按钮的歌单行寻找按钮，后改为实际曲库行导航，未修改产品 UI。
- 客户端记录：Flutter `docs/cn/issue-563.md` / `docs/en/issue-563.md`；Lynx 新验证记录和 handoff 使用独立中英版本，既有 `docs/project/progress.md` 沿用单文件双语结构（未新建全量英文历史副本）。未提交、推送、发布或更新任何 gitlink。

## 提交、SDK 版本与交付

本会话随后获得主仓库／子仓库提交、推送、SDK 版本升级和回复 `songloft-org/songloft#563` 的明确授权。客户端正式版本升级与关闭 Issue 未在本次授权范围内。

- Flutter：[`c0cfa70`](https://github.com/songloft-org/songloft-player/commit/c0cfa70d4f5d85edc969dc71b258dc56308fb9e9)，主分支自动构建 dev 包。
- Lynx：[`84a1f56`](https://github.com/songloft-org/songloft-player-lynx/commit/84a1f5601382c4d17c0c50c97f134d9198c03a60)，主分支自动构建 dev 包。
- SDK：功能提交 `82b58c8`，发布提交 [`bc58da6`](https://github.com/songloft-org/plugin-toolchain/commit/bc58da63333521f2cb7192c44fcfffcf61656616)。执行 `node scripts/release.mjs minor --yes`，由 `2.18.0` 升为 `2.19.0` 并推送 [`v2.19.0`](https://github.com/songloft-org/plugin-toolchain/releases/tag/v2.19.0)。脚本同步升级其关联发布包；npm 可用状态以该版本 Release 工作流与 npm 为准。
- 以上子仓库先提交／推送，父仓库本次提交同步三个 gitlink；父提交使用 `Ref #563`，避免未经单独授权自动关闭 Issue。
- 推送后追踪相同 SHA 的必要 CI；最终回复列出实际发布／未发布情况与原生设备、Last.fm 账号尚未验证的边界。交付本地日志与最终仓库 SHA 清单位于 `/tmp/songloft-563-publish/`；提交前源文件指纹保持在前述 `worktree-final.json`。
