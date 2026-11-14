# Issue #567：跨域 HLS 资源代理验证

## 范围与代码

- 仓库 / Issue：songloft-org/songloft#567，https://github.com/songloft-org/songloft/issues/567。
- 基线：`6115afb969ff1f4b7ced0fbe39d223350adb9e5c`；开始时 `main` 工作树干净，远端 main 为同一 SHA。本文记录实现阶段的本地验证；后续提交、推送、CI 与回复状态以 Issue 中的交付记录为准。
- 环境 / 时间：2026-11-14，Linux 6.1 amd64，Go 1.26.0，FFmpeg 5.1.9，Docker HeadlessChrome 121 / hls.js 1.5.20。真实 SQLite 内存数据库、HTTP 服务及媒体解码；上游为本地构造的公开地址夹具，没有连接示例 IP 所属服务。
- 验证对象：基线加本次工作树。核心源码 SHA-256：

| 文件 | SHA-256 |
| --- | --- |
| `internal/handlers/hls.go` | `90704fc4561a7ba8c1725ec172baa92ac62ef7641f4e67e68023013d8378a129` |
| `internal/handlers/hls_test.go` | `13ef894d0abf2d0c6f78fe12d6dc684501a139805656d6fa14f4ce3432e85cf8` |
| `internal/handlers/hls_cross_origin_test.go` | `a7513e0e5838114d8c8e9ded70a6f94b8dd660a91a68904029f813547f84d453` |

完整工作树标识另由 `scripts/ai-review-context.mjs commits` 记录在 `/tmp/songloft-567/worktree-final.json`。日志、截图和隔离 overlay 均存 `/tmp/songloft-567/`，不含报告人的私有源地址。

## 根因与修复行为

旧 `rewriteRef` 只改写与上游播放列表同源的引用，端点又要求目标与 `song.URL` 同源。因此公网 IPTV 服务输出跨域 CDN 地址时，即使打开 HLS 代理，客户端仍直连这些资源；重定向后的相对资源也可能被端点拒绝。报告人的评论与此机制一致，但没有提供完整直播地址或导出日志，不能据此断言线上播放器失败的所有原因。

现将已识别的 HTTP(S) 资源（子列表、分片、key、init、LL-HLS）一并改写为相对代理链接，并附加 HMAC-SHA256 `sig`，绑定歌曲 ID、当前源 URL、端点类型及完整资源 URL。无签名请求保留原同源校验；篡改签名、URL、歌曲或端点返回 403，编辑源 URL 后旧签名失效。入口及每次重定向独立执行原私网拦截；不会套用通用资源代理的私网白名单。`data:` / `skd:` 等引用保留原文。

相对资源以重定向后的最终列表 URL 解析；保留部署前缀、`access_token`、上游时间窗口 query、HEAD、Range / 206 与字节级响应头。随机密钥只在进程内有效，重启后重新获取播放列表。

## 验证结果

| 命令 / 场景 | 结果与证据 |
| --- | --- |
| 旧代码 Go overlay，运行跨域子列表、切片/key、不同端口回归 | 按预期失败：资源仍是直连 URL；`old-overlay.log`。早期回归另见 `before.log` |
| 三个独立 overlay 移除签名、入口私网或重定向主机校验 | 均按预期失败，抓到未授权请求或额外上游访问；`signature-mutation.log`、`private-mutation.log`、`redirect-mutation.log` |
| `go test -tags dev,lite ./internal/handlers -run 'TestRewriteM3U8\|TestHLS\|TestIsSameOrigin' -count=1` | 通过；`after.log`。最终增强重定向路径断言后的源码由下列全量及 race 检查覆盖 |
| `go test -tags dev,lite ./...` | 通过；`tests-final.log` |
| `go test -race -tags dev,lite ./internal/handlers -run 'TestRewriteM3U8\|TestHLS\|TestIsSameOrigin' -count=1` | 通过；`race-final.log` |
| `go vet -tags dev,lite ./...` | 通过；`vet-final.log` |
| `make swagger VERSION=2.13.9` | 通过；`swagger.log`。两条 HLS 路径均生成 `sig` 参数及更新的 403 语义，三个生成文件同步 |
| `npm run docs:build`（docs 目录） | 通过；`docs-final.log`，保留现有 chunk 大小提示 |
| `gofmt -w .`、`git diff --check`、改动文件 UTF-8 检查 | 通过 |
| 真实 FFmpeg 经 HTTP 解码跨域 HLS | 通过，生成 2 秒 AAC/MPEG-TS，再经签名代理解码为至少 30,000 字节单声道 8 kHz PCM。初轮全量 / race 的夹具因无扩展名代理端点被 FFmpeg 拒绝；按工具支持情况设置 `allowed_extensions`、`allowed_segment_extensions` 和 `extension_picky` 后重跑通过。初轮日志保留为 `tests.log`、`race.log` |
| Docker Chrome 页面点击「播放测试流」 | 通过；独立播放夹具使用生产 HLS handler 和仓内 hls.js，未改业务 UI。`currentTime=1.010818`、`readyState=4`、`paused=false`，播放及页面错误为空，全部 HTTP 请求均指向本地 Songloft 测试服务；`browser.json`、`browser.png`、`browser-fixture.log` |

跨域 HTTP 夹具保留生产主机策略与重定向回调，仅将公开 IP 的 DialContext 定向到本地服务。覆盖主列表及子列表跨域重定向、最终路径下的相对资源、跨域 key / init / PART / 分片、未签名请求、签名及 URL 篡改、跨歌曲/端点复用、编辑源后失效，以及私网 / 回环 / IPv6 / 元数据地址和重定向拦截。

## Not run 与交付边界

- 报告人的真实 ysp-live 实例和 CCTV 直播：源地址被隐藏，未提供可复现部署或完整日志；仅验证相同代理机制。
- Songloft 完整客户端操作、Android/iOS 真机、视频画面及特定编码兼容性：本次未更改客户端，浏览器和 FFmpeg 使用合成音频夹具。
- 完整嵌入构建、打包、远端 CI、正式发布：本地验证阶段尚未提交或推送，本文覆盖 `dev,lite` 后端及文档构建。
- 内网 ysp-live 地址仍被 HLS 拒绝，即使写入通用私网白名单；这是保留的业务边界。公网跨域代理修复不能等同于支持任意内网 IPTV 源。

建议提交信息：`fix(hls): 为跨域播放资源生成签名代理链接 (#567)`。
