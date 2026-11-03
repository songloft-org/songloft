# 开发经验

保存主仓库与公共宿主契约的证据型经验；执行见 [AI 开发流程](docs/ai-development.md)。后续会话在相关任务前读取适用条目。客户端、插件和设备经验放所属子仓库。

## 记录与升级

- 每条含规则、范围、失败/修复证据、正确示例和独立事件日期；使用完整仓库引用与 commit SHA/链接。
- `Seen N×` 统计独立事件的不同日期，同一事故反复复盘不递增；三天后提出规范升级并标记权威位置。已有铁律、严重问题不等待三次。
- 不编造次数、修复状态或验证结果，与 [英文版](LESSONS.en.md) 同步，敏感信息脱敏。

## 条目

### CUE-001：传统编码不能按解码成功或出现汉字判定

- 范围：外部 CUE 编码检测，songloft-org/songloft#554；基线 `5a9bed40ec8542acf72f5afb71074a8046245214`。
- 证据：报告人的 CP950 / UTF-8 同内容样本在旧实现下产生不同的专辑、艺术家和 16 首曲目；旧 `tag.FixEncoding` 优先接受含汉字的 GBK 结果，甚至带有 U+FFFD。另一个回归确认同一 Big5 输入也能无替换字符地解码为不同的 GB18030 文本，仅拒绝替换字符仍不足。
- 规则与修复：合法 Unicode 保持原文；传统编码结合统计置信度和候选差距，解码结果另查替换字符与异常控制字符。无法判定时返回可操作的 UTF-8 转换提示，不用尝试顺序静默猜测。重新导入必须在成功解析后才删除旧记录。
- Done right：旧解析器的隔离 Go overlay 使新增解析与真实 SQLite 入库回归失败，修复后通过；实际临时后端 HTTP 扫描与重新导入核对全部 16 首曲目。最终对象、命令、日志和边界见 [验证记录](docs/issue-554.md)。
- Seen 1× (2026-11-03)。状态：本地修复与验证，未提交、推送或发布。

### DOC-001：仓库中存在的相对链接不一定是文档站路由

- 范围：主仓库 VitePress 文档引用子仓库验证记录，songloft-org/songloft#550；提交 `24b05df18c03630968fd0aead904d1f68a349809`。
- 证据：[Pages CI](https://github.com/songloft-org/songloft/actions/runs/41391858600) 报 4 个死链接。`../clients/...` 文件虽然在工作区存在，目标却不在 VitePress 文档根下，也未列入同步清单。
- 规则与修复：跨到未同步的子仓库文档时使用固定提交的 GitHub 源码链接；文件存在性检查不能替代完整文档构建，不关闭死链接校验。
- Done right：中英四个链接改为已推送子提交；执行 `npm run docs:build` 验证，日志 `/tmp/songloft-550-delivery-docs.log`；最终本地和 CI 结果以本批交付记录为准。
- Seen 1× (2026-11-02)。状态：文档链接修正，不改变排序代码。

### PLAYLIST-002：内置身份保护不能禁止成员排序

- 范围：歌单歌曲 reorder / move / sort 与两客户端，songloft-org/songloft#550；基线 `71635ae30ec76a6a0a8bf4efa94304fe2276a057`。
- 规则：分别核对歌单身份与成员操作的保护条件；允许调整收藏成员位置，同时保留内置歌单的名称、类型、标签、隐藏和删除约束。
- 证据与 Done right：新增真实 SQLite / HTTP 回归在旧代码下 12 项均因 403 失败；修复后三端点、持久顺序、成员集合、内置身份与删除保护通过。两客户端的真实拖动、后端位置和刷新读取分别核对，不能从普通歌单外推内置行为。最终对象、日志与未验设备见 [记录](docs/issue-550.md)。
- Seen 1× (2026-11-02)。状态：本地修复与验证，未提交、推送或发布。

### AUTH-001：永久设备凭证必须与登录会话区分身份

- 范围：认证中间件、Token 管理与两客户端永久访问 Token；songloft-org/songloft#546，基线 `022a56cebe4b4b6bc1ff7ca043f56d426bddaf71`。
- 风险核对（非线上事故）：现有 `/auth/tokens` 返回完整登录凭证，原路由只有通用认证。新增永久 Token 若直接复用通用认证即可调用该接口，会绕过「只能用正常登录会话管理 Token」的产品边界；仅保护新增创建接口不足。
- 规则与实现：认证服务从持久记录确定凭证类型，中间件把身份类别传入上下文；旧、新 Token 管理及退出登录路由都用 `RequireLoginSession`。永久凭证每次查库，不复用 JWT 的有效状态缓存，保证撤销影响后续请求。
- Done right：真实 SQLite 服务/HTTP 测试覆盖 Access/Refresh/插件/永久凭证、两种认证传法、跨实例立即撤销及重启。隔离 overlay 移除登录身份检查，`TestPersonalTokenHTTP` 因创建返回 201 而非 403 失败；移除撤销检查，生命周期测试因已撤销凭证仍被接受失败。正向及 race 通过，证据 `/tmp/songloft-546-negative-{session,revocation}.log`、`/tmp/songloft-546-go-race.log`，最终源码身份见 `/tmp/songloft-546-verification.json`。
- Seen 1× (2026-10-28)。状态：新功能的身份与撤销边界验证经验；提交、推送及 CI 按本批交付记录核对，真实设备边界见 [#546](docs/issue-546.md)。

### CI-002：ffprobe 包哈希必须按字段读取，不能截取整段 CSV

- 范围：`TestHLSTranscodeRealFFmpeg` 的视频复制校验；基线 `af5a4b3873ba72e9918ddf661da87ae563448d36`。
- 失败证据：2026-10-27，[Quality](https://github.com/songloft-org/songloft/actions/runs/40443632206) 在 Ubuntu / FFmpeg 6.1.1 报 `video packet 39 changed`；同一旧测试在本地 FFmpeg 5.1.7 重复十次通过。旧解析器从 CSV 的 `SHA256:` 截到行末，混入后续字段。真实源文件与首分片的第 39 包哈希相同，但源文件该包带 MPEG-TS Stream ID，分片末包不带；[ffprobe 6.1.1 源码](https://github.com/FFmpeg/FFmpeg/blob/n6.1.1/fftools/ffprobe.c)先输出 `data_hash` 再输出 side data，旧比较因此依赖输出布局。
- 规则与修复：用 JSON 的 `packets[].data_hash` 比较压缩载荷，保留包顺序、非空输出、SHA-256 字段有效性及逐包相等校验。CSV 附加元数据变化不能当成视频重编码证据；不跳过检查、不放宽比较，也不改 FFmpeg 业务参数。
- Done right：Linux amd64 / Go 1.26.0 / FFmpeg 5.1.7，`CGO_ENABLED=0 go test -tags dev,lite ./...`、同标签 vet、59 项脚本测试、格式及 diff 检查通过。隔离 Go overlay 将视频复制改为 libx264 重编码，新测试按预期因第 0 包哈希变化失败，证明仍能检测真实数据变化。日志、工具版本和版本对照证据位于 `/tmp/songloft-ci-hls-20261027/`；Ubuntu 6.1.1 及推送后同一 SHA 的 CI 结果按交付记录核对。
- Not run：Safari、真实 IPTV 源及真机播放；本次仅修正测试解析，未改变服务端播放行为。
- Seen 1× (2026-10-27)。状态：测试兼容性经验，与缺少 FFmpeg 依赖的 CI-001 分开记录。

### CI-001：媒体集成测试的外部工具必须在质量作业显式安装

- 范围：主仓库 Release/Quality、HLS 转码测试；songloft-org/songloft#541、songloft-org/songloft#542。
- 失败证据：2026-10-27，指针提交 `b4ceafab36c6d6ec6e29df260f95a081f8c0acd5` 的 [Quality](https://github.com/songloft-org/songloft/actions/runs/40440501499) 未安装 ffmpeg，HLS 失败/取消/关闭测试在探测上游前返回「ffmpeg 未配置」，真实媒体夹具也无法运行，后续打包全部跳过。
- 规则与修复：涉及真实 ffmpeg/ffprobe 的测试，CI 在运行测试前显式安装并输出两者版本；不能用扩充 Skip 或改预期状态码掩盖缺失依赖。`release.yml` 的 Quality 统一安装 Ubuntu ffmpeg 包。
- Done right：Linux amd64 / Go 1.26.0 / FFmpeg 5.1.7，`CGO_ENABLED=0 go test -tags dev,lite ./...`、对应 vet、59 项脚本测试及 actionlint v1.7.12 通过；Go 测试复用同源缓存。日志 `/tmp/songloft-542-quality-{go,vet,scripts}.log`、`/tmp/songloft-542-actionlint.log`。CI 新运行的实际安装、测试与打包状态以修复提交的 GitHub 记录为准。
- Seen 1× (2026-10-27)。状态：经验；本次只补 CI 依赖，不改 HLS 业务行为。

### HLS-001：按真实输入容器验证 FFmpeg 参数，探测成功不代表转码命令可用

- 范围：远程视频 HLS 转码，songloft-org/songloft#541；基线 `041d34d7e9387b60ed8126022071f731a7268aa6`。
- 失败证据：2026-10-26，初版将 HLS demuxer 的 `-allowed_extensions ALL` 放到所有输入命令；ffprobe 对真实 H.264 + MP2 TS 源探测成功，ffmpeg 却在生成首片前报 `Option allowed_extensions not found`。隔离 overlay 恢复此参数，`TestHLSTranscodeRealFFmpeg` 在 0.41 秒按同样原因失败，日志 `/tmp/songloft-541-negative-demux.log`。
- 规则与修复：不要把特定 demuxer 的选项放进共用输入参数；网关给改写资源保留允许的媒体后缀，使用通用协议/容器限制，普通 TS 输入无需 HLS 专属参数。测试分别覆盖直接 TS 与主列表→子列表→TS 分片的完整执行链。
- Done right：Linux amd64 / Go 1.26.0 / FFmpeg 5.1.7，真实视频包 SHA-256 逐包不变、输出 AAC-LC 双声道 48kHz、滚动窗口、会话回收、定向 race 和实际后端 HTTP 验证通过。日志 `/tmp/songloft-541-race-complete.log`，实际进程与目录回收证据 `/tmp/songloft-541-smoke-rln_xh54/result.json`。另一个 overlay 去掉会话归属判断时，回归按预期因跨客户端读取成功而失败，日志 `/tmp/songloft-541-negative-owner.log`。
- Not run：iPhone Safari、报告人的实际 IPTV 源及其他系统真机；后端/媒体验证不替代设备播放验收。
- Seen 1× (2026-10-26)。状态：经验；主仓库后端实现与验证完成，发布及 Safari 实测以 #541 后续证据为准。

### PLAYLIST-001：内置歌单的展示偏好与隐藏歌曲语义必须分开核对

- 范围：首页收藏展示、客户端菜单及服务端歌单可见性，songloft-org/songloft#532。
- 失败证据：2026-10-21，[初次回复](https://github.com/songloft-org/songloft/issues/532#issuecomment-6308512268)将普通歌单的隐藏操作套用于内置「收藏／电台收藏」。实际 Web 截图显示内置「收藏」菜单只有置顶；Flutter/Lynx 菜单与后端 `/playlists/{id}/visibility` 均明确限制内置歌单。[更正及截图](https://github.com/songloft-org/songloft/issues/532#issuecomment-6309354674)已发布。
- 规则：解释操作前核对目标对象的身份条件、实际菜单和后端限制，不能仅从普通对象的调用链推断内置对象可用。设计首页展示时同时追踪 `hidden` 标签的消费者：歌曲列表默认排除隐藏歌单内的歌曲，因此首页卡片显示偏好必须独立保存。
- Done right：本次经用户确认，在两客户端新增独立、按服务器与账号保存在本机的首页显示开关；按内置 ID 与 `built_in` 识别，不改收藏成员或标签。测试、实际浏览器及未跑项分别记录在 [Flutter](clients/player/docs/cn/issue-532.md) 与 [Lynx](clients/player-lynx/docs/project/issue-532.md)，最终结果绑定各自工作树，不外推为原生设备或已发布版本。
- Seen 1× (2026-10-21)。状态：经验；实现与验证完成，提交、CI 与版本状态以 #532 后续回复及交付验证记录为准。

### JSPLUG-002：客户端 SDK 的方法存在不等于宿主传输可用

- 范围：公共资源 `internal/jsplugin/assets/common.js` 的 Flutter / Lynx / iframe 宿主桥；无关联 Issue，基线 `32ca05ccab18a68133ab1ccb776460610c835726`。
- 证据：2026-10-20，旧 `isHostAvailable/invokeHost` 只识别 Flutter callHandler 与 iframe，Lynx 原生顶层 WebView 即使已有业务分发器，仍判定为无宿主。新真实脚本回归在隔离旧资源下失败 5 项、保留 Flutter/iframe 2 项通过。
- 规则与修复：按宿主实际入口分别接 Android `LynxWebViewBridge`、iOS `webkit.messageHandlers.nativeApp`、HarmonyOS `LynxJSBridge`，再验证带请求 ID 的成功/错误回执、超时清理和状态退订。不能只 mock 分发器便宣称 SDK 可用；原生壳、公共资源与客户端分发器分别核对。
- Done right：未提交工作树下 `node --test scripts/tests/plugin-client-bridge.test.mjs scripts/tests/plugin-theme-appearance.test.mjs` 9 项通过，`go test ./internal/jsplugin` 通过。Lynx 的完整接口、实际商店插件和平台验收记录见子仓库 `docs/reference/plugin-client-sdk.md`；原生 URL 参数只由嵌入页添加，外部浏览器入口不带该标记。
- Not run：iOS/HarmonyOS 编译及三端原生设备 Cookie store；受控消息入口不替代原生页面验收。
- Seen 1× (2026-10-20)。状态：经验；未提交、推送或发布。

### JSPLUG-001：常驻插件的后台 pump 不能用固定周期轮询推进异步链

- 范围：`internal/jsplugin` 的后台定时器协程与 `internal/jsruntime` 的异步唤醒通道，影响所有常驻 JS 插件；失败实例为 [songloft-org/songloft-plugin-miot#119](https://github.com/songloft-org/songloft-plugin-miot/issues/119)。
- 失败证据：2026-10-20，基线 `7c9f9f341865514d8e606bdb90e01ac92e148208` 的 `runTimerProcessor` 用固定 500ms ticker，脱离 `ExecuteJS` 的后台异步链每个 await 续作都要等下一个 tick。报告人[日志](https://github.com/songloft-org/songloft-plugin-miot/issues/119#issuecomment-6281088024)显示模型首字 `first_content_ms=1022`，播报下发却到 `speech_dispatch_ms=9499`；同一轮里 Go 侧 `doHTTPRequest` 的真实 RTT 为 848/294/103/245ms，而 JS 侧观测到的对应时刻是 43.652/44.151/47.145/47.644，全部落在 500ms 网格上。新增的 `TestTimerProcessor_BackgroundChainIsNotPumpBound` 在 overlay 隔离的旧实现下，10 跳链 2 秒只推进 4 跳而失败。
- 规则与修复：pump 协程由 `asyncSignal` 即时唤醒，否则睡到 `GetNextTimerDeadline`，两者都没有才退回 500ms 兜底；下限 `minWakeInterval = 10ms`，避免已到期定时器在 VM 锁被长请求占用期间空转。`executeInEnv` 返回时 `signalAsync` 一次——插件在本次调用里新注册的定时器（`onInit` 起后台轮询、`onHTTPRequest` 用 `setTimeout(fn, 0)` 把任务甩到后台）对已经睡下的 pump 协程不可见。排查插件延迟时先把宿主侧网络时间戳与 JS 侧观测时刻对齐，差值落在固定网格上就是 pump 周期，不是上游慢。
- Done right：Linux amd64 / Go 1.26.0 / Node 26.5.0，基线 `7c9f9f341865514d8e606bdb90e01ac92e148208` + 本次未提交工作树。`make check`（含 gofmt、`go vet`、全量 Go 测试）通过；新回归普通模式 `-count=20`、`-race -count=5` 均通过；用 `go test -overlay` 装回固定 ticker 的旧实现时按预期失败（2 秒 4/10 跳）。日志 `/tmp/songloft-119-20261020/go-{before,after,race}.log`。写回归时不能在等待期间调用插件：`InvokeHTTP` 走的 `ExecuteJS` 自带事件循环，会替 pump 协程把链推完，从而掩盖回归；耗时上限也不要绑死在 VM 实际速度上，按旧轮询网格的几分之一取值才在 `-race` 下稳定。
- Not run：真实音箱与长期运行；端到端延迟收益来自日志推断，未在设备上复测。插件侧的配套改动见子仓库 MIOT-011。
- Seen 1× (2026-10-20)。状态：经验；真机复验与发布另行核对。

### DOCKER-001：在线升级必须按运行架构选包，并在替换前拒绝执行失败

- 范围：Docker 在线/上传升级和启动时底包恢复，songloft-org/songloft；无关联 Issue。
- 失败证据：2026-10-19，基线 `341b49695b3a7763c2ead5e5b43f228d0dcbd3de` 的下载后缀写死 `linux-amd64`，ARM 镜像因此选错包；`TestBinary` / `ValidateBinary` 忽略除超时外的所有执行错误。隔离旧实现后，错架构、HTML、截断、非零退出和无版本输出的回归均失败。Linux amd64 Docker 用修改 ELF machine 的本地文件复现：显示升级完成、文件被替换，随后重启循环；启动脚本在打印坏文件版本时受 `set -e` 影响，尚未进入 unknown 恢复分支就退出。
- 规则与修复：用运行程序的 `runtime.GOOS/GOARCH` 对齐 Release 的 `linux-amd64/linux-arm64/linux-armv7` 命名，保留底包 full/lite 选择；替换前检查 ELF machine/class，并要求 `-version` 成功退出且含 Songloft 版本。在线和上传确认共用校验，失败不备份、不替换、不重启。启动脚本不信任失败的版本命令，诊断输出失败不阻断底包恢复。
- Done right：本次未提交工作树，Linux amd64 / Go 1.26.0；`CGO_ENABLED=0 go test -tags dev,lite ./...`、同标签 `go vet ./...`、定向 `CGO_ENABLED=1 go test -race` 与 52 项 `node --test scripts/tests/*.test.mjs` 通过；arm64、armv7 的服务测试交叉编译通过。真实 Docker 验证坏架构/HTML 被拒绝且旧文件哈希不变、零重启，lite 正常升级和 PUID/PGID=1000 升级后进程版本正确，以及坏文件恢复底包。Docker Chrome 实际登录、点击设置→关于与更新→升级，full 服务重启一次后返回目标 commit，落盘文件 SHA-256 与候选一致，刷新后显示已是最新。
- 证据：`/tmp/songloft-docker-upgrade-20261019/` 中的 `regression-{before,after}.log`、`entrypoint-{before,after}.log`、`backend-tests.log`、`vet.log`、`race.log`、`scripts.log`、`docker-results.json`、`browser-result.json`、`browser-upgrade.png` 和 `validation.json`。旧启动脚本在新增恢复回归中失败 2 项，修复后通过；下载通过本地 GitHub 代理夹具提供，非线上发布包验收。
- Not run：ARM 真机、ARM 容器实际运行和正式发布/远端 CI；后缀矩阵及交叉编译不替代 ARM 实机升级。既有故障实例可先停止容器，保留数据库并将 data 卷的 `songloft` 可执行文件改名备份，再启动从镜像底包初始化。
- Seen 1× (2026-10-19)。状态：经验；正式发布与 ARM 实机验收另行核对。

### WEB-001：依赖中存在 SharedArrayBuffer 不等于页面必须启用跨源隔离

- 范围：完整版嵌入 Lynx Web 的部署评估，songloft-org/songloft 与 songloft-org/songloft-player-lynx；无关联 Issue。
- 证据：2026-10-16，父仓库 `d59b12e373525e5dea546ba6531931a01af42c8e`、Lynx `9ced2b895376dfc829895100db70530bf8ac4426`。初次调查沿用部署文档和 CORP 注释，将 RPC 库中的 SAB 同步分支误判为启动硬依赖。实际 `web-core 0.23.1` 的 33 个内部端点全部为异步；Docker Chrome 121 在无 COOP/COEP 的局域网 HTTP、localhost 子路径及 HTTPS 中，页面与两个 Worker 的 SAB 均为 `undefined`，仍完成登录、真实媒体播放/seek/暂停/切歌、收藏后端持久化和刷新会话恢复。
- Done right：核对实际调用端点，并在无安全上下文、无隔离头、无 SAB 的真实浏览器中验证关键动作；用媒体时间与后端状态辅助截图，不从依赖库符号存在推导部署门槛。HTTPS 场景仅在本次浏览器会话忽略测试证书错误，另用测试 CA 验证 TLS 链；未修改系统信任。证据位于 `/home/hanxi/work/songloft-review/lynx-full-20261016/` 的 `rpc-audit.json`、`logs/*-final.json` 与 `screenshots/`。
- Not run：Firefox、真实 Safari、插件双引擎和长后台；不将已验收的核心流程外推为所有浏览器/插件均无 SAB 需求。HTTPS 对剪贴板、Wake Lock 等能力的要求独立存在。
- 后续落地：用户验收预览后授权默认切换及提交推送。构建脚本分离父/子仓库 tag、SHA 和元数据，显式执行引擎补丁、拒绝测试桥产物并预压缩 bundle/WASM；嵌入页增加 Go 可替换的 base 标签，旧脚本在新回归中失败。本地 Lynx 3543 项、84 项发布检查、后端全量及 42 项脚本检查通过，根/子路径 19 项实际动作及旧书签启动通过。证据 `/home/hanxi/work/songloft-review/lynx-default-20261016/`；最终提交、CI 与 dev 产物另按同一 SHA 验收。
- CI 后续：子仓库归档检查将 base 标签误当作资源，已补实际打包→归档→校验回归及 HTMLParser 修复；base 仅由父仓库通过 `--go-embed` 启用，保留普通 embedded 静态包的子目录部署，最终 87 项发布检查通过。见 [Lynx WEB-001](clients/player-lynx/LESSONS.md)。旧构建取消后重新按最新 gitlink 验收，不将取消视为通过。
- Seen 1× (2026-10-16)。状态：经验；同一事故不递增，不打正式版 tag。

### SORT-001：歌单文字排序必须在分页前统一生成拼音键

- 范围：歌单歌曲分页、播放定位 ID 列表及永久名称排序，songloft-org/songloft#524。
- 失败证据：2026-10-15，基线 `cffffe39a05f32df6651e3e47acefb4692b8b533` 的 `applyPlaylistSongOrder` 直接按 SQLite 文本编码排列，中文首字母跳动、英文大小写分组；新回归在旧实现下的 title/artist/album 双方向均失败。隔离旧 filters.go 后，永久 name_asc/name_desc 回归也因错误顺序失败，日志 `/tmp/songloft-524/{before,permanent-before}.log`。
- 修复：本次工作树注册确定性 SQLite 拼音键函数，保留非中文字符、忽略大小写及首尾空白，由同一个排序构造器在 LIMIT/OFFSET 前使用；等值键继续按 position 同方向排序。多音字使用字典默认读音，不承诺词组语境识别。两个客户端将「标题」菜单明确为「歌曲名称」。
- Done right：真实内存 Repository 回归覆盖简繁中文/英文混排、大小写与同音等值、四页拼接、搜索、数字/自定义顺序；文件数据库双连接验证函数注册。后端完整测试/vet、定向 race 和 8 组真实 HTTP 场景通过；30,000 首合成曲目的中间页约 344ms、完整 ID 约 263ms，二者顺序一致（单次本机采样，非性能保证）。证据 `/tmp/songloft-524/{backend-focused,race,perf}.log`、`api-result.json`。
- Seen 1× (2026-10-15)。状态：经验；Android 原机验收与发布状态另行核对。

双客户端中英文 390px 浏览器实测名称升降序、艺术家排序、后端偏好保存及重新进入恢复通过，页面异常为零；证据 `/tmp/songloft-524/{flutter-browser,lynx-browser,validation}.json`。浏览器验证不替代 Android 真机验收。

### PLUG-001：插件版本兼容性需要核对更新入口的比较规则

- 范围：插件清单校验、服务端更新检查与安装、Flutter/Lynx 插件商店及 GitHub 发现页。
- 证据：2026-10-14，为 [songloft-org/songloft#495](https://github.com/songloft-org/songloft/issues/495) 准备补丁版时，先核对构建器与服务端便拟使用 `v2026.10.14.1`；用户要求继续核对完整升级链路后，发现各入口并不统一。主仓库 `6b050ff34c52f8140063e373319692814b030ec0` 的构建器/宿主清单正则接受四段，`CompareVersion` 逐段比较，真实 Repository + HTTP fixture 的 `CheckUpdate`/非强制 `DownloadUpdate` 完成 `2026.10.14 → 2026.10.14.1` 安装；已发布 v2.13.1 的服务端比较实现相同。
- 限制：Flutter `a0b2ae17194dd0a1acdaa7c0050ca4681e119c07`、Lynx `9a92929f5f33efdefecc1a01632cb8270ebdc59b` 的普通插件源比较可识别第四段，但 GitHub 发现页分别调用 `comparePluginVersions` / `compareStableVersions`，严格只接受三段，四段比较返回未知，不显示更新提示。这不是所有插件升级都失败，也不能宣称所有入口均兼容。tag 可带 `v`，清单中的 `version` 必须不带。
- Done right：实际执行 Go 清单/更新/下载/安装链路和两客户端的源码比较函数；三段 `2026.10.15` 在这些比较入口均识别为新版本，本次发布采用该版本，不修改客户端原生升级协议。证据在 `/tmp/songloft-495-20261014/version-upgrade-probe.log`、`version-probe_test.go`、`client-version-probe.dart`。临时 Go 测试已清理，未修改产品代码。
- 规则：发布新版本格式前核对所有相关入口，区分打包/安装可接受与 UI 能否提示更新；不以日期/semver 文案代替实际校验。不能为插件比较直接放宽客户端原生应用升级协议。
- 后续修复：按用户要求改为自动四段日期版本发布，Flutter/Lynx GitHub 发现页采用插件专用三/四段比较，原生应用与最低宿主版本比较仍只接受三段。回归覆盖 `.9 → .10`、跨日、三段与 `.0` 相等，以及更新按钮和对应包安装请求；旧 Lynx 发现页在新增交互用例中显示 Reinstall，修复后显示更新版本。
- 迁移规则：既有 `2026.10.15` 与 `2026.10.15.0` 数值相等，首次迁移从 `.1` 开始；北京时间日期追上前沿用该前缀，下一新日期从 `.0` 开始。构建器、Go 安装链路接受四段不代表旧客户端发现页自动获得该能力。
- Not run：各历史客户端发布包的完整矩阵、真机安装；交互回归不证明已安装旧客户端能提示四段更新。
- Seen 1× (2026-10-14)。状态：经验，当前源码的四段插件比较已统一；历史客户端边界保留。

### FLOW-001：关联 Issue 的修复推送后，主动交付待确认的回复草稿

- 范围：主仓库与子仓库的开发交付及 Issue 沟通。
- 失败：2026-10-13，songloft-org/songloft#495 的修复 [songloft-org/songloft-plugin-miot@1cb1b52](https://github.com/songloft-org/songloft-plugin-miot/commit/1cb1b5214612e9fe611e8d8c61bfa47d5fc2f579) 已推送，v2026.10.14 已发布并同步主仓库 [540bf0c](https://github.com/songloft-org/songloft/commit/540bf0c76f87f08ba54dec1573cf9922126ad68c)，交付却未准备并展示 Issue 回复草稿。用户追问后才明确安排回复，造成额外往返。
- 修复：当前未提交的流程更新，在 AGENTS、AI 流程及 dev-flow 中要求推送后主动展示完整草稿、目标 Issue 和具体评论确认动作；已有适用评论授权直接执行。草稿跟随 CI/发布结果更新，评论与关闭分别授权。
- Done right：用户明确要求回复后，发布说明已[发表并回读验证](https://github.com/songloft-org/songloft/issues/495#issuecomment-6147071915)，说明可用版本、实际验证和真机/时长差异的验收边界。新增「主动草稿 → 确认 → 发表」流程的后续实用验证待补，不能把本次补发视为已验证该完整流程。
- Seen 1× (2026-10-13)。状态：按用户明确要求立即纳入 [关联 Issue 回复](docs/ai-development.md#关联-issue-回复)，不等待经验次数门槛。

### TEST-001：临时目录清理失败要核对后台文件写入

- 范围：Go 插件生命周期、异步字节码缓存与测试临时目录。
- 规则：`TempDir RemoveAll: directory not empty` 不能直接归为环境问题或用延时掩盖；检查测试结束和后台写盘的先后顺序。重跑通过只证明该次检查通过，不代表生命周期竞态已修复。
- 失败：2026-10-14，songloft-org/songloft#518 的父指针提交 [`405e8a0`](https://github.com/songloft-org/songloft/commit/405e8a0f2697bac53c3311567fdca025256335df) 在 [CI 首轮](https://github.com/songloft-org/songloft/actions/runs/38526078645/attempts/1) 的 `TestManager_ReloadPlugin` 清理阶段报错。日志先出现管理器关闭，再出现 `bytecode cached`；本批只改客户端指针，未改 Go 源码。
- 调查与处理：`JSService.Load` 异步执行字节码编译及缓存写入，关闭流程未等待该写盘；证据指向后台写盘与临时目录清理竞态。同一 SHA 本地 `CGO_ENABLED=0 go test -tags dev,lite ./internal/jsplugin -run '^TestManager_ReloadPlugin$' -count=30` 通过；失败作业重跑后 Quality 通过。日志在 `/tmp/songloft-518-{parent-ci-failed,reload-repeat}.log`。本批未修复该后台任务生命周期，后续应补确定性回归与退出等待。
- Done right：保留首轮失败、实际 SHA 与重跑结果；若修复生命周期，验证关闭返回后不再写缓存，再清理临时目录，不把单次重跑当成修复证明。
- 后续证据：v2.13.2 的 `c2ecbf9f924528b75106daa56f1cfe087f91377f` 在 [Release 首轮](https://github.com/songloft-org/songloft/actions/runs/38605138465/attempts/1) 的 `TestManager_ReloadPlugin`、[第二轮](https://github.com/songloft-org/songloft/actions/runs/38605138465/attempts/2) 的 `TestIdleWakeup_HTTPRequestReloadsEvictedPlugin` 均出现关闭后 `bytecode cached` 与临时目录清理失败。2026-10-15 排查确认该 SHA 的 `JSService.Load` 仍用未等待的 goroutine 编译并写缓存，`Stop` / `Manager.Close` 未等待缓存任务；属于同一未修复生命周期问题的后续证据，不增加独立事件次数。日志在 `/tmp/songloft-run-38605138465-{attempt1-failed,failed}.log`；本次只调查，未修复或重跑 CI。
- 修复与验证（2026-10-15）：本次修复在 `JSService.Load` 持锁登记缓存完成信号，后台任务退出时关闭；`Stop` 在无锁等待整个编译及写盘完成后才执行生命周期清理，因此卸载、重载和管理器关闭也会等待。新增 `TestBytecodeCache_ShutdownWaitsForCompletion`，通过日志屏障阻塞真实缓存任务，覆盖 stop/unload/close；用 Go overlay 隔离加载旧 `service.go` 时三项均因提前返回失败，修复后三项通过。Linux amd64 / Go 1.26.0 / Node 22.17.1，基线 `3bbea95fa9eba7519eed1e4e000123923d78e3b4` + 本次工作树：`CGO_ENABLED=0 go vet -tags dev,lite ./...`、`CGO_ENABLED=0 go test -tags dev,lite ./...`、`node --test scripts/tests/*.test.mjs` 全通过；新回归与上述两项原失败测试重复 30 轮通过，同组 `CGO_ENABLED=1 go test -race -tags dev,lite` 通过。证据在 `/tmp/songloft-cache-{before,regression,repeat,all-tests,vet,scripts,race}.log`。Not run：各原生平台实机验证；本次未修改客户端或 API。
- Seen 1× (2026-10-14)。状态：经验，本次提交修复缓存任务的停止等待；远端构建结果另行核对同一 SHA。

### TEST-002：并发事件回归必须同步请求就绪与回调完成

- 范围：Go / QuickJS 入站 WebSocket 与挂起 HTTP 请求的集成测试。
- 失败：2026-10-15，songloft-org/songloft#520 的指针提交 `8252cb762e8363283a860c3a091748a0aadb4b02` 在 [CI 首轮](https://github.com/songloft-org/songloft/actions/runs/38670932842/attempts/1) 的 `TestInboundWebSocketHostEventDeliveredDuringHTTPRequest` 返回空事件列表；本批没有修改运行时代码。本地 `CGO_ENABLED=0 go test -tags dev,lite ./internal/jsplugin -run '^TestInboundWebSocketHostEventDeliveredDuringHTTPRequest$' -count=30` 也复现同一失败。测试用固定 100ms 推断 HTTP 已清空列表，再以 500ms 定时器返回，未同步真正的请求开始与 close 回调。
- 规则与修复：使用 WebSocket `socket-ready` / `http-started` 消息建立先后顺序，HTTP Promise 由 close 回调记录事件后完成；Go 侧设置读与请求截止时间，只用于失败收敛，不把延时当同步。仅调整测试，未改运行时。
- Done right：修复后目标用例重复 30 轮、`CGO_ENABLED=0 go test -tags dev,lite ./...` 和 `go vet -tags dev,lite ./...` 通过；Go overlay 隔离删去 close 事件记录后，用例按预期因只收到 `message:ping` 而失败。基线 `9b5ffd96cdd156cfd12b170a6df02eff5aa0f5f1` + 本次测试工作树，Linux amd64 / Go 1.26.0，日志 `/tmp/songloft-520-followup/parent-{ci-failed,websocket-repeat,websocket-fixed,websocket-negative,tests-fixed,vet-fixed}.log`。最终提交与 CI 另按对应 SHA 核对。
- Seen 1× (2026-10-15)。状态：经验。

### DOCS-001：自动侧栏必须固定扫描根并显式排除依赖目录

- 范围：`docs/.vitepress/config.mts` 的自动侧栏及 LLM 文档索引。
- 证据：2026-10-15，基线 `8252cb762e8363283a860c3a091748a0aadb4b02` 的 `path: '.'` 从仓库根启动时混入子模块与依赖，运行中的 `/@siteData` 返回 7,497,742 字节；从 `docs` 构建也会因 `node_modules` 未被默认排除产生 `No matching file found for sidebar link`。
- 规则与修复：插件 1.7.0 用 `join(process.cwd(), path)`，不是 `resolve`，不能直接传绝对路径；用配置文件的 `import.meta.url` 定位 docs，再转为当前目录相对路径。递归仅使用显式 `ignoreList`，因此同时列出依赖、构建、静态资源及插件原有排除目录，不屏蔽告警。
- Done right：根目录执行 `node docs/node_modules/vitepress/bin/vitepress.js build docs`，缺失侧栏链接警告归零；在 `docs` 启动 dev 后站点数据降至 95,743 字节。Docker Chrome 实际点击开发版及构建产物的中英文侧栏、语言链接和移动菜单，采集到的 66 个侧栏链接经 HTTP 均返回 200。证据：`/tmp/songloft-sidebar-WVrz5I/` 中的构建日志、运行时数据、浏览器记录及截图。
- 边界：构建包体积警告仍存在；编辑配置触发插件热重启时遇到 MiniSearch 重复索引，冷启动验证正常，本次未修复热重启。Not run：单元测试和类型检查，本次采用构建及真实运行验证。
- Seen 1× (2026-10-15)。状态：经验；扫描范围修复在本次未提交工作树。

### RELEASE-001：跨仓库发布必须按「子仓库 tag 先推送、gitlink 后落地、父仓库最后」的顺序

- 范围：`scripts/release-all.sh`、三个仓库的 `bump-version.sh` 统一契约、父仓库 gitlink 提交。
- 规则：父仓库 bump 脚本要求干净工作树，因此两个子模块指针必须先用独立 commit 落地；gitlink commit 必须用 pathspec 限定（`git commit -m ... -- clients/player clients/player-lynx`），否则 staged/无关改动会被裹带。链式解析依赖「stdout 末行 = v<版本>、日志走 stderr」的固定契约。
- 证据：2026-10-15，基线 `9d008b8`。旧版三个脚本参数与行为各异（Flutter tag 冲突可覆盖、父仓库无预检），无法安全编排；统一为 Lynx 严格契约后由 `scripts/tests/release-all.test.mjs` 夹具（本地 bare remote + stub）锁定顺序、脏树中止、失败即停、dry-run 零写入与 gitlink 隔离；`scripts/tests/bump-version.test.mjs` 覆盖预检与 `--no-push` 本地路径。Lynx 既有 `pnpm run test:release` 77 项通过。
- Done right：真实仓库 `--dry-run` 验证三脚本计划输出与预检（脏子模块在调用任何发布脚本前被拦截）。推送 tag 触发各 CI 属发布动作，另行授权；本条不证明真实发布链路。
- 同日交互补证：`190c32c1768a88678edea3de5b1b94c40108c95d` 编排捕获 stdout，而 Lynx `0de993ff272ee29df96c8401b46df56256646438` 将计划/提示写到 stdout，空回车取消还退出 0，导致误跳过继续。旧实现下新增的 5 项 Lynx 流/取消回归和 9 项编排版本输出回归均失败；本次修复把 Lynx 提示移至 stderr、取消退出 1，并在编排端校验三个版本结果。主仓库 31 项脚本测试与 Lynx 81 项发布工具测试通过，PTY 在真实命令替换下验证回车中止与 yes 本地发布。日志 `/tmp/songloft-release-{all-before,lynx-before,parent-suite,lynx-suite}.log`、终端证据 `/tmp/songloft-release-terminal-result.json`；不增加同日 Seen。本次未运行真实发布。
- Seen 1× (2026-10-15)。状态：经验。

### RELEASE-002：更新日志必须以上一个已发布正式版为起点，不能只看 tag

- 范围：主仓库 Release 工作流的变更范围、`CHANGELOG.md` 与 GitHub Release 说明。
- 失败证据：2026-10-15，v2.13.2 的 [Release](https://github.com/songloft-org/songloft/actions/runs/38605138465) 在 Quality 失败后遗留 tag；v2.13.3 的 [成功发布](https://github.com/songloft-org/songloft/actions/runs/38693348336) 仍选择 v2.13.2 为起点，导致 v2.13.1..v2.13.2 的 66 条符合现有规则的提交全部漏记（另 5 条非规范提交被配置过滤）。代码已包含在 v2.13.3，遗漏发生在仓库日志与线上说明。
- 规则与修复：本次工作树通过分页 Releases API，仅从非 draft、非 prerelease 且已有发布时间的三段版本中选择当前历史可达的最高版本，排除当前 tag 与 HEAD 同 SHA 的 tag；API 失败直接中止，仅无适用正式版时回退首提交。把遗漏补入 v2.13.3，比较范围修正为 v2.13.1...v2.13.3，不把失败的 v2.13.2 写成已发布版本。
- Done right：基线 `c31806ad51483d7e3c67b6016ed3dd463ecef501` + 本次工作树，Linux amd64 / Node 22.17.1；7 项实际工作流步骤回归中旧实现失败 6 项，首项明确误选 v2.13.2；修复后 7 项及全套 38 项脚本测试通过，actionlint v1.7.12、Prettier 与 `git diff --check` 通过。真实只读查询对 v2.13.3 返回 v2.13.1，对 dev 返回 v2.13.3；提交集合核对确认 80 条、无重复、无遗漏。日志 `/tmp/songloft-changelog-{regression-before,regression-after,scripts,actionlint}.log`。
- Not run：完整后端测试、平台打包及远端工作流运行，本次仅修改发布脚本与文档，工作树尚未提交/推送。
- Seen 1× (2026-10-15)。状态：经验。

## 条目格式

```text
## 稳定编号：一句话规则
范围：仓库/模块/平台
失败：owner/repo#编号、commit SHA、日期、行为与后果
修复：commit SHA、验证对象与实际结果
Done right：正确示例及证据；没有时写待补
Seen N× (YYYY-MM-DD, YYYY-MM-DD)
状态：经验 / 升级候选 / 已升级（规范位置）
```
