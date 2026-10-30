# 业务约束与踩坑

本页保存主仓库业务约束、设计取舍和历史回归背景，供 `AGENTS.md` 按任务索引。修改相关模块前必须阅读对应章节；摘要不能代替完整约束。

代码路径均相对仓库根目录。新经验按[开发与复盘流程](ai-development.md)记录；不要把专题全文重新堆回入口文件。英文版见 [English](en/pitfalls.md)。

<a id="plugin-identity"></a>

## 插件商店的 entry_path 撞名（identity）

`entry_path` 不是「插件 ID」那么单纯 —— 它同时是 registry 去重键、安装态匹配键、
`js_plugins.entry_path` UNIQUE 约束、ZIP 文件名（`<entryPath>.jsplugin.zip`）、static 目录名
（`jsplugins_data/<entryPath>/static`）、路由前缀（`/api/v1/jsplugin/{entryPath}/*`）、
manager/scheduler 的内存 map 键，以及 `plugin_storage.plugin_entry_path` /
`songs.plugin_entry_path` 的归属键。两个不同作者的插件完全可能撞上同一个 entry_path
（songloft-org/songloft#339）。改这块前先读完下面几条。

- **本地无法共存同 entry_path 的两个插件**，这是数据层事实而非 UI 限制。#339 只修了商店层
  （让两条都显示、安装态各自准确、覆盖前二次确认）。真要共存得引入真正的 plugin id、把
  entry_path 降级为可消歧的路由前缀，要动 DB 约束、磁盘布局、路由、`EntryPathFromZipName`、
  `SyncPluginsFromDirectory` 的孤儿清理，还要迁移 `plugin_storage` / `songs`
- **身份（identity）= 规范化 author，author 为空时用 `update_url` 的 GitHub `owner/repo` 兜底**
  （`internal/jsplugin/identity.go`）。author 规范化要剥掉 `<邮箱>` 与 `(备注)` 再转小写
  ——同一插件在不同源里写成 `hanxi` / `Hanxi <a@b.com>` 很常见，不归一会把它分裂成两条商店条目。
  **非 GitHub 的自托管 URL 刻意不推断仓库**：那种路径布局任意，`/plugins/a/` 与 `/plugins/b/`
  会被当成两个仓库，把同一插件的两个镜像地址分裂开
- **`SameIdentity` 任一方为空时返回 true**（视为同一插件）。宁可漏报冲突，也不能因为对方缺
  author 就误报冲突拦住用户的正常升级
- **跨源去重键刻意不含订阅源 URL**（`FetchAndMergeMulti`）：官方源与社区聚合源经常同时收录
  同一个插件，把源并入键会让它在「全部」模式里重复显示多遍。identity 已足够区分真正不同的插件
- **手动上传路径刻意不做冲突检测**：`InstallFromUpload` 保持原行为，只有商店安装走
  `InstallFromUploadWithOptions(RejectIdentityConflict: true)`。手动上传是用户明确指定文件、
  意图清楚，且插件迭代中 author 写法变动不应导致上传失败
- **冲突时必须在写任何东西之前返回**：以前 `package.go` 撞名就静默走 `Update`——覆盖 ZIP、
  `os.RemoveAll(staticDir)` 后重新解压、原地改写 DB 记录（保留原 ID 与 status）。结果是原插件被
  无声销毁，新插件继承了它在 `plugin_storage` 里的全部数据，原插件导入的歌曲
  （`songs.plugin_entry_path`）也被记账到新插件名下
- **商店的 `has_update` 必须用 `CompareVersion(...) > 0`，不能用字符串 `!=`**：撞名时两边版本号
  本就不同，字符串比较会永久显示「可更新」，用户点下去就把本地插件换成了另一个作者的插件
- **前端就地更新安装态要按 `(entryPath, identity)` 匹配**（`RegistryPluginEntry.matches`）：
  只比 entryPath 会把同名的其他条目一起点亮成「已安装」。列表项的 `key` 用 `rowKey`
  （`entryPath|identity`）而非 index——`_RegistryPluginItem` 持有 `_installing` 本地状态，
  按 index 复用 Element 会让 loading 圈跑到别的条目上

<a id="registry-cache"></a>

## 插件商店的结果缓存

商店的分页与搜索都在**完整插件列表**上做（服务端切片 + 子串过滤），因此未加缓存时每次翻页、每次改搜索词
都会触发一次完整的注册表拉取——最多 500 个 `plugin.json`、8 并发、单请求 15s 超时。
`registry_cache.go` 给拉取结果加了 5 分钟的进程内 TTL 缓存。

- **`RegistryService` 必须由 handler 长生命周期持有**（`JSPluginHandler.registrySvc`）。
  以前是每个 HTTP 请求 `NewRegistryService()`，那样缓存永远命不中
- **`proxyDown` 因此必须是每次调用的局部变量**，不能挂在 `RegistryService` 上。它记忆
  「GitHub 代理本次已失效」，作为单例字段会让代理一次失败后**永久直连**（代理恢复也不再尝试），
  且并发请求互相干扰。私有方法签名都带 `proxyDown *atomic.Bool` 就是为此
- **缓存键必须含所有影响结果的输入**：模式（单源/全部）、源 URL、token、源顺序、`github_proxy`。
  源顺序不能省——`FetchAndMergeMulti` 按源顺序决定同版本插件由哪个源胜出。**token 进键前要
  哈希**，缓存键会进日志与调试输出，不能带明文凭据
- **刻意不缓存安装状态**：`installed` / `has_update` / `conflict` 每次请求都由
  `buildInstalledMap` 从 DB 实时算，所以装完插件立刻翻页也能看到状态更新，无需失效缓存
- **失败不写缓存**（也不动既有缓存）：否则一次网络抖动会把错误状态粘住整个 TTL。
  `FetchAndMergeMulti` 不返回 error（单源失败只进 warnings），所以那条路径改为**空结果不缓存**
- **前端只有「刷新」与「拉取失败重试」传 `force: true`**；翻页、搜索、首次加载、源切换都不传。
  源切换与改源配置靠缓存键变化自然重拉，`UpdateRegistriesSetting` 另外显式 `InvalidateCache()`
  腾出条目配额

<a id="scan-title"></a>

## scan 标题规则

- 默认 `scan_title_source=tag`：标签有 title 就直接使用，否则取文件名去扩展名。
- 显式设置 `scan_title_source=filename` 时始终使用文件名，不被标签覆盖；分支在 `internal/services/metadata.go`。
- **不要**再做"最长公共子串去重 + 拼接"，会产生"艺术家 - 标题"这种把艺术家冗余到标题字段的结果
- 视频容器探测：扫描 mp4/mov/m4v/mkv/webm/avi/ts/mpg/mpeg/flv/wmv/rm/rmvb/3gp 等容器时用 ffprobe 探测是否含真实视频轨（排除封面 attached_pic）来置 `songs.is_video`，客户端据此渲染画面 / 选择投屏 mime
- **本地起服务验证时，music 目录不要放在 `/tmp` 下**：`music_path` 的默认 `exclude_dirs` 含 `tmp`，
  而 `ShouldExcludeDir` 是**按路径任一层级的目录名**匹配的，于是整个 `/tmp/...` 根目录被排除。
  表现是扫描「成功完成」但 `discovered_files=0`，**不报错、不打 warn**，极易误判为自己的改动坏了

<a id="scan-batches"></a>

## scan 流式入库（大曲库铁律 — songloft-org/songloft#447）

`doScanAndImport` 的入库是**流式**的：`scanBatchAccumulator`（`scan_batch.go`）按目录分组，
目录定稿即 `flushScanBatch`。别改回「全量累积后一次性写」——几万首的库会在整个提取阶段
（数十分钟）里进度恒为 0、数据库一条不写，用户只能判定为卡死。

- **定稿单位必须是目录，不能是固定条数**：`fixSpamTags` 按目录分组、以「同目录内超过一半文件
  共享同一 `(title, artist)`」判定垃圾 tag，需要整个目录作样本。按 50 条切批会把一个目录劈成
  几段，判定结果随切分位置漂移
- **`scanDirGroupCap` 是内存兜底**：平铺几千文件的目录到达上限就提前定稿，否则退化成全库累积。
  代价是该目录的垃圾 tag 判定改用子样本，可接受（判据是 >50% 且 ≥3 条）
- **进度计数只在批次落库后累加**（`flushScanBatch` 里的 `UpdateProgress`）。提取阶段单独调
  `SetCurrentFile` 上报当前文件，别在提取阶段也走 `UpdateProgress`，会双重计数
- **写库的 ctx 与取消的 ctx 必须分开**：`doScanAndImport` 会把入参 ctx 派生成可取消 ctx（取消时
  杀 ffmpeg 子进程），但 `flushScanBatch` 必须用派生前的 `flushCtx`。取消时要把已提取的成果落库，
  用被取消的 ctx 会让事务立刻失败、重试三次后全部记为 failed，「取消保留成果」静默不生效
- **取消不再丢弃已提取结果**（v2.12.0 之后的行为变更）：worker 退出前的结果照样入库，
  `sealAll` 负责冲出 pending 归不了零的残余目录组。以前整批丢弃，用户连续取消两次就白干二十分钟

<a id="directory-walk"></a>

## 目录遍历不要逐条目 `os.Stat`（songloft-org/songloft#447）

`scanDirWithCue` / `collectDirNames` 用 `resolveEntry`（`scanner.go`）判断目录项类型：
`os.ReadDir` 返回的 `DirEntry.Type()` 在 Linux 上直接来自 `d_type`，零额外系统调用；
只有软链接与未知类型才回退 `os.Stat`（跟随软链接，保持「软链接目录仍递归、断链跳过」的语义）。

- 逐条目 `os.Stat` 会让万首级曲库多出几万次系统调用。`GET /scan/dir-names` 就是因此超过客户端
  10s 超时 → `r.Context()` 被取消 → 遍历半途中止 → 500，且**每次重试都从零重来**
- 该端点因此还有两层保护：`CollectAllDirNames` 的 60s TTL 缓存（`dirNamesMu` 兼作单飞锁，
  挡住前端重试风暴），以及 handler 侧的 `context.WithoutCancel(r.Context())`——脱开客户端取消，
  让首次遍历跑完并填上缓存，重试立刻命中。缓存挂在 `Scanner` 上，`music_path` 变更时
  `app.go` 重建 Scanner，缓存自然失效
- d_type 不可用的文件系统上 `ReadDir` 内部会自行 lstat 补齐类型，等价于原行为，不会变慢

<a id="dsf"></a>

## DSF（DSD）扫描

- `dsf` 是默认扫描格式；`app.go` 两处回退列表和迁移 `0039_scan_support_dsf.sql` 保持同步，迁移为已有配置幂等追加（大小写无关）。
- `pkg/tag` 可读取 DSF 元数据与封面；DSD raw 采样率可能是 ffprobe PCM-equivalent 的 8 倍，已有元数据测试处理该差异。
- 播放沿用 libmpv / ffmpeg；浏览器与不支持 DSD 的 DLNA 设备需显式传 `?format=mp3` 等转码参数，当前 Flutter Web 的 `AudioFormatHelper` 不会为 DSF 自动追加。DSF 不在标签写入矩阵中，写入失败应沿用现有降级处理。

<a id="sidecar-lyrics"></a>

## 旁挂歌词（.lrc）

- **匹配规则**（`FindSidecarLyricFile`）：`<base>.lrc` / `.LRC` / `.Lrc`，然后 `<含扩展名>.lrc` / `.LRC` / `.Lrc`。
  前者优先。**禁止**改成 `ReadDir` + `EqualFold` 遍历（O(歌曲数×目录项数) 不可接受）。
- **空文件视作未命中**：`st.Size()==0` 或目录同名均跳过。理由：防止空 lrc 把 `lyric_source` 打成 `file` +
  `lyric=""` 而前端永不请求、插件也无法兜底的死角。
- **编码处理**（`ReadSidecarLyric`）：UTF-16 LE/BE 按 BOM 识别 → `x/text/encoding/unicode` 解码；
  其余走 `tag.FixEncoding`（GBK 系修正）。
- **扫描跳过的三级短路**（`needsSidecarLyricImport`）：
  ① `LyricSource ∈ {file, manual}` → false（无 IO）；
  ② 目录不在 `ScanResult.LyricDirs` → false（内存 map）；
  ③ 该歌曲对应的 lrc 确实存在 → true（最多 6 次 Stat）。
  收敛性：命中后 `lyric_source=file` → 下一轮走 ① 短路。
- **运行时优先级链**（`GetSongLyric` handler）：
  sidecar .lrc > DB url > DB payload > 歌词搜索插件。`manual` 不被旁挂覆盖（`SidecarLyricForSong` 排除）。
- **`SyncSidecarLyric` 不回写音频标签**：.lrc 本身就是持久化载体，嵌入标签后用户删 lrc 会留下删不掉的过期副本；
  且 `WriteSongTags` 是重建模式会读写封面二进制，性能代价大。
- **`shouldApplyScanLyric` 护栏**：`manual` 不覆盖；新歌词为空时不抹掉库中已有歌词/远程URL。
  这是行为变更（以往重新导入可清空歌词），已写入 CHANGELOG。
- **旁挂命中会整体替换**插件带来的翻译/罗马音（`tlyric`/`rlyric`/`lxlyric`），不做"主歌词用文件+翻译沿用插件"的合并——
  两份歌词时间轴不同源，错位比没有翻译更糟。

<a id="tag-writing"></a>

## tag 写入（pkg/tag）

- `tag.WriteTag(filePath, opts)` 按扩展名 dispatch，所有格式均使用临时文件 + `os.Rename` 原子写入
- 支持矩阵：

| 格式 | 文本字段 | 歌词 | 封面 |
|------|---------|------|------|
| MP3 | ID3v2.3 text frames | USLT | APIC |
| FLAC | Vorbis Comment | LYRICS | PICTURE block |
| M4A/MP4/M4B/M4V/MOV/3GP | iTunes atoms (©nam 等) | ©lyr | covr |
| OGG(.ogg/.oga/.opus) | Vorbis Comment | LYRICS | METADATA_BLOCK_PICTURE (base64) |
| APE | APEv2 text items | Lyrics | Cover Art (Front) (binary item) |
| WAV | RIFF LIST INFO | ICMT | **不支持**（格式限制） |
| AIFF/AIF | ID3v2.3 (ID3 chunk) + NAME/AUTH | USLT (ID3 chunk) | APIC (ID3 chunk) |
- 不支持的格式 → 返回 `ErrUnsupportedWrite`，调用方**必须**降级为日志，**不要**阻塞主流程

<a id="fingerprints"></a>

## 音频指纹（fingerprint — 开销控制铁律）

指纹（ffmpeg chromaprint）只服务两处：设置页「重复歌曲检测」和插件歌词/封面搜索的**可选**参数。
它是按需功能，**不是**扫描的必要环节。改这块前先读完下面几条，`songloft-org/songloft#323`
就是这些约束缺位叠加出的「扫描显示完成但 CPU 永久 100%」。

- **扫描后自动指纹默认关闭**：业务端点 `GET/PUT /api/v1/settings/scan-auto-fingerprint` 体 `{enabled: bool}`，
  config key `scan_auto_fingerprint`，默认 `false`。`runAutoFingerprint`（`song_service.go`）开头判定，
  与 `scan_auto_create_playlists` 对称。关闭时用户在重复检测页手动 `POST /scan/fingerprints`
- **失败必须落库标记**：`songs.fingerprint_attempted_at`（unix 秒，0 = 未尝试）。
  `ListLocalWithoutFingerprint` 的条件是 `fingerprint = '' AND fingerprint_attempted_at = 0`。
  **绝不能**只在失败时打日志——没有标记，AutoScanner 每轮（默认 3600s）都会把同一批注定失败的
  长音频/无音轨文件重新捞出来跑 ffmpeg 全解码。`ClearAllFingerprints` 会重置该标记，
  所以「重新计算全部」是重试失败项的唯一入口
- **采样上限 120 秒**：`ExtractFingerprint` 带 `-t 120`（常量 `fingerprintSampleSeconds`），
  这也是 AcoustID 的事实标准。**不要**去掉——30 分钟有声书全解码在弱 NAS 上必然超时，
  实测同一文件全长 3.8s vs 120s 采样 0.35s。超时 30s + `cmd.WaitDelay` 防 ffmpeg 子进程挂住 worker
- **CUE 轨按区间采样**：CUE 轨的 `file_path` 指向整轨镜像，必须传 `cue_start_seconds/cue_end_seconds`
  走 `-ss`，否则同一镜像下所有 track 拿到**完全相同**的指纹并互判重复
- **并发按 CPU 自适应**：`fpWorkerCount()` = `clamp(GOMAXPROCS/4, 1, 4)`。**不要**改回硬编码 4，
  Go 的 GOMAXPROCS 感知 cgroup 限额，Docker 限核能正确收敛
- **可取消**：`POST /api/v1/scan/fingerprints/cancel` → `FingerprintService.Cancel()`。
  指纹任务用独立的 `context.Background()`，**不能**复用扫描的 cancelCh——`scanProgressManager.Complete()`
  已经 `close(cancel); cancel = nil`，之后 `GetCancelChannel()` 返回 nil。取消中途的歌曲不打
  attempted 标记，下次续算
- **去重带时长护栏**：`ListDuplicateGroups` 在同指纹内还按 `fingerprint_duration`（全片时长）
  以 30 秒容差**相邻**聚簇（时长为 0 = 未知则不切）。因为只采样前 120 秒，「统一片头的有声书」存在指纹碰撞可能；
  护栏刻意保守——漏报真重复比误报更不可接受，用户会照列表删文件
- 迁移 `0029` 已把旧的全长指纹一次性清空（与 120 秒采样不可比），升级后需重算一次
- `IsChromaprintAvailable` 走 `SetFingerprintFFmpegPath` 注入的 `ffmpeg_path` 配置，不再只查 PATH

<a id="play-history"></a>

## 播放历史（play_history — 按播放上下文）

每个「播放上下文」独立记住最近播放过的歌曲（songloft-org/songloft#333）。上下文 = `(context_type, context_key)`：
歌单是 `("playlist", "<id>")`，分面维度是 `("artist", "周杰伦")` 这类；还支持 `tag`。合法类型以
`IsValidPlayContextType` 组合的 `models` 常量与 `database.IsSongFacetField` 为准，**不另抄一份枚举**。改这块前先读完下面几条。

- **序数不可靠，所以读端点不返回「第几首」**：分面列表走 `GET /songs?artist=X`，默认排序 `added_at DESC`，
  而 `added_at` 是秒级 `DATETIME`、批量扫描在单事务里顺序 INSERT → 成百上千首歌 `added_at` **完全相同**；
  `applyOrder`（`filters.go`）只产出单列 `ORDER BY`、**无 `id` tie-break**。据此算「第 N 首」在数学上不确定，
  会起播到错的歌。**不要**为此去改 `applyOrder` 加 tie-break——那会动既有分页行为，属独立 issue
- **`absIndex` 与分页 offset 的一致性依赖 SQLite planner**：`/songs/ids`（一次全排）与
  `/songs?limit&offset`（可能走 bounded sorter）是两个查询，并列 `added_at` 下的相对顺序全靠
  planner 恰好一致（实测 350 / 30000 首均一致）。真要收紧就得给 `applyOrder` 默认排序补 `, id DESC`
  ——那会动所有 `/songs` 分页调用方，属独立 issue。同理 `ListSongIDsOrdered` 与 `GetPlaylistSongsPaginated`
  都只按 `position ASC` 无次级键，仅在同歌单出现重复 position（并发 reorder 中断）时才会错位
- **客户端起播 = 首曲直起 + 后台环形补齐**：历史条目自带完整 `Song`，先用它当队列**零请求**出声，
  再后台拉有序 ID 列表（歌单 `GET /playlists/{id}/song-ids`、分面 `GET /songs/ids`）`indexOf` 定位，
  依次补「目标之后」与回卷的「开头…目标之前」。`currentIndex` 全程为 0 不动，
  不牵连随机模式的 `_playedIndices` / `_preSelectedNextIndex`；歌单与 7 个分面走同一条代码路径
- **只有 `type=play` 落库**：写入挂在既有打点端点 `POST /songs/{id}/played` 的 `context_type`/`context_key`
  参数上（不新开第二条打点通道，客户端零额外请求）。`finish` 是同一首歌的重复写；**`skip` 尤其危险**——
  它上报的是**上一首**歌，此时客户端上下文可能已切到新歌单，会把上一首错记到新上下文名下。
  前端对 skip/finish 不传 context，后端也只认 `play`，双重保险。落库失败只 `slog.Warn`，**响应码永远 204**
- **上限 50 条，upsert + 裁剪同事务**：去重靠 `UNIQUE(context_type, context_key, song_id)` + `ON CONFLICT DO UPDATE`，
  不在应用层查重。裁剪用 `id NOT IN (... ORDER BY played_at DESC, id DESC LIMIT ?)`——`played_at` 秒级会撞，
  `id DESC` 做确定性 tie-break。`MaxPlayHistoryPerContext` 是 Go 常量，**不要**做成配置项
- **`context_key` 是 TEXT，无法对 playlists 建外键**：删歌单靠 `PlaylistService.clearPlayHistory` 显式清理。
  **批量删除时只能清理真正被删掉的歌单**——仓储层会跳过 `built_in`，若无脑遍历入参 ids 就会把「收藏」
  的历史一并清空（实现时踩过，已由 `TestPlaylistDeleteClearsPlayHistory` 覆盖）
- **`sourcePlaylistId` 不能改签名**：它是 JS 插件公开契约（`plugin_host_dispatch.dart` 导出 `source_playlist_id`）
  且有 5 处「正在播放」高亮消费点。前端泛化成 `PlaybackContext` 时把它降级为**派生 getter**，
  故上述消费点零改动。prefs 读兼容旧 `player_source_playlist_id`、写时双 key 并存，保证 Android 热更回滚安全

<a id="hls-proxy"></a>

## HLS 电台代理模式（/settings/hls-proxy）

- 业务开关端点：`GET/PUT /api/v1/settings/hls-proxy` 体 `{enabled: bool}`，默认 `false`
  - `false`：电台 `.m3u8` 直接 302 给 player，由 player 自己拉源站。零开销但受源站防盗链/CORS 限制
  - `true`：服务端拉取并改写 m3u8、代理所有切片/key/init 段。**所有切片走本机带宽**，注意流量成本
- 切换时机：源站 Referer/UA 防盗链导致播放失败 / Web 嵌入模式 CORS 阻塞时，开启代理
- 反代端点：`/api/v1/songs/{id}/hls/playlist?u=<base64url>` 和 `/api/v1/songs/{id}/hls/segment?u=<base64url>`
- HLS 电台 song.url 强制带 `.m3u8` 后缀（`/api/v1/songs/{id}/play.m3u8`）：ExoPlayer/AVPlayer 按 URL 后缀选 MediaSource，无后缀会落到 ProgressiveMediaSource 导致直播无法播
- 改写规则：经典 HLS + LL-HLS 全集（PART/PRELOAD-HINT/RENDITION-REPORT）+ `EXT-X-DATERANGE:X-ASSET-URI`（HLS Interstitials 单 URI）。`X-ASSET-LIST`（JSON 子代理）暂未实现，遇到时原样透传
- 安全：每次端点入口做"同源校验（scheme+host+port 与 song.URL 严格相等）"作第一道防线，`services.IsHostnameAllowed` 作 SSRF 兜底。**非同源 URL 保持原样不改写**，避免成为开放代理
- player 跨域：改写后的 URL 全部是相对路径（`playlist?u=...` / `segment?u=...`），规避 BASE_PATH 子路径部署问题
- 上游 4xx/5xx 透传给 player；playlist 体上限 1 MB；首行必须 `#EXTM3U`

<a id="http-proxy"></a>

## 通用 HTTP Proxy（/settings/http-proxy）

- 业务端点：`GET/PUT /api/v1/settings/http-proxy` 体 `{proxy: string}`，默认 `""`（直连）
- 仅使用代理感知客户端的后端外发 HTTP 请求经此代理；不能假定任意 `http.Client` 或插件 `fetch` 自动接入。
- 典型值：`http://192.168.1.1:7890`（支持 HTTP/HTTPS/SOCKS5 代理）
- loopback 地址（`localhost`/`127.0.0.1`/`::1`）自动跳过代理，避免影响内部请求
- 与 GitHub 镜像加速（`github_proxy` URL 前缀拼接）**共存**：先拼接镜像前缀再经 HTTP Proxy 转发
- 实现：`internal/httputil/proxy.go` 的 `globalProxy`（私有 `proxyConfig`）通过 `SetGlobalProxy` / `GetGlobalProxy` 管理；`NewClient`、`NewStreamingClient`、`NewDownloadClient` 分别使用适合普通请求、流与下载的共享 Transport。
- 启动时从 config 表加载已保存的代理地址（`app.go`）；PUT 时即时生效无需重启
- 接入示例：`jsplugin/registry.go`、`jsplugin/package.go`、`services/upgrade_service.go`、`handlers/jsplugin_registry.go`、`handlers/hls.go`；完整范围以 `httputil.New*Client` 调用点为准。
- ffmpeg 远程拉流转码路径（`services/radio_transcode.go` 电台转码、`services/url_transcode.go` 转码代理）同样接入：经 `-http_proxy` 传给 ffmpeg。**仅 http/https 代理**，SOCKS5 不支持（ffmpeg `-http_proxy` 限制）

<a id="private-allowlist"></a>

## 私网代理白名单（/settings/proxy-private-allowlist）

- 背景：通用资源代理 `GET /api/v1/proxy?url=` 默认用 `services.IsHostnameAllowed` 封禁一切内网 / 回环 / 链路本地地址（防 SSRF），导致「公网 Songloft 代理仅私网可达的 WebDAV」被拒（songloft-org/songloft#313）
- 业务端点：`GET/PUT /api/v1/settings/proxy-private-allowlist` 体 `{allowlist: []string}`，默认 `[]`（空 = 维持全阻断，行为不变）
- 每条为单个 IP（`192.168.1.100`）或 CIDR 网段（`192.168.1.0/24`），PUT 时 `services.ParseAllowlist` 校验，非法条目返回 400
- 判定：`services.IsHostnameAllowedWithAllowlist(hostname, allowlist)`——外网恒放行，私网 IP 仅当命中白名单某条网段才放行；`localhost`/`.local`/空主机名仍字符串级封禁（白名单只按 IP/CIDR 匹配）
- **影响通用 `/proxy` 与 `/proxy/transcode`**；HLS 反代（`hls.go`）仍走不带白名单的 `IsHostnameAllowed(hostname)`，不放宽其私网限制。
- 实现：`internal/services/whitelist.go`（`ParseAllowlist` / `IsHostnameAllowedWithAllowlist`）+ `internal/handlers/proxy.go`（`ProxyHandler` 持有 `*ConfigService`，config key `proxy_private_allowlist`）

<a id="url-transcode"></a>

## 音频转码代理（/proxy/transcode）

- 业务端点：`GET /api/v1/proxy/transcode?url=&format=mp3[&bitrate=&duration=&user_agent=&referer=]`（`@Security BearerAuth`，需 token；token 作为首个查询参数，规避音箱固件把 `&` 替换为空格的坑）
- 服务端拉取远程音频 URL，经 ffmpeg 实时转码为 mp3（CBR 320k）流式返回。供客户端播放原始格式不兼容的远程音频（如 webm/opus，songloft-org/songloft#394），客户端使用本端点获得兼容的 MP3 流
- 不落盘、不入库、不缓存（与 no-import 语义一致；youtube 直链每次重签，缓存零收益，重播收益走入库路径）
- SSRF 防护：复用 `/settings/proxy-private-allowlist`，与通用 `/proxy` 同一 `IsHostnameAllowedWithAllowlist` 校验
- 输出 mp3 CBR + `-write_xing 0` + `-map 0:a:0 -vn`：pipe 不可 seek，音箱靠字节估算时长，CBR 避免提前切歌（同 seek 流取舍）
- 并发与 seek/均衡流共享 `seekStreamSem`（cap=4），满则 503 不排队
- ffmpeg 直拉远程 URL（`-i <url>`），经 `-http_proxy` 复用全局 http-proxy（仅 http/https）
- 实现：`internal/services/url_transcode.go`（`StreamTranscodedURL`）+ `internal/handlers/proxy.go`（`Transcode`，`ProxyHandler` 持有 `*CacheService`）

<a id="hls-transcode"></a>

## 远程视频 HLS 转码会话（#541）

- 入口：`POST /api/v1/proxy/transcode/hls/sessions`，JSON `{url, audio_bitrate?, user_agent?, referer?}`。`url` 为 HTTP/HTTPS TS、MP4 或普通 HLS 源；不接受 URL 用户凭证。码率为 64/96/128/192/256 kbps，默认 128。需配置的 ffmpeg 及其同目录或 PATH 中的 ffprobe。
- 工具构建必须同时启用 **HLS 输入 demuxer** 和 **crypto 输入协议**；有 HLS 输出 muxer 不代表能读取 m3u8。`ffprobe -hide_banner -demuxers` 应有 `hls`，`ffprobe -hide_banner -protocols` 的 Input 应有 `crypto`（AES-128 HLS）。Docker 基础工具来自 `tools/ffmpeg-builder`；其产物测试覆盖 TS、普通及加密 HLS。Songloft 二进制在线升级不会替换容器内工具，工具修复后需重新构建 Songloft 镜像并拉取、重建容器。
- 首版固定复制第一条**逐行 H.264** 视频轨、第一条音频转 **AAC-LC / 双声道 / 48kHz**，输出 MPEG-TS HLS。HEVC、隔行视频、缺少音视频轨返回 422；不自动重新编码视频。源的 H.264 profile/level、分辨率及关键帧仍需适合实际播放器，不承诺所有 IPTV 源均可在 Safari 播放。
- 创建等待首个完整分片就绪，最长 30 秒；成功返回 201 `{id, playlist_url, idle_timeout_seconds:120}`，并设置 `Location`。`playlist_url` 包含部署 `BASE_PATH`、不包含 token。Safari 的 `<video>` 使用该地址时追加 `?access_token=<URL编码的访问token>`；服务端将其透传给相对分片链接。访问 token 到期仍会返回 401；调用方需刷新 token 并更新播放地址，同一 ClientID 可继续使用原会话。若播放器使用 Authorization 头，需自行给每条子请求发送该头。
- `GET/HEAD /api/v1/proxy/transcode/hls/sessions/{id}/playlist.m3u8` 只读取现有会话；`GET/HEAD .../{id}/segments/{name}` 支持 Range；`DELETE .../{id}` 等待进程退出后返回 204。所有请求都需要 JWT，且会话绑定创建者 `ClientID`，其他客户端和过期会话返回 404。随机 ID 本身不是凭证。
- 目标分片 4 秒，实际边界依赖输入关键帧；播放列表保留 6 片，额外保留 3 片供重试，使用原子更新。不提供节目回看；暂停超过闲置时间后需创建新会话。播放列表 `no-store`，分片 `private, no-store`。
- 会话不入库，分片在独立系统临时目录滚动生成，不参与音乐缓存配置、清理或统计。进程按实时速度读取；每 200ms 检查临时空间，超过 128 MiB 停止，检查间隔内可能短暂超限。运行中 30 秒没有新播放列表也会停止；失败后的读取返回 502。上游自然结束则保留可播放的有限内容直至闲置回收。
- 从首播就绪开始，120 秒无成功播放列表/分片请求自动回收；删除和应用关闭也会取消并等待进程。已经打开的分片有文件租约，传输关闭后清理。正常结束/失败会释放运行槽位，最多保留 16 个会话。运行中的探测/转码与 seek/均衡/MP3 URL 转码共用 4 个槽位，满则 503、不排队。进程被强制终止时临时残留由系统临时目录清理策略处理。
- 拉流经独立回环 HTTP 网关：所有上游 HLS URI（子列表、分片、密钥、init）与重定向均校验 HTTP/HTTPS 和私网代理白名单；直连在实际拨号时校验并固定解析 IP。FFmpeg 仅允许受限输入协议/容器，不能直接读取本地文件、UDP 或未改写的 HTTPS 引用。源中的变量替换和内容导向等扩展首版返回 422。
- 复用全局 HTTP/HTTPS 代理（会话创建时快照），SOCKS5 返回 503；代理是管理员配置的可信出站设施，代理模式下目标仍做本地地址校验，远端 DNS 解析依赖代理。已有电台 HLS 反代开关及其同源限制不受影响，旧 MP3 `/proxy/transcode` 保持原有契约。
- 实现：`internal/services/hls_transcode.go`、`hls_pull_gateway.go`、`internal/handlers/hls_transcode.go`。API 契约以生成的 Swagger 为准。

<a id="music-cache"></a>

## 音乐缓存（cache_service）

- 播放远程歌曲时流式代理上游音频到客户端（不阻塞），同时后台异步写入缓存；后续播放缓存命中后直接从本地返回
- 流式代理 `ServeRemoteResourceWithCache`：200 OK 时 TeeReader 同时代理+写临时文件，206 Partial 时正常代理并触发异步全量下载
- 缓存路径持久化在 `songs.cache_path` 字段（DB 级别），查找时优先 `cache_path`，fallback 到旧格式哈希分桶目录
- 缓存目录默认 `{data_dir}/music_cache/`，可通过 `PUT /api/v1/cache-manage/config` 的 `cache_dir` 字段自定义为绝对路径
- 启动时从 `music_cache_config` 配置读取自定义目录；运行时切换目录会自动重建 LRU 索引，不迁移旧文件
- LRU 淘汰：超出 `max_size`（默认 1GB）时按最后访问时间淘汰，`max_size=0` 表示不限制
- **缓存落盘转码**（`transcode_format` / `transcode_quality`，`PUT /api/v1/cache-manage/config`，songloft-org/songloft#300）：默认 `""` 按上游原码落盘（YouTube .mkv/.webm、B站 .mov）；设为 `mp3/m4a/ogg/flac/wav` 后，缓存网络歌曲落盘时统一转码为该格式（适配不支持 MKV 的音频客户端）。由 `EnsureCachedFormat` 在两个播放侧产出点执行——`FinalizeCache`（流式播放 + 206 异步全量）与 `prepareSongPlayback` 的 prefetch 预热（`Get` 之后）；复用 `runFFmpeg`，转码后删原码、`cache_path` 指向新格式。缺 ffmpeg 或转码失败时**优雅降级保留原格式**。**不影响** `songs.download` 的显式格式处理（下载路径复用 `Get` 但自带 `opts.Format`，故转码逻辑只挂在播放侧，不动 `moveToCache`/`Get`）。**取舍**：`ffmpeg -vn` 会丢视频轨，故对 `is_video` 的远程歌开启后 `media=video` 投屏只得纯音频（属预期，满足 YouTube MKV→mp3 主诉求）；`EnsureCachedFormat` 内含 `cacheTranscodeTimeout`（15min）兜底，防坏源卡死永久占用 `transcodeSem`
- `POST /api/v1/cache-manage/validate-dir` 可预先验证目录（自动创建 + 可写性检查 + 返回磁盘空间）
- inflight 去重：同 `song.ID` 的并发请求只下载一次；首请求被 `ctx.Canceled` 时后续等待者自动重试

<a id="normalization"></a>

## 音量均衡（`/songs/{id}/play?normalize=1`）

EBU R128 响度均衡（songloft-org/songloft#315），消除不同音源之间的响度落差。客户端通过播放 URL 的 `normalize=1` 参数请求均衡。

- 落盘转码与实时流共用 `CacheService.normalizeLoudnessFilter()`，从 `volume_normalize_loudness` 读取目标响度（默认 -16 LUFS，允许 -40～-5）。仍是 `loudnorm=I=<目标>:LRA=11:TP=-1.5` 的**单遍**动态模式；`loudnormFilter` 常量仅保留作默认值测试基准。**不要**内联写死，两条路径必须给出同一响度。
- `transcodedFileName` 与 inflight key 使用 `normalizeLoudnessTag()`，标记含目标响度（如 `norm-16` / `norm-14`）；不同响度及不均衡的产物**互不通用**。
- **转码产物未就绪时边转边发**（`tryLiveTranscodeStream` → `StreamSeekedMP3`，`internal/handlers/music.go`）：
  `GetOrTranscode` 是同步的，首字节 = 整首转码的墙钟时间；整首 loudnorm 要 20+ 秒，
  普通 `?format=` / `?quality=` 转码在弱 CPU（N5095）上无损转 mp3 也要等 5 秒以上才出声
  （songloft-org/songloft#442）。改为 pipe 直出后实测均衡场景首字节 10.03s → 0.088s。
  **`format` 与 `quality` 两条路都覆盖**（`SeekStreamOptions.Bitrate` 让 pipe 能如实表达 `?quality=`，
  不再固定 320k），仅当目标格式是 mp3、非 `media=video` / 抽轨 / CUE / HEAD 时生效，其余场景保持原阻塞路径
- **`ForceTranscode` 必须显式传，不能靠「源扩展名不是 mp3」推断**（`SeekStreamOptions`）：纯 `?format=mp3`
  既不 seek 也不均衡不变速，`StreamSeekedMP3` 的守卫会判它「没活干」而拒绝；而伪 mp3（扩展名 `.mp3`
  内容是 WebM/Opus，songloft-org/songloft#300）恰恰是扩展名为 mp3 却必须转码的一类，靠扩展名推断
  会把它放进 `-c:a copy` 快路径 → mp3 muxer 拒收 Opus → 零输出 → 降级，白起一个 ffmpeg 且延迟一点没改善
- **泛化后的守卫必须自洽**：`tryLiveTranscodeStream` 自己算 `normalize || bitrate > 0 ||
  NeedsTranscodeForServe(...)`，不依赖两个调用点（`serveLocal` / `serveCachedFile`）的 `if` 条件
  ——判错的代价是每次播放都白烧一个 libmp3lame 进程
- **pipe 流必须支持 Range，否则拖动会静默卡死**（`cbrRangePlan`，songloft-org/songloft#442）：
  chunked 无 `Content-Length` 的响应会被浏览器判定为不可 seek。Docker 无头 Chrome 实测拖动进度条后
  **一个新请求都不发**，`readyState` 从 4 掉到 1、buffered 零增长、`error` 为 null——**播放静默卡住，
  只能重新点播**。而进度条总时长取自 DB 的 `song.duration`，UI 上完全看不出这条流不能拖。
  只补 `Accept-Ranges: bytes` 没用（实测同样只发一个请求就卡死）：浏览器需要**总字节数**才能把
  「第 N 秒」映射成字节偏移。**能算得出来的原因**：pipe 一律 CBR（`-b:a`），字节与时间成线性，
  配合已知的 `song.duration` 即可给出总长度与任意偏移对应的秒数；而「从第 N 秒起流」正是
  `StreamSeekedMP3` 的 `-ss` input seek。于是 Range 被**真正**满足，而不是「忽略 Range 从头重发」
  （那会让进度条与实际声音错位）
- **Range 前提以 `planCBRRange` 为准**：歌曲时长已知且不超过 24 小时、不变速、seek 后剩余时长 > 0。
  `?seek=N` **可以与 Range 组合**：总长度按剩余时长估算，Range 是这条新流的字节偏移，
  ffmpeg 起点为 `N + Range 偏移 / 每秒字节数`（songloft-org/songloft#488）。不要重新加回「有 seek 就禁用
  Range」的旧限制；音箱需要 Content-Length 与 206 做分段缓冲。copy VBR MP3 的估算仍有偏差，
  `exactLengthWriter` 仅保证应答字节数，不代表音频时间或帧边界完全精确。不满足前提时退回 chunked 无长度
- **承诺了 `Content-Length` 就必须写够写准**（`exactLengthWriter`）：ffmpeg 的实际输出量不会精确等于
  「码率 × 时长」——LAME 的 CBR 靠帧 padding 贴合码率、末尾可能是不完整帧、`-ss` 只能对齐到帧边界，
  累计误差在几帧内。写少了客户端当作连接提前断开，写多了违反 HTTP/1.1 长度契约，所以超出截断、
  不足补零。**超出时必须返回 error 中断 io.Copy**（`errExactLengthReached`）而非静默丢弃：
  闭区间 Range（`bytes=N-M`）只要一小段而 ffmpeg 会转到源结尾，静默丢弃等于让一个 `bytes=0-100`
  请求白转整首
- **206 的 `WriteHeader` 必须推迟到首次真的写字节时**（`deferredStatusWriter`）：一旦 WriteHeader
  响应就提交了、再也无法降级，而 `StreamSeekedMP3` 的契约是「`Peek(1)` 确认 ffmpeg 真有输出后才写」
  ——「首次 Write」正是那个安全时刻。同理**降级时要 Del 掉 `Content-Length`/`Content-Range`/
  `Accept-Ranges`**，残留的长度会和 `http.ServeFile` 自己算的冲突
- **包装 writer 必须转发 `Flush()`**：services 侧是 `io.Copy(&flushingWriter{w: w}, ...)`，而
  `flushingWriter` 靠**类型断言** `w.(http.Flusher)` 决定要不要 flush。`exactLengthWriter` /
  `deferredStatusWriter` 少了这个方法就会把断言挡掉，字节攒在缓冲里不下发——**首字节延迟原地回归**
- **实时流刻意不顺手起后台转码补缓存**：同一首歌并跑两个 ffmpeg 会在弱 NAS 上把 CPU 翻倍，
  而用户正等着这一秒出声。缓存产物交给 `?prefetch=1`（带同样的 format/quality/normalize）生成
- **接入 #442 后没有提高 `seekStreamMaxConcurrent`（仍是 4）**：普通转码播放的频率比 seek/均衡高一个
  数量级，槽位更容易占满，但 #442 的报告环境正是弱 CPU——放大并发对它是伤害，而占满时降级回
  阻塞路径本身无损（只是回到修复前的行为）
- **预热必须带 normalize**：`prepareSongPlayback` 的 `normalize` 参数不能丢，且短路判断里要有 `!normalize`
  ——mp3 源 + `format=mp3` 时 `NeedsTranscodeForServe` 为 false，少了这一项预热会直接 return、一行没干
  （日志中也不会出现 `prefetch ready`）

<a id="seek-stream"></a>

## 服务端 seek 流（`/songs/{id}/play?seek=<秒>`）

给 URL 推流客户端表达续播位置用。客户端通过 `seek=N` 请求一条**以第 N 秒为开头**的新流；
Range 表示这条流的字节偏移，两者语义独立。实现在 `internal/services/seek_stream.go`，
骨架与 `radio_transcode.go` 同构（`Peek(1)` 确认有输出才提交响应，否则返回哨兵错误让
handler 无损降级为从头 `ServeFile`）。

- **一律输出 MP3**：源是 mp3 走 `-c:a copy`（input seek，实测整首 0.14s、近零 CPU），否则
  `libmp3lame`。**不要**扩成多格式矩阵——浏览器有 Range 可用，这个参数只服务推流客户端
- **重编码必须 CBR（`-b:a 320k`）**：pipe 不可 seek → 无法回写 Xing 帧（故显式 `-write_xing 0`），
  客户端只能按「首帧码率 × 字节数」估时长。**不要**改成项目别处的 `-q:a 0`：实测 105.4s 的流
  会被估成 97.6s，音箱据此可能提前判定播完
- **`-map 0:a:0` 不能删**：mp3 muxer 只接单条音频流，双音轨 `.mka`（#298）不选轨会直接失败，
  表现为静默降级回「从头播」，只有一行 warn，极难归因
- **只对本地歌曲与已缓存的网络歌曲生效**：未缓存时拿到本地文件要先同步下载整首，会让「续播」
  这一下按键卡住一整首下载时长。电台（直播无位置）、`media=video`（`-vn` 会丢画面）、HEAD 均忽略
- **seek 必须夹到距结尾 3 秒之外**（`parseSeekSeconds`；客户端也应遵守这一边界）：
  越过文件尾 → ffmpeg 零输出 → 触发降级 → **整首从头重播**，比忽略 seek 更糟
- **不占 `transcodeSem`**（进程存活整首剩余时长，会饿死其他转码），改用本文件内 `cap=4` 的独立
  信号量，满了直接降级不排队；`exec.CommandContext` 另带「剩余时长 + 5min」硬超时回收孤儿进程。
  该信号量与上面的实时均衡流、变速流、普通转码流（#442）**共用**（`StreamSeekedMP3` 是同一个函数），
  所以 4 是这几类流的总额。
  实际占用时长取决于客户端读得多快而非歌曲时长——音箱是贪婪缓冲，实测 4 分钟的歌 10 秒就流完退出。
  槽位由 `io.Copy` 持有，**ctx 取消不会打断它**（io.Copy 只看读端 EOF / 写端出错），
  所以注册 playActivity 只能掐掉 ffmpeg 省 CPU，不能提前归还槽位
- **剩余时长未知（`songs.duration == 0`）时硬超时不能退化成 5 分钟**（见 `seekStreamUnknownDurationTimeout`）：
  对整首流那不是宽限而是硬上限，客户端边播边读会在第 5 分钟被杀掉，而响应正常闭合、客户端以为播完了。
  `duration == 0` 是常态（远程歌曲元数据未刷新、本地文件 tag 与 ffprobe 都拿不到时长）
- 已知时长、不变速且满足 `planCBRRange` 前提时返回 `Content-Length` / `Accept-Ranges: bytes`，
  分段请求可返回 206；其他情况才是 chunked 无长度。两者均使用 `Cache-Control: no-store`

<a id="song-download"></a>

## 歌曲持久化（song_downloader — 插件基础设施）

- **定位**：插件基础设施能力，不是主程序面向用户的功能。主程序提供 `songs.download` Bridge API，允许插件将用户自有网络存储（NAS/WebDAV/Subsonic 等）中的远程歌曲持久化到服务端本地 `music_path`，转为 `local` 类型。**此能力仅用于用户合法拥有的音乐资源，不得用于下载第三方商业音乐平台的受版权保护内容**
- 核心服务 `SongDownloader.Download`：获取音频（缓存命中直接 copy，否则同步下载）→ 可选转码 → 路径模板渲染 → 可选元数据嵌入（所有支持的格式）→ 更新 DB（type=local）
- **下载转码**（`SongDownloadOptions.Format` / `Quality`）：插件可传 `format`（mp3/m4a/ogg/flac/wav）+ 可选 `quality`（128/192/320），复用播放路径的 `CacheService.GetOrTranscode` 在下载时转成标准音频容器。典型用途：B 站等源产出 `.mov` 视频容器无法刮削歌词，转 mp3 后落地。`format` 为空=不转码保留源格式；转码依赖 ffmpeg，缺失/失败时优雅降级（仅 warn，保留源格式，不阻塞下载）
- **URL 歌词自动拉取**：`embed_metadata=true` 且 `lyric_source=url` 时，通过 `LyricFetcher` 拉取歌词 → 主歌词写入文件标签 → 完整 payload（含翻译/罗马音）缓存到 DB → `lyric_source` 更新为 `embedded`。拉取失败仅 warn 不阻塞持久化
- 通过 Bridge API `songs.download` 暴露给 JS 插件，权限映射到 `PermSongsWrite`
- 官方插件 `songloft-plugin-downloader`（独立仓库 `songloft-org/songloft-plugin-downloader`）基于此 API，提供将用户自有网络存储中的远程歌曲下载到本地的功能

<a id="file-move"></a>

## 文件搬移：跨设备 rename 陷阱

- `os.Rename` 在 src 和 dst 不在同一文件系统（挂载点）时会返回 `syscall.EXDEV`（cross-device link）错误
- 典型场景：`os.CreateTemp("")` 创建在系统 `/tmp`（tmpfs），目标 cache/music 目录挂载在独立磁盘或 Docker volume
- **统一使用** `fileutil.MoveFile(src, dst)`（`internal/fileutil/file_move.go`）替代跨设备的裸 `os.Rename`：先尝试 rename，EXDEV 时自动回退 copy + remove；Windows 还识别 `ERROR_NOT_SAME_DEVICE`。
- `pkg/tag` 的原子写不受影响：它用 `os.CreateTemp(dir, ...)` 在源文件**同目录**创建临时文件，rename 一定同设备
- 新增下载/缓存逻辑如果需要"先写临时文件再挪到目标位置"，**必须**用 `fileutil.MoveFile`，**不要**裸 `os.Rename`

<a id="config-endpoints"></a>

## 配置接口的三种风格（2026-06 决策）

项目里同时存在三类配置接口。**用户可见的功能开关一律走业务端点**，通用 KV 只作 admin 入口：

- **`/api/v1/settings/<kebab-case-name>` — 孤立配置的默认入口**（如 `hls-proxy`、`music-path`、`http-proxy`、`library-browse`、`proxy-private-allowlist`）。数据是**强类型 JSON**（如 `{enabled: bool}` 或聚合对象），不是 `{value: string}`；默认值由 handler 承担（缺失时 GET 返回业务默认，PUT 直接写入，前端无需先 POST 创建）；副作用在 PUT 内直接触发（如 `music_path` PUT 后异步重建 Scanner）；handler 同时持有 `*services.ConfigService`。命名套路：`Is<Name>Enabled/Set<Name>Enabled` 业务方法 + `Get<Name>Setting/Update<Name>Setting` HTTP handler。
- **`/api/v1/<module>/config` — 模块内配置**：与该模块动作端点强相关的配置保留在模块前缀下，不拆到 `/settings/`。已有 `/cache-manage/config`（与 `stats/clean` 共用 `CacheService`）。判定：孤立或跨模块共享 → `/settings/<name>`；模块内强相关 → `/<module>/config` 或 `/<module>/<sub-name>`。
- **`/api/v1/configs/{key}` — 通用 KV（仅 admin 编辑器）**：仅供 `config_manager.dart` 这类通用配置编辑器调试。新业务功能**不要直调**：通用 PUT 在 key 不存在时返回 404，且没有强类型、副作用和默认值。业务化封装后通用接口仍可改同一 key，但副作用必须同时挂在 `configHandler.SetOnConfigChanged` 回调（参考 `routers.go` 的 `musicPathChanged`）。

客户端约定：`SettingsApi`（`clients/player/lib/features/settings/data/settings_api.dart`）封装所有 `/settings/*` 调用，业务 Provider 一律走它；`ConfigApi` 只在 `config_manager.dart` 和「列出所有配置」这类 admin UI 中使用。

历史背景：该规范在 2026-06 引入——`hls_proxy_enabled` 默认未预置导致 PUT `/configs/{key}` 返回 404，暴露出 `/configs`、`/settings/*`、`/cache-manage/config` 三种风格并存。最终确定业务端点是用户可见入口的唯一来源，通用 KV 退化为 admin 后门。
