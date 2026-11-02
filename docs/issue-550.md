# 内置收藏歌单排序（songloft-org/songloft#550）

## 问题与修复

[Issue #550](https://github.com/songloft-org/songloft/issues/550) 报告 Docker 2.13.6 / Windows Flutter 客户端在收藏、电台收藏内手动排序后保存失败。截图界面与 Flutter 实现一致。

后端 `ReorderSongs`、`MoveSong` 和 `SortSongs` 把内置歌单身份保护用于成员位置更新，三个端点均返回 403。移除这三处身份限制，保留歌单存在性、完整列表长度、歌曲归属与事务更新检查；名称、类型、标签、隐藏和删除保护继续由原有路径处理。Swagger 同步说明内置歌单可排序。

Flutter 原有流程是加载完整歌单、提交 `song_ids`、保存成功后恢复自定义升序，所需修复位于后端。Lynx 另需开放内置歌单的手动排序入口，并修正跨行向下拖动时误选第一个位移条目的逻辑；手动排序以自定义升序为准。

## 验证对象与结果

- 时间：2026-11-02；Linux amd64，Go 1.26.0，Flutter 3.44.8 / Dart 3.12.2，Node 22.17.1，Docker Chrome 121。
- 主仓库基线：`71635ae30ec76a6a0a8bf4efa94304fe2276a057`；Flutter：`ae85c9570943c5f06ac77397dd384ef5907e2811`；Lynx：`a90cded03be18426de61c6df436d08a29e7a3077`。最终对象为上述基线加本次未提交工作树，指纹见 `/tmp/songloft-550-verification.json`。
- 旧实现的新增真实 SQLite / HTTP 回归：两个内置 ID × 手动全量排序、单首移动、名称双方向、数字前缀、随机排序，共 12 个场景均因 403 失败；修复后通过，校验持久位置、成员集合、内置身份与删除保护。
- `CGO_ENABLED=0 go test -p 2 -tags dev,lite ./...`、同标签 `go vet ./...`、`go test -race -tags dev,lite ./internal/handlers -run '^TestBuiltInPlaylistSongOrdering$'` 通过；`gofmt -w .`、`make swagger` 与 diff 检查通过。
- 首次全量 Go 测试中既有 `TestHLSTranscodeRealFFmpeg` 报 `playlist did not advance`；该测试独立重跑及降低包并发后的全量测试均通过。保留失败日志，不将首次失败改写为通过。
- 日志：`/tmp/songloft-550-{backend-before,backend-focused,backend-tests,backend-tests-final,backend-vet,backend-race,hls-retry,swagger}.log`。浏览器脚本、结果和截图：`/tmp/songloft-550-ui/`；客户端详细记录见 [Flutter](../clients/player/docs/cn/issue-550.md) 与 [Lynx](../clients/player-lynx/docs/project/issue-550.md)。

Not run：Windows 原生客户端、Android/iOS/HarmonyOS 真机与远端 CI。浏览器验收不替代原生设备验收。尚未提交、推送、发布或回复 Issue；后端必须升级，Lynx 入口修复需同步分发客户端。
