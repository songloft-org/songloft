# 开发经验

保存主仓库与公共宿主契约的证据型经验；执行见 [AI 开发流程](docs/ai-development.md)。后续会话在相关任务前读取适用条目。客户端、插件和设备经验放所属子仓库。

## 记录与升级

- 每条含规则、范围、失败/修复证据、正确示例和独立事件日期；使用完整仓库引用与 commit SHA/链接。
- `Seen N×` 统计独立事件的不同日期，同一事故反复复盘不递增；三天后提出规范升级并标记权威位置。已有铁律、严重问题不等待三次。
- 不编造次数、修复状态或验证结果，与 [英文版](LESSONS.en.md) 同步，敏感信息脱敏。

## 条目

### SCAN-FORMAT-001：新增扫描格式要验证默认播放地址

- 本地 DTS 支持的主仓库基线为 `01284955a6e1272b2dfb0fb03b2dcadca5f2c831`。[songloft-org/songloft#588](https://github.com/songloft-org/songloft/issues/588)
- 隔离移除迁移时，真实 DTS 夹具扫描发现零首。仅恢复旧播放逻辑时，已入库歌曲的默认地址返回裸 DTS。现有 Docker 镜像的 ffprobe 无法读取同一个样本。
- 迁移与扫描回退列表同步新增扩展名，复用 ffprobe 补齐技术参数，默认播放走已有兼容转码。扫描迁移测试用 `UpTo` 验证目标版本，避免之后的格式迁移改变预期值。Docker 和 Bundle 的精简工具同时启用对应 demuxer。
- 本地 `.dts` 和 `.DTS` 都完成入库，5.1 输入默认返回双声道 MP3，显式 FLAC 保留六声道，Range 与 HEAD 通过。Linux FFmpeg 8.0.1 候选工具通过了 DTS 和 DTS-HD 的实际后端扫描与播放。Docker Chrome 在曲库点播后确认默认请求返回 MP3，播放进度持续推进。
- Seen 1× (2026-11-21)。

### CI：签名配置变更需要启动新一轮发布

- [后端 Release](https://github.com/songloft-org/songloft/actions/runs/44754365874)生成了 Bundle 测试包，缺少 Lynx 桌面与 Android 发布签名时跳过 `Create Release`。随后补齐 Secrets，文档 push 只触发文档站。
- 改签名配置后从对应仓库手动运行工作流，检查实际发布 job 与目标 Release 附件。签名身份和触发规则见 [Bundle 发布指南](docs/bundle-mode.md#发布触发与签名)。
- 补齐签名后的[首轮构建](https://github.com/songloft-org/songloft/actions/runs/44830577454)在 Android Gradle 解析依赖时收到 Maven Central 的 `429`。[下一轮后端 CI](https://github.com/songloft-org/songloft/actions/runs/44832013772)在相同签名配置下通过，发布了 Bundle 安装包和签名清单。
- Seen 1× (2026-11-21)。

### BUNDLE-FFMPEG-001：多平台资产必须绑定具体 Release 身份

- 范围：后端 Release 的 Flutter/Lynx Bundle 构建与 `tools/ffmpeg-builder` 资产解析。
- 规则：构建开始时按 `dev` 或稳定通道解析具体 Release tag，校验 manifest 的源码提交、平台、ABI、大小和 SHA-256，并把 `ffmpeg-resolution.json` 交给所有矩阵 job。不要在单个平台 job 直接下载滚动 `latest` 文件。
- 正确示例：解析记录同时保存 `release_tag`、`source_commit`、`manifest_sha256` 和资产摘要；重试只复用记录中的固定 URL 与摘要。
- Seen 1× (2026-11-20)。[FFmpeg dev CI](https://github.com/hanxi/ffmpeg-builder/actions/runs/44751699585) 发布了十个平台资产。[后端 CI](https://github.com/songloft-org/songloft/actions/runs/44754365874) 校验了同一份清单，并生成了 Lynx Android、iOS、Windows 和两种 macOS 架构的安装包。真机安装与播放待验收。

### BUNDLE-FFMPEG-002：原生转码应保留拉流选项中的空值

- 范围：iOS Bundle 的 FFmpeg 原生运行器及电台、HLS 回环网关。
- 证据：在主仓库基线 `c6eb437` 的工作树中，Linux 下的 `TestHLSTranscodeRealFFmpeg` 在原生运行器使用字符串解析 FFmpeg 参数时失败。HLS 网关传入了空的 `http_proxy`。改为直接设置 `AVDictionary` 后，实际 TS、HLS 和 AES-128 HLS 转码及取消回归通过。
- 正确示例：用 `av_dict_set` 传递原值，包括空代理和以 CRLF 结束的 HTTP 请求头，同时保留输入协议及格式白名单。
- Seen 1× (2026-11-20)。状态：Linux 原生分支已验证。iOS 真机待验收。

### BUNDLE-FFMPEG-003：原生桥接需编译验证发布使用的 FFmpeg 头文件

- 范围：iOS Bundle 原生桥接与后端 Linux 质量检查。
- 证据：`b786eca` 在本地 FFmpeg 5.1 编译通过，Release CI `44745510791` 的较新 FFmpeg 头文件已删除 `FF_PROFILE_AAC_LOW`。
- 规则：优先使用 `AV_PROFILE_AAC_LOW`，旧版头文件通过条件定义兼容。保留 CI 原生测试，覆盖本机与发布环境之间的 FFmpeg 版本差异。
- Seen 1× (2026-11-20)。

### BUNDLE-PORT-001：动态端口必须在监听后同步到所有 URL 生成器

- 范围：Bundle 移动后端、JS 插件宿主地址和内部插件代理 URL；[songloft-org/songloft-player-lynx#16](https://github.com/songloft-org/songloft-player-lynx/issues/16)，父仓库基线 `dff636c`，Lynx 基线 `c403ea0`。
- 失败证据：报告日志显示后端以请求端口 `0` 初始化，实际 listener 使用 `33807`，但音箱插件的宿主桥仍生成 `localhost:0`。同一日志中的普通本地播放使用实际的 `127.0.0.1/play`，说明问题集中在启动前捕获端口的 URL 生成器。
- 规则与修复：`net.Listen` 返回后，将真实端口同步到 `InternalURLResolver`、JS 插件管理器及已存在的桥接处理器，再开始接受请求。Bundle 的动态端口不能只写入配置或返回给客户端，任何缓存监听端口的地址生成器都必须更新。
- 验证：新增解析器端口更新、管理器更新既有桥接处理器、宿主地址及移动 listener 入口回归；`make check` 与 `go test ./internal/app ./internal/services ./internal/jsplugin ./mobile` 通过。Lynx Android 真机和音箱复测待用户环境验收。
- Seen 1× (2026-11-20)。状态：父仓库本地未提交修复。

### JSPLUG-003：插件宿主基础 URL 需要携带部署路径

- 范围：JS 插件宿主桥接，[songloft-org/songloft-plugin-miot#149](https://github.com/songloft-org/songloft-plugin-miot/issues/149)，后端修复基线 `4e90470`。
- 证据：Linux amd64 上，Songloft v2.13.12（`8c014b3`）与 MIoT v2026.11.20.1 在 `/songloft` 部署下，浏览器打开及刷新智能音箱页均触发未带部署路径的 `/api/v1/songs/stats` 并返回 404。
- 正确示例：将路由使用的 `BasePath` 传入桥接处理器。`getHostUrl()` 和 `getNetworkAddresses()` 使用同一个基础 URL 构造函数，使插件能直接追加 `/api/v1/…`。回归同时覆盖根路径、单级及多级子路径和动态监听端口。
- 验证：旧实现未通过两组子路径回归，修复后 `make check`、定向 race 检查、三种路径的实际插件 HTTP 请求均通过。Docker Chrome 首次打开及刷新时均成功显示歌单控件，插件响应包含「全部歌曲」。真实音箱播放待验收。
- Seen 1× (2026-11-20)。状态：后端本地未提交修复。

### PLAY-STREAM-001：音箱播放流必须登记并按会话切换回收

- 范围：本地歌曲播放、播放活动会话和鉴权 context；[songloft-org/songloft#584](https://github.com/songloft-org/songloft/issues/584)，基线 `7dede0c`。
- 失败证据：报告日志中的同一歌曲连续出现多个 `/songs/{id}/play` 请求，部分请求保持数十秒到 19 分钟，已传字节远少于声明长度。旧实现的本地 `http.ServeFile` 不登记播放活动，且鉴权写入的 typed `client_id` key 与播放活动读取的 string key 不相等，生产请求无法按客户端会话取消旧流。
- 规则与修复：认证中间件和播放活动共用同一个 typed context key；正式播放登记 `CatPlay`，同会话切换歌曲时回收旧播放流；本地及缓存文件通过可观察 context 的 `ServeContent` 保留 Range/条件请求语义，并在下一次读写前结束已取消的流。同曲请求继续保留，以兼容同一客户端向多台音箱同步播放；预热和转码条目不受影响。
- 验证：基线隔离副本运行新增鉴权和本地流回收回归均失败，修复后通过。`make check`、定向播放/鉴权/播放活动测试、`CGO_ENABLED=1 go test -race ./internal/services/playactivity ./internal/middleware ./internal/handlers -run 'TestGetSongPlay'` 通过。完整真实音箱发声、外接机械硬盘吞吐和 Docker 部署未验收。
- 边界：完整 `go test -race` 运行在既有 `TestPlaylistCoverRemoteTimeoutStillServesLocalMember` 并行子测试中报告 goose 全局 `SetBaseFS` / `SetDialect` 数据竞争，日志见 `/tmp/songloft-584-race-full.log`，与本次播放代码无调用关系。
- Seen 1× (2026-11-19)。状态：本地未提交实现，Issue 报告人的设备环境仍待复测。

### BUNDLE-ANDROID-001：在应用沙盒内验证内嵌 SQLite

- 范围：Flutter 与 Lynx 的 Android x86_64 Bundle。比对后端提交 `8c014b342ea69683293fec729e9396a925b94e66` 和 `81204e279428db3565f4c33df176a76f7fbf4514`，两包的 modernc SQLite 和 libc 版本一致。
- 证据：在同一个 Android 14 x86_64 模拟器上，将 Flutter 2.13.12 公开 APK 和 Lynx 候选 APK 的 Go 库分别放入同样的应用测试宿主。两者在启动后端时均因旧 Linux `lstat` 被沙盒终止。使用 Android NDK 的 SQLite 驱动的候选库在同一宿主返回健康检查 200。证据见 `/tmp/songloft-bundle-build.v9dPHY/sqlite-android-harness/{results,crashes}.log`。
- 规则：把修复限定在验证过的 Android x86_64 gomobile 路径，复用既有 SQLite 文件、迁移与排序函数。附带记录其他架构选择的驱动。APK 构建通过与实际启动分别验证，完整界面仍需设备验收。
- Seen 1× (2026-11-18)。

### RADIO-001：电台入口的 URL 后缀可能与实际媒体容器不同

- 范围：[songloft-org/songloft#578](https://github.com/songloft-org/songloft/issues/578)，电台播放 URL 与服务端音轨提取，基线 `81204e279428db3565f4c33df176a76f7fbf4514`。
- 证据：咪咕数字路径经两次重定向返回 HLS 主播放列表，媒体分片含 HEVC 与 AAC。基线的 HTTP 添加响应给出 `/play`，客户端据此选择渐进式播放。新实现的 HLS 模式给出 `/play.m3u8`，经实际代理取得的分片可解码 HEVC 帧，同一电台改为仅音频后输出单轨 MP3。日志在 `/tmp/songloft-578.jCp7cj/{baseline,live}.log`。
- 规则与示例：在电台数据中保存用户声明的播放模式，原始入口地址保留重定向行为。向客户端输出与最终响应类型一致的播放路径。音频提取复用有地址校验的[直播拉流网关](docs/pitfalls.md#hls-proxy)。
- Seen 1× (2026-11-18)。

### RADIO-002：按客户端播放能力启动直播转封装

- 范围：[songloft-org/songloft#579](https://github.com/songloft-org/songloft/issues/579)，后端基线 `3184a8b745a4e834c4607ba34bc13caa45601aee`，Lynx 基线 `debad6e80f422c9ed092fa3a5e4af21d34426a83`。
- 证据：本地 FLV 在旧 FFmpeg 白名单下以零输出拒绝，放行后产生单轨 MP3。虎牙报告源用现有电台 UA 经实际后端得到原始 FLV、H.264 + AAC 的 HLS 分片和单轨 MP3。日志见 `/tmp/songloft-579-verify.IUctyN/actual-source.log`。
- 规则与示例：保存源封装并返回明确的播放路径。仅由正在播放且需要兼容输出的客户端创建 HLS 会话，播放取消时回收，普通断流保留原路径重试。浏览器直放时同一个媒体元素负责普通播放和全屏，验证同时检查播放时间、解码帧数及上游请求次数。
- 后续路径校验回归（基线 `00162dd`）：HTTP-FLV 电台通过 `/play.m3u8` 的 GET／HEAD 均返回 200，且错误的带后缀路径会取消原有播放。两个后缀都在联系上游或激活播放会话前校验电台模式，验证同时检查 404、上游请求次数和原播放上下文。 [#579 后续报告](https://github.com/songloft-org/songloft/issues/579#issuecomment-7082650501)
- 自动模式的探测必须在给播放器发地址之前完成。用无后缀 FLV 及经 `.php` 跳转的 HLS 夹具验证添加、换源和显式切回自动模式均生成正确的后缀。手选模式和探测结果分开保存，旧源的探测结果不能回写到新源。
- Seen 1× (2026-11-19)。

### RADIO-003：FLV 直播续流要重新访问电台入口

- 范围：[songloft-org/songloft#579 后续日志](https://github.com/songloft-org/songloft/issues/579#issuecomment-7056482402)，后端基线 `47458a95dae6f8e5560814c6c71dc4267bea62f8`。
- 证据：附件含一条约 123 秒的播放请求。两条报告中的虎牙源在本地旧版分别于约 128 秒、131 秒结束。受控 FLV 夹具中，旧实现结束响应时只访问了一次重定向入口。
- 做法：正常 EOF 后，在同一 MP3 响应内重新访问保存的电台入口以取得新的重定向；首帧前按帧速率补有限的静音。受控回归校验第二次访问入口、MP3 拼接后可解码、固定长度源正常结束，以及显式 MP3 路径只带查询 token 即可播放。
- 验证：两条虎牙源分别通过 `/play` 和 `/play.mp3` 播满 155 秒，同一响应的最大收包间隔约 0.30 秒。Docker Chrome 播放 6 秒后结束的夹具，跨两次回源推进至 9.19 秒，只收到一条播放响应，起播后无 `waiting` 事件。
- 同次续流优化：报告人在一条 362 秒的响应内测得 2.88 秒和 2.02 秒的补齐静音。本地以 `bcbd41c` 为基线，用网关和 FFmpeg 追踪两条虎牙源，上游 EOF 到原编码结束还有约 5–6 秒的缓冲输出。网关在 EOF 时就重新访问入口，下一轮接手已经准备的连接。各拉流 155 秒，观察到的静音从 2.016 秒、1.728 秒降至 0.576 秒、0.288 秒。回归验证新连接仅接手一次，预拉失败可正常重试，取消会释放未用连接。L17A 上的优化效果待复测。 [报告人的长播实测](https://github.com/songloft-org/songloft/issues/579#issuecomment-7062955987)
- 后续取证（基线 `00162dd`）：Linux amd64 通过实际 `StreamAudio` 和 FFmpeg 对两条公开虎牙源各取流 385 秒，每条跨 3 次上游重开。分别观察到 48 帧和 36 帧的最长补帧段（1.152 秒、0.864 秒），4.03 秒未复现。补帧时长和 HTTP 字节到达间隔需分别测量。本轮仅从 HTTP 客户端收流，未对音箱推送。 [真机及服务端复测报告](https://github.com/songloft-org/songloft/issues/579#issuecomment-7082650501)
- Seen 1× (2026-11-21)。

### LYNX-CACHE-001：回退页必须直接引用带版本的宿主脚本

- 范围：Electron 与系统 WebView 的 Lynx 插件回退页，[songloft-org/songloft-plugin-miot#143](https://github.com/songloft-org/songloft-plugin-miot/issues/143)，服务端基线 `b6719811adbf56737ec08f45ec2bcfb636aa209a`。
- 证据：[桌面修复](https://github.com/songloft-org/songloft-desktop/commit/502688952fe0198d9e78dc7acc2c6716f9dde9d2) 记录旧的无版本 shim 曾被缓存一年，且旧脚本不会设置 Lynx bundle 地址。隔离旧 Go 路由后，回归用例因回退页仍使用固定地址而失败。
- 规则与正确示例：在服务回退 HTML 时直接改写工具链生成的 shim 引用，按嵌入内容版本化 shim 和桥接模块。插件生成自己的 bundle URL，宿主读取声明即可。浏览器尚有旧的不可变缓存时，单独更改旧 URL 的响应头无法使浏览器请求该 URL。
- 验证：Docker Chrome 先命中旧 shim 的不可变缓存夹具，再打开子路径部署的 MIoT 正式包，三个新资源和插件 API 均返回 200。证据位于 `/tmp/songloft-lynx-cache-1yoKQ2/`。Electron 应用和原生 Lynx frame 未实测。
- Seen 1× (2026-11-18)。

### SCAN-EXCLUDE-001：排除扫描与排除自动歌单要分开解释

- 范围：定向扫描、排除诊断及客户端扫描入口；songloft-org/songloft#573，基线 `4e73b078d86472e0c3409d24a87a6aca2b82380a`。
- 证据：报告人确认将 `yt-dlp` 加入扫描排除是为了避免单曲目录生成大量歌单；子目录定向扫描继承该规则，旧接口返回成功启动，最终没有发现歌曲。隔离移除启动校验后，新回归观察到 200 和后台任务；移除遍历诊断后，实际剪枝目录没有记录。
- 规则与 Done right：复用扫描匹配器返回路径和命中规则；全排除不启动，混合目录正常扫描并解释跳过项。诊断只数被剪枝目录，不能为估算歌曲数额外遍历。保留排除及清理语义，不增加临时豁免；只排除自动歌单用现有 `auto_create_exclude_dirs`。真实 HTTP、客户端正反向测试与验证边界见[记录](docs/issue-573-scan-exclusions.md)。
- Seen 1× (2026-11-17)。状态：本地实现与验证完成，用户随后授权提交、推送与 Issue 回复，具体交付以本批记录为准；不把本地夹具当成报告人 Docker 验收。

### HTTP-PROXY-001：插件的独立 Transport 必须显式接入全局代理

- 范围：QuickJS `fetch`、`songloft.net.httpStream` 与公共代理契约；[songloft-org/songloft#575](https://github.com/songloft-org/songloft/issues/575)，基线 `8177b2f208d8436f24a97e08a4ae237f372f88c5`。
- 证据：设置端点已保存代理，运行时普通及跳过 TLS 校验的 Transport 却没有 `Proxy`；旧实现下四组请求回归失败且代理命中为 0，回环直连对照通过。仅检查代理配置保存或插件商店客户端不能证明插件 `fetch` 已接入。
- 规则与 Done right：两种独立 TLS 连接池都按请求调用 `httputil.ProxyFromGlobal`；保留重定向、权限和回环语义。已有 JS 环境切换 / 清空代理回归及 race 检查通过；正式 lyrics 插件的精确 / 模糊搜索经真实 CONNECT 隧道取得夹具歌词，本地自定义源仍直连。完整记录见[验证记录](docs/issue-575-verification.md)。
- Seen 1× (2026-11-17)。状态：本地修复与验证完成；提交、推送及 dev 构建状态见本次交付记录。报告人的 Docker、mihomo 和线上 LRCLIB 未验收。

### SCAN-ORDER-001：自动歌单空更新也会经触发器改变排序

- 范围：自动歌单重建与默认列表排序；[songloft-org/songloft#573](https://github.com/songloft-org/songloft/issues/573)，基线 `8177b2f208d8436f24a97e08a4ae237f372f88c5`。
- 证据：真实内存 SQLite 中，恢复旧生成查询后，两项回归分别暴露相同 position 的旧歌单因更新时间改变而重排、新自动歌单因默认 position=0 插到手动歌单之前。只删除 UPDATE 中显式的时间赋值仍会触发数据库更新时间，不能解决空写问题。
- 规则与 Done right：元数据无变化时不执行 UPDATE；新增自动歌单与普通歌单一样在事务内追加位置。验证同时覆盖历史同位置数据和新增目录，并确认成员/真实封面变化仍更新、用户位置及置顶偏好保留。
- 验证：最终排序回归在旧查询 overlay 下两项按预期失败，新实现全量 `make check`、自动歌单测试及真实文件搬移/定向扫描/手工添加 HTTP 场景通过；详见[验证记录](docs/issue-573-verification.md)。歌曲不可见的原始报告尚未复现，不能把排序修复当作整个 Issue 已解决。
- Seen 1× (2026-11-17)。状态：排序部分修复与本地验证完成；提交、CI 及版本可用性以 #573 后续回复为准。

### LICENSE-001：跨仓库许可证核对要覆盖正文、包元数据与分发副本

- 范围：主题、插件、工具链及客户端声明；主仓库基线 `44309463af527e5aff5273ef7abc74d820b8a6d3`。
- 证据：2026-11-17 的本地 21 仓库检查发现八个子仓库缺少根 LICENSE；Subsonic README 写 MIT，而 `plugin.json` 自初始提交即声明 Apache-2.0；工具链七个包自创建即声明 MIT，不能仅凭根 README 不同就判为遗漏。
- 规则与 Done right：沿实际资源/包路径核对声明和正文；已声明 Apache-2.0 的仓库补同一标准正文，npm 包按自身元数据放置 LICENSE，并用 `npm pack --dry-run --json --ignore-scripts` 检查实际收录。工具链 README 明确七个 MIT 例外，保留第三方许可证；脚手架基础模板也带 LICENSE。
- 验证：八个根 LICENSE 与主仓库正文逐字节一致，12 个工具链 npm 包均收录匹配正文；本地证据、工作树标识与相关 Not run 见 `/tmp/songloft-license-fix-20261117/verification.json`，提交 SHA、推送及自动流水线结果另见同目录 `delivery.json`。
- Seen 1× (2026-11-17)。状态：本地修正完成，按本批授权提交、推送并核对自动流水线。

### LYNX-FALLBACK-001：WebView 回退路由必须连接实际分发的运行时

- 范围：Flutter 打开 Lynx 插件的公共 HTML 回退，主仓库基线 `39cc87db2a1b92f0be2d96741dc3a1178a1ea7c3`，Flutter 基线 `7fc5a04e4c65dfe54b0df299671bbecd2b4f8478`。
- 证据：现有 Docker 的 `/api/v1/jsplugin-assets/web-core/static/js/client.js` 返回 404，而内嵌 `/web-core/static/js/client.js` 返回 200；旧路由只查独立磁盘目录。Android 14/API 34 x86_64、系统 WebView 113.0.5672.136 的隔离回退页复现白屏，`lynx-view` 未定义；复用内嵌资源后，miot 2026.11.16.2 正式包显示设备、歌单和设置入口并发起业务 API 请求。
- 规则与 Done right：在应用装配时向插件管理器注入内嵌 web-core FS，插件公共路由直接提供 JS/CSS/WASM；保留磁盘整套覆盖，不混用不同版本文件。固定名称文件使用 `no-cache`，不能因运行时升级仍缓存一年。开发环境手工放过 web-core 的成功不能代表正式镜像可用。
- 验证：`make check`、Swagger 生成、资源/路径/MIME 回归通过；隔离恢复仅磁盘读取时四项资源用例均因 404 失败，安卓页面同时恢复白屏。证据 `/tmp/songloft-flutter-lynx-20261117/`：`check.log`、`assets-before.log`、`webview-{before,inspect-final}.log`、`before-webview.png`、`android-webview.png`；最终工作树标识见 `worktree-final.json`。
- 边界：系统 WebView 截图来自最小 Android 宿主，ADB 实际点击可进入设置（`android-settings.png`）。完整 Flutter debug 构建在 Gradle 依赖解析期间主动停止，未产出 APK（`flutter-debug-build.log`）；完整 Flutter 包交互、真实手机、音箱发声、iOS/macOS/Windows 为 Not run。lite 或旧 Flutter 嵌入构建仍需独立 web-core 目录。未替换用户 Docker；提交、推送与同 SHA CI 另行核对，正式版本及部署另行验收。
- Seen 1× (2026-11-17)。状态：服务端修复及系统 WebView 对照验证完成。

### CI-004：Flutter Linux 发布必须固定构建系统基线

- 范围：标准 Flutter Linux 与后端 Linux Bundle；主仓库基线 `3db91f69c1c00eecf04f630ba425df91c6fb5544`，Flutter `cebc894cde359a0d366c23d9a3acc75ade7832f2`。
- 证据：[失败 Linux job](https://github.com/songloft-org/songloft/actions/runs/43882625312/job/133353770838) 的 `ubuntu-latest` 已切换到 Ubuntu 26.04，安装 `libayatana-appindicator3-dev 0.5.94-1build1` 后，`tray_manager_plugin.cc:118` 的 `app_indicator_new` 因 `-Werror,-Wdeprecated-declarations` 编译失败。此前成功发布使用 Ubuntu 24.04。
- 规则与 Done right：两个 Linux 发布 job 同时固定 `ubuntu-24.04`，保留现有编译告警策略和发行版兼容基线；不能只修标准版而让 Bundle 继续跟随 latest 漂移。本次仅修改 runner 选择。
- 验证：actionlint v1.7.12 检查两个工作流通过；原始日志 `/tmp/songloft-release-2.13.11-20261116/linux-baseline.log`。正式版本的实际 Linux 构建另行验收；Not run：桌面安装和托盘交互，运行时代码未修改。
- 远端验收：`v2.13.11` 的 [Flutter Linux job](https://github.com/songloft-org/songloft-player-flutter/actions/runs/43887627188/job/133369948089)（源码 `7fc5a04`）与 [Linux Bundle job](https://github.com/songloft-org/songloft/actions/runs/43892661200/job/133389054723)（源码 `da5d0da`）均实际使用 Ubuntu 24.04 并成功构建；标准版 tar.gz/DEB/RPM/AppImage 四项产物齐全。两端完整正式发布 CI 通过，版本、提交标识及资产清单回读正确；日志 `/tmp/songloft-release-2.13.11-20261116/{flutter,backend}-linux-fixed.log`。
- Seen 1× (2026-11-16)。状态：固定基线的实际构建及正式版本发布已验收；桌面实际安装和托盘交互仍 Not run。

### LYNX-FOCUS-001：scroll-view 委托焦点会改变长表单点击目标

- 范围：公共 Lynx HTML 回退与 Lynx Web 嵌套插件，基线 `e2bfea9b1b2ccce65176f27e9fa0968dbc14acd9` 加工作树。
- 证据：真实浏览器 pointerdown 命中高级设置后，scroll-view 的 delegatesFocus 将焦点转移到首个密码框，滚动归零，mouseup/click 变成祖先；仅等待或修业务 blur 无效。去除宿主 guard 的隔离反向检查使新增用例失败。
- 规则与 Done right：宿主对非文本控件的鼠标按下阻止默认委托焦点，并 blur 当前输入；保留输入路径本身的默认编辑行为。服务端 shim 与客户端同名副本同步，实际嵌套 frame 和回退均操作长表单。证据见 miot `docs/lynx-migration.md`；3 项 shim 回归通过，原生触摸验收仍是 Not run。
- 同日封面复核：用户 Docker 的旧桥接没有 resourceUrl；相同实际歌单 30 行封面全部为空。新增宿主能力后按授权提交主程序，保留图片直接请求，避免插件后端 Base64 兼容造成额外开销。UI 和插件源码更新不能替代宿主更新，验证必须记录实际运行的宿主版本。
- Seen 1× (2026-11-16)。本轮按用户明确授权提交主程序；发布和设备边界见交付记录。

### ALBUM-001：专辑顺序必须在分页前统一解释音轨与碟号

- 范围：专辑列表、ID 队列、扫描及标签契约；songloft-org/songloft#571，基线 `7180df38176ca7b44c1a7d8cf2b3ec269d35ac7d`。
- 证据：原 HTTP 默认添加时间，Lynx 还显式请求该排序；保存轨号不等于使用轨号排序。恢复旧排序机制后，数字 1/2/10、多碟、分页与 ID 列表回归均失败。
- 规则与 Done right：完整歌曲与 ID 查询在 LIMIT/OFFSET 前共用数字碟号/轨号排序；核对客户端显式参数和后台补齐队列。迁移只加字段，既有碟号须通过重新导入读取；标签写回保留旧调用者没有提供的碟号。真实 SQLite、六格式标签、扫描/重扫、浏览器列表及播放队列证据见[验证记录](docs/issue-571-verification.md)。
- Seen 1× (2026-11-16)。状态：实现及本地验证完成；最终提交、推送及 CI 见 #571 交付回复，未创建正式版本，真机未验收。

### CACHE-001：远程缓存的落盘与命中必须共用音源校验

- 范围：外链下载、流式代理落盘、远程缓存命中与插件换源；songloft-org/songloft#570，基线 `6b3f38c923cf15358a1e7156c6290cac9d43b8c2`。
- 证据：HTTP 200 + 有效音频格式不能证明拿到了整曲；真实 10 秒 MP3 在旧逻辑 overlay 中经 200 / 206 两路持久化，播放接口还会通过 `song.CachePath` 直接返回旧提示音。恢复旧旁路后 13 个子场景失败，正常音频对照通过。
- 规则与 Done right：在统一移动入口调用现有 `source.Validate`；命中时校验，失败淘汰原文件及衍生文件并清理数据库路径。记忆技术参数须绑定文件身份、大小和修改时间，每次仍应用当前阈值与预期时长；取消不能当成坏文件。插件校验失败上报健康度并触发现有去重换源。真实 SQLite、HTTP、备用源写回及三轮 race 检查通过，详见[验证记录](docs/issue-570-verification.md)。
- 边界：后台整首校验不能撤回首播已传出的提示音；纯外链无跨插件候选，所有源失败时的播放器重试另行处理。macOS/OrbStack arm64、真实网易音源和小米音箱 Not run。
- Seen 1× (2026-11-15)。状态：本地修复与验证完成；提交、CI 与版本可用性以 #570 的交付回复为准。

### HLS-001：跨域 CDN 资源需要可验证的代理授权

- 范围：HLS 电台播放列表、子列表、分片/key/init 与重定向；songloft-org/songloft#567，基线 `6115afb969ff1f4b7ced0fbe39d223350adb9e5c`。
- 证据：旧实现的隔离 overlay 在跨域子列表、切片/key 和不同端口回归中失败；仅代理同源资源会让播放器继续直连 CDN。报告人的线上源未提供，不能把本地夹具验证等同于该实例验收。
- 规则与 Done right：实际播放列表中的跨域 HTTP(S) 引用生成 HMAC 签名，绑定歌曲 ID、当前源 URL、端点类型和完整目标 URL；无签名请求保留同源校验，签名不绕过私网检查。入口与每次重定向均检查主机；相对引用以重定向后的最终 URL 解析。移除签名、入口私网检查或重定向检查的三个独立 overlay 均被 HTTP 回归抓到。详见[验证记录](docs/issue-567-verification.md)。
- 同次真实源补验：直播 `currentTime` 可从非零时间开始，不能只断言超过固定阈值；本次初始时间已约 54 秒但视频帧仍为 0。正确验证同时采样进度与解码帧数：CCTV-1 实播从 56.883789 到 58.927471 秒、30 到 81 帧，另核对真实分片直连/代理 SHA-256 相同；详见上述记录。
- Seen 1× (2026-11-14)。状态：实现与本地验证完成；提交、推送、CI 及版本可用性以 #567 后续交付记录为准。

### EVENT-001：成员变更事件必须来自已提交行

- 范围：歌单服务、QuickJS 事件及 SDK，songloft-org/songloft#563；基线 `c686994c13cb4a6611a29ef410b3f13236516158`。
- 规则：批量写入按已提交批次的实际 ID 发事件，不能使用输入列表或回滚前的计数；整表替换只发提交后的成员差集。单次大批量操作整批入队，避免仅因成员数超过队列容量而丢事件。空闲订阅身份只用于唤醒，新 VM 必须经当前权限重新注册。
- 证据与 Done right：真实 SQLite 外键失败回归在恢复旧批量方法的 overlay 下错误报告 501 行，实际仅 500 行提交；移除通知后回归缺少 add 事件，移除重载校验后回归继承已撤销权限。修复后的服务、HTTP、QuickJS 生命周期与超过队列容量的批量投递验证见[记录](docs/issue-563-verification.md)。
- 同次补充：成员时间必须独立于曲库时间；原子替换不得重建保留行。恢复旧清空／重建逻辑后，真实 SQLite 的保留时间回归失败。收藏失效通知在事务提交后唤醒长轮询，HTTP 优雅关闭前释放等待者；客户端经验记于子仓库。
- Seen 1× (2026-11-11)。状态：新功能机制验证及旧计数缺陷复现；提交前验证对象见记录；提交、推送与 SDK 发布以本批交付记录为准。

### COVER-001：动态封面候选不能共用过期上下文或修改时间

- 范围：歌单远程／本地封面回退，songloft-org/songloft#562。
- 规则：总远程预算只约束网络尝试，不能取消后续本地候选读取；动态地址更换候选时，旧文件的 `Last-Modified` 不能验证新文件。缩略图 ETag 须包含文件身份。
- 证据与 Done right：审核新增真实 HTTP 超时回归，两条路径在修复前均错误返回 404；更换为修改时间较早的成员封面时，旧实现误返回 304。分离远程上下文、继续读取本地候选，并在动态回退移除日期校验后全部通过。日志 `/tmp/songloft-562-review-{before,covers}.log`，详见 [验证记录](docs/issue-562-verification.md)。
- Seen 1× (2026-11-11)。状态：本地审核发现并修复，未提交或发布。

### COVER-002：封面代理应在出站前拦截本服务的封面引用

- 范围：歌曲和歌单远程封面，基线 `00162dd`。 [#579 用户报告](https://github.com/songloft-org/songloft/issues/579#issuecomment-7082650501)
- 证据与做法：受控 HTTP 夹具在修复前收到了相对路径、同实例绝对地址和重定向指向的封面请求。保存时拒绝已知的本服务封面引用，读取时在首个请求和每次重定向前检查，并对远程封面设置满载即返回的并发上限。测试断言拒绝路径没有出站请求，插件图片和歌单本地候选仍可读取。
- 同次封面超时验证：同一坏地址原先连续两次都等约 5 秒，增加 30 秒有界失败缓存后第二次直接回退。10 个不同坏地址并行时总耗时约 5 秒，1.5 秒的有效图片仍能返回，因此首个请求沿用原预算。
- Seen 1× (2026-11-23)。

### TAG-001：JSON 标签引用须与定义删除串行化

- 范围：共享歌单标签，songloft-org/songloft#562。
- 规则：普通 `INTEGER PRIMARY KEY` 会复用 ID；JSON 中的引用没有外键保护，删除、引用清理与绑定校验/写入要使用同一 UnitOfWork。元数据更新不得从旧对象恢复标签引用。
- 证据与 Done right：去掉清理的 Go overlay 在 ID 复用与旧对象回写回归中失败，保留 `tag:1`；实际实现通过无效批次回滚、系统标记保留、隐藏计数和跨实例备份测试。见 [验证记录](docs/issue-562-verification.md)。
- 同次审核补充：标签删除还须刷新客户端缓存的歌单元数据与绑定。Flutter 实际删除交互和 Lynx QueryClient 回归在修复前均读到旧 `tag:1`，补齐失效后通过；这不增加独立事件次数。
- 同次计数补查：歌单单个／批量删除及隐藏／恢复也须刷新标签目录中的可见歌单数；两端四种操作的缓存回归均在修复前失败。日志 `/tmp/songloft-562-review-counts-*-before.log`。
- Seen 1× (2026-11-11)。状态：本地实现，未提交或发布；这是机制回归验证，非新增线上事故。

### MOBILE-001：嵌入移动端后端必须先配置可写临时目录

- 范围：`mobile.Start` 与调用默认 Go 临时目录的服务；songloft-org/songloft#559，基线 `f1c4936d5a6dda299dd26cc3178beb61d5e71dce`。
- 证据：Android Bundle 启动时报 `mkdir /data/local/tmp/songloft-hls-*: permission denied`；Go 1.26 的 Android 默认临时目录是 `/data/local/tmp`。用文件充当不可用 `TMPDIR` 的新回归在旧实现下于 HLS 初始化失败，日志 `/tmp/songloft-559-before.log`。
- 规则与修复：移动入口在任何服务初始化前创建沙盒 `<dataDir>/tmp` 并设置 `TMPDIR`；不能只修复某一个 `MkdirTemp` 调用。目录创建失败须在打开数据库前返回；停止后保留环境配置，由各服务清理自己的临时文件。说明见 [Bundle 路径约定](docs/architecture_frontend.md#deployment-modes)。
- Done right：Linux amd64 / Go 1.26.0，修复后真实 SQLite、HTTP 健康检查、两次启动/停止、临时文件创建与 HLS 目录回收通过；`CGO_ENABLED=0 go test -tags dev,lite ./...`、同标签 vet 和冻结导出面检查通过。日志 `/tmp/songloft-559-{after,tests,vet}.log`。
- 剩余验证：移动入口 `-race -count=3` 失败，暴露插件管理器异步 Start/Close 的既有竞态；隔离旧入口、使用正常系统临时目录的快速启停同样失败，证据 `/tmp/songloft-559-{race,baseline-race}.log`。本次未修改插件生命周期；Android/iOS 打包和真机 Not run（未配置工具链/设备）。
- Seen 1× (2026-11-06)。状态：权限修复与本地验证完成；提交、推送和 CI 状态以 #559 后续回复为准；竞态单独待处理。

### CI-003：Android Bundle 连续编译必须释放 runner 磁盘

- 范围：主仓库 Go gomobile + Flutter Android 发布；基线 `299c4254ae4d4dd3b91cbcfa0b82830869d48e6d`。
- 证据：2026-11-04，[v2.13.8 构建](https://github.com/songloft-org/songloft/actions/runs/41826126515) 在 `mergeReleaseNativeLibs` 仅剩 76 MB，随后报 `No space left on device`；服务器/Docker 成功不能证明 Android Bundle 已发布。
- 规则与修复：在 GitHub 托管 Ubuntu runner 上先移除本任务不用的 .NET、Haskell 与 CodeQL 工具链；AAR 生成并检查非空后清理 Go 编译/测试缓存，再进入 Flutter 构建。保留 Android SDK/NDK、Java、模块缓存和 AAR；输出磁盘用量，不跳过 ABI 或 JNI 校验。
- Done right：修复版本 `d34b8b8a7a720065d13fccf4497785d7fb066d5f` 的 [Android job](https://github.com/songloft-org/songloft/actions/runs/41864164882) 释放约 11 GB，进入 Flutter 前有 96 GB 可用；ARM64/ARMv7/x86_64 APK、三项 JNI 检查和前后端补丁全部通过并上传。远端 Quality、本地 Go 测试/vet、59 项脚本测试及 actionlint v1.7.12 通过。原生设备运行 Not run，本次仅修改构建资源管理。
- Seen 1× (2026-11-04)。状态：Android 构建修复已验收；首轮 Apple job 因 hosted runner 分配失败取消，2026-11-05 重跑后整轮 CI 通过，[v2.13.9](https://github.com/songloft-org/songloft/releases/tag/v2.13.9) 的 41 项资产完整发布。下载的版本文件和六份 Android 补丁清单均核对为 `d34b8b8`。

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
- Done right：本次经用户确认，在两客户端新增独立、按服务器与账号保存在本机的首页显示开关；按内置 ID 与 `built_in` 识别，不改收藏成员或标签。测试、实际浏览器及未跑项分别记录在 [Flutter](clients/player-flutter/docs/cn/issue-532.md) 与 [Lynx](clients/player-lynx/docs/project/issue-532.md)，最终结果绑定各自工作树，不外推为原生设备或已发布版本。
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
- 规则：父仓库 bump 脚本要求干净工作树，因此两个子模块指针必须先用独立 commit 落地；gitlink commit 必须用 pathspec 限定（`git commit -m ... -- clients/player-flutter clients/player-lynx`），否则 staged/无关改动会被裹带。链式解析依赖「stdout 末行 = v<版本>、日志走 stderr」的固定契约。
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

### RELEASE-003：生成 Swagger 时排除工作区内的演练副本

- 范围：后端 `make swagger` 和调用它的 `scripts/bump-version.sh`。
- 证据：2026-11-21，后端基线 `6899aaf`。`tmp/agent-*/` 下保留完整演练副本时，正式发版的 `swag init` 递归扫描并报 `cannot find models.ErrorResponse`。同一基线在隔离 checkout 中生成成功，继续发布为 `v2.13.13`（`57a9f6b`）。
- Done right：工作区内存在演练副本时，从干净 checkout 运行发版命令。使用 `swag init --exclude tmp -o <输出目录>` 在原工作区复核，生成的 `docs.go`、`swagger.json` 和 `swagger.yaml` 与发布 tag 逐字节一致。
- Seen 1× (2026-11-21)。状态：经验。

### BACKUP-001：备份版本升级须核对客户端双向校验

- 范围：歌单备份公共契约、Lynx Web 数据管理；主仓库基线 `2326b38`，Lynx `1de4735`。
- 证据：服务端 `BackupVersion = 2` 且接受 v1 / v2；Lynx `data-transfer.ts` 的 `validateBackup` 只接受 v1，同时用于导入文件与导出响应。Node 22.17.1 转译执行实际 `jsonMultipart` / `PlaylistBackupApi.exportBackup`，传输使用夹具：v1 两路通过，v2 两路均报 `invalid_json`。证据 `/tmp/songloft-doc-sync-backup-probe.{cjs,json}`，不是浏览器或真实服务端端到端验证。
- 规则与 Done right：更新备份格式须核对服务端导出 / 导入、客户端上传前及下载后校验；跨实例标签按名称映射，不能手改版本号。后续按用户要求允许 v1 / v2，旧代码的三项 v2 回归失败，修复后 28 项定向验证通过。实际 Web 下载的 v2 文件跨两个临时后端导入，标签 ID 从 1 映射为 2，歌曲与歌单引用正确；v1 成功，v3 上传前拒绝。证据 `/tmp/songloft-backup-v2/roundtrip.json`，完整检查见子仓库[验证记录](https://github.com/songloft-org/songloft-player-lynx/blob/main/docs/project/backup-v2.md)。
- Seen 1× (2026-11-15)。状态：兼容修复和真实浏览器验证完成，未提交或发布；同次后续验收不增加 Seen。

### LYNX-001：原生插件迁移必须验证完整 Web 回退链路

- 范围：Lynx SDK、工具链生成入口、服务端公共资源；主仓库基线 `4df1e97b3a7874d08d1d74ce799baa9295b512b4`。
- 证据：2026-11-15 bili 迁移中，原生/Web bundle 构建通过，但真实 Chromium 121 回退页因插件 `<base>`、相对 Worker 地址、缺失桥接模块及 COEP 下 srcdoc 初始化停滞而空白。去除回退页 COEP 的隔离对比能加载 bundle 并执行真实插件 API；记录见插件 `VERIFICATION.md` 与 `/tmp/songloft-bili-verification/`。
- 规则与 Done right：先安装桥接再加载绝对 bundle 地址；模块由公共资源端点提供，资源路径包含部署子路径，无版本桥接资源不得 immutable 缓存。原生 frame 和 HTML 回退分别验证渲染、真实 GET/POST 与持久化，不能仅依据编译通过。普通 WebView 保留 COEP；Lynx HTML 回退例外及隔离宿主跨源嵌入限制明确写入插件指南。
- Seen 1× (2026-11-15)。状态：本次工作树修复，未提交/发布；Flutter 真机及真实 B 站账号导入/下载未验。

### BACKEND-001：父进程退出清理必须等待 HTTP listener 注册

- 范围：桌面 Bundle 子进程与 `internal/app.App.Start`。
- 证据：2026-11-18 源码审查发现，parent stdin EOF 可早于 `App.Start` 创建 `http.Server`；若退出 goroutine 先调用 `Shutdown`，它会关闭数据库，随后 `Start` 仍可能开始监听，留下不可用的孤儿服务。
- 规则与修复：父管道应在应用初始化前开始读取，但退出清理必须等 HTTP Server 注册 listener 并进入 `Accept` 后再执行。新增 `StartWithReady` 与 `readyListener` 将就绪信号放在 Server 已追踪 listener 的边界；通用 `Start` 保持原契约。
- Done right：本次 Go 源已 `gofmt`，可在源码中检查 EOF → ready callback → `Shutdown` 顺序。Not run：Go 测试、构建、宿主崩溃/启动竞态进程验收。
- Seen 1× (2026-11-18)。状态：经验，跨平台进程验证待最终验收。

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
