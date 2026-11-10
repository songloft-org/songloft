# 统一自定义标签与歌单唯一名称设计

状态：已实施并完成本地验证及审核，用户已授权提交、推送和回复 Issue。2026-11-11。关联 [songloft-org/songloft#562](https://github.com/songloft-org/songloft/issues/562) 与 [songloft-org/songloft#558](https://github.com/songloft-org/songloft/issues/558)。[English](en/issue-562-design.md)。

## 1. 设计结论

- 歌曲和歌单共用现有自定义标签定义；歌曲继续使用 `song_tag_links`，歌单通过现有 `playlists.labels` 引用标签，不增加 `playlist_tag_links`。
- 保留歌单名称全局唯一、现有数据库唯一索引和仓储查重，不改成按标签唯一，也不增加显示名字段。
- 来源通过自定义标签分类、筛选和展示。自动同步优先使用远端原名；实际重名时采用“原名-来源”，该名称也被占用时再追加 ` (2)`、` (3)`，不再给所有名称加来源前缀。
- 自动分配的名称随已有歌单 ID 保存和复用；名称与标签均不承担同步归属校验。
- 用户直接创建、改名或明确配置目标名称时，重名仍返回冲突，不静默改动用户指定名称。

## 2. 改动前实现与约束

核对基线：主仓库 `22cd87edf308b53ad6301e0b8a418bd8a6e784a2`；Lynx `2a18bb57819886a2c9b2c0476bdf51a127f09ae1`；Flutter `59bc9c7def13e73ced81df045cf77e3ce32f0426`；Navidrome `04d87b0d79c880d1e648d056375a680e97d78802`；MIoT `1379528daab3841e7e3ca0edb05d14046ff14a17`。

| 现状 | 代码依据 | 设计影响 |
| --- | --- | --- |
| 自定义标签是 `song_tags`，歌曲通过 `song_tag_links` 关联 | `internal/models/song_tag.go`、迁移 `0036_song_tags.sql` | 复用现有定义和标签 ID |
| 歌单有 JSON 字符串数组 `labels`，仓储支持精确包含筛选 | `internal/models/models.go`、`internal/database/playlist_repository.go` | 增加标签引用和接口接线，不增加关联表 |
| 歌单名称有仓储查重和数据库唯一索引 | `PlaylistRepository.Create/Update`、迁移 `0001_init.sql` | 两层约束均保留；并发冲突以实际写入结果为准 |
| 备份按名称复用歌单；自动扫描用名称匹配分组 | `BackupService.findOrCreatePlaylist`、`PlaylistRepository.AutoCreate` | 本次不改变已有名称身份假设和合并规则 |
| Navidrome 使用连接、远端 ID、歌单 ID 映射和 description 标记 | Navidrome `src/sync.ts` | 按已有 ID 改名；不能发现同名就接管 |
| WebDAV 已有名称占用检查和冲突重试 | WebDAV `src/sync-task.ts` | 可参考流程；不必为本次统一改写其他插件 |

代码参考：[歌单仓储](https://github.com/songloft-org/songloft/blob/22cd87edf308b53ad6301e0b8a418bd8a6e784a2/internal/database/playlist_repository.go)、[Navidrome 同步](https://github.com/songloft-org/songloft-plugin-navidrome/blob/04d87b0d79c880d1e648d056375a680e97d78802/src/sync.ts)。

## 3. 统一标签

### 3.1 存储与写入

沿用 `song_tags` 表及其现有名称唯一规则。UI 统一称为“自定义标签”。例如标签 12 是“Navidrome”，歌单保存：

```json
{
  "id": 42,
  "name": "收藏-Navidrome",
  "labels": ["hidden", "tag:12"]
}
```

`tag:<十进制正整数 ID>` 是自定义标签引用；`hidden`、`built_in`、`auto_created` 仍由各自业务管理。公共标签接口接收 `tag_ids`，仅修改 `tag:*` 部分并保留其他标记，不能借此改变系统身份。

写入时校验全部标签存在、去重，任一无效则整体失败。旧客户端未传标签字段时保留原绑定。标签改名、改色后，歌曲和歌单都从同一定义显示。歌单绑定不自动给成员歌曲打标签，歌曲绑定也不反向修改所在歌单。

### 3.2 删除与事务

删除标签时，在同一 `RunInTx + UnitOfWork` 中清理歌单引用并删除定义；歌曲关联由外键级联清理。实现时把标签仓储纳入 UnitOfWork。绑定时的存在性校验和写入也须与删除串行化，避免留下悬空引用。

当前标签 ID 是普通 `INTEGER PRIMARY KEY`，删除后可能复用。因此不能留下旧引用；插件每轮按配置的标签名解析定义，不能永久信任缓存 ID。删除后重新创建同名标签不恢复旧手工绑定。同步仍启用来源标签时，下一次同步可按配置重新建立对应绑定。

### 3.3 分类与接口

标签详情分别提供“歌单 / 歌曲”入口，各自分页搜索；全局歌单列表支持标签筛选。歌曲数与歌单数分别聚合，避免重复计数；隐藏歌单的计数与浏览使用相同过滤条件，空歌单绑定后也能出现。

以下接口已实现。保留现有 `/song-tags` 路径与字段，减少旧消费者迁移。

| 接口 / 桥接 | 行为 |
| --- | --- |
| `GET /playlists/{id}/tags` | 返回歌单的自定义标签定义 |
| `PUT /playlists/{id}/tags`，`{tag_ids: [...]}` | 替换自定义标签部分，保留系统标记 |
| `GET /playlists?tag_id=12` | 在分页前按标签筛选，列表和总数共用条件 |
| `GET /song-tags` / `GET /song-tags/{id}` | 增加 `playlist_count`，保留 `song_count` 原语义 |
| `songloft.tags.getPlaylistTags/bindPlaylists/unbindPlaylists` | 与已有歌曲方法对称，bind/unbind 仅增删指定关系 |

复用现有 `tags` 权限；宿主桥接、SDK 类型同步更新。新 handler 补齐 swag 注释并生成 API 文档。

## 4. 保留名称唯一，冲突时追加来源

### 4.1 命名规则

默认同步依次尝试“原名”“原名-来源”“原名-来源 (2)”“原名-来源 (3)”。例如来源为 Navidrome 时，冲突名称为“收藏-Navidrome”。使用半角连字符，不额外添加空格；编号仅作为来源后缀仍然冲突时的兜底。所有歌单共享同一个名称空间，不因来源、连接、标签或歌单类型不同而允许重名。比较方式沿用现有数据库规则，不顺带改变大小写语义。

| 情况 | 歌单名称 | 自定义标签 |
| --- | --- | --- |
| 系统内置收藏 | 收藏 | 无需来源标签，保留内置标记 |
| Navidrome 的收藏与内置名称冲突 | 收藏-Navidrome | Navidrome、家中 NAS |
| 第二个 Navidrome 连接也有收藏 | 收藏-Navidrome (2) | Navidrome、办公室 NAS |
| 没有重名的远端歌单 | 钢琴曲 | Navidrome |
| 已有“钢琴曲”，同步另一份同名歌单 | 钢琴曲-Navidrome | Navidrome |

原名本身包含连字符、来源文字、括号或编号时按完整名称处理，不猜测或剥离它。后缀中的来源使用插件约定的稳定名称，与可改名的显示标签分开；用户重命名“Navidrome”标签不会改动歌单名称。多个同源连接的区别通过连接标签展示，名称冲突只用数字兜底，避免名称持续加长。名称分配只在创建、首次迁移或远端原名发生有效变化时执行。显式目标名称优先于远端原名；目标配置不变时，远端改名不触发本地改名。

### 4.2 稳定同步与冲突处理

1. 先使用现有连接 + 远端 ID 映射寻找歌单，并校验归属标记；映射缺失时仅按已有归属恢复规则查找。绝不按名称合并或覆盖其他歌单。
2. 初次分配时读取完整的已用名称集合，排除当前歌单自身。候选名称仍须通过真实创建 / 更新操作确认，预检不构成名称预留。
3. 只有明确的名称冲突才尝试下一个候选并有限重试；网络、权限、归属等其他错误直接返回。当前插件桥接的字符串错误需要核对识别方式，不能把任意失败当作重名。
4. 插件私有状态在现有 ID 映射之外保存命名模式、请求原名及已分配名称，不新增服务端字段；异常恢复以已有歌单 ID、归属和实际名称为依据。
5. 已分配“收藏-Navidrome (2)”后，即使“收藏-Navidrome”被删除，后续同步仍保留原名称；无来源后缀的名称空出时也不自动缩短。增删标签、连接显示名变更同样不触发改名。用户在宿主手动改名后保留该名称，直到用户修改目标名称配置或默认模式下远端原名变化。
6. 远端原名变化时，在同一歌单 ID 上重新分配并更新；不删除重建。明确配置的目标名称发生冲突时返回清晰错误，由用户修改配置，不擅自加编号。
7. 命名成功后再替换歌单成员，避免改名失败后成员已经被替换。名称、标签和成员尚无跨插件存储的统一事务，不宣称整轮同步原子；失败须保留可重试的原映射并反馈实际完成状态。

同步预览使用同一分配规则，展示预计名称及冲突原因；不写入数据。实际执行时重新确认，处理预览之后出现的并发占用。执行结果显示最终名称。

### 4.3 旧歌单迁移

旧 Navidrome 歌单目前采用“连接名 · 远端歌单名”。升级时只对映射和归属都确认的受管歌单执行一次命名迁移，保留 ID、歌曲、排序、置顶与历史关联。

没有明确目标名且本地名称仍符合插件旧默认规则时，尝试改为远端原名；冲突则追加来源，必要时再加编号，并记住结果。已有明确目标名保持原配置；本地名称偏离旧默认值或无法确定归属时保留现状，不擅自改名。不要通过全局字符串替换删除其他歌单名称中的“Navidrome”。

首次迁移的分配顺序固定，例如按连接 ID、远端 ID 排序，避免每次预览得到不同结果。后续同步以已保存分配为准。

## 5. 显示与兼容

主标题显示唯一名称；来源标签放第二行或分类入口，长标题补齐滚动显示。标签多时优先展示来源与连接，其余进入详情。description 中可识别的内部同步标记只在 UI 隐藏，继续原样保存，避免破坏插件归属校验。

本方案不需要名称相关 schema 迁移，不需要修改按名称查找的返回类型，也不要求为小爱新增同名候选流程。备份按名称合并、扫描命名和内置收藏规则延续现状；它们各自已有的边界不在本次扩大。

共享标签的备份仍需独立处理：扩展格式保存标签定义及歌曲 / 歌单绑定，导入按标签定义映射成本地 ID，不能直接复制跨实例的 `tag:12`。旧备份保持原格式兼容，不凭空恢复缺失的标签定义；无法解析的引用不写入。插件归属和原始 ID 的跨实例可移植性另行遵循备份已有契约，不能因显示标签相同就推断归属。

## 6. 歌单封面与布局：纳入 #562 的交付范围

### 6.1 曲库歌单封面

当前有两处断点：歌单自身没有封面字段时，`Playlist.CoverURLPath` 返回空字符串，卡片根本不会请求回退端点；`GetPlaylistCover` 的成员回退只检查前 20 首歌曲的本地封面，不能使用 Navidrome 歌曲已有的远程封面。插件相对地址还需要复用内部 URL 解析与鉴权。

- 没有独立封面、但有成员可供回退的歌单也返回统一封面端点。列表组装避免逐歌单读取成员，具体选图在端点按需执行。
- 优先级为歌单自己的有效本地封面、歌单远程封面、成员歌曲的有效本地 / 远程封面。尊重用户设置的独立封面，远程与插件地址复用已有资源处理能力。
- 查询固定数量的封面候选，而非仅查看前 20 首任意歌曲；远程失败回退受总请求预算约束，不扫描整张歌单或无限请求网络。
- 服务端部署与 Bundle 均能解析插件封面地址。验证同步、成员变更与封面替换之后的缓存刷新，空歌单或全部失败时仍显示占位图。
- 曲库网格、歌单列表、首页歌单卡片和详情使用同一封面契约，避免某个入口恢复、其他入口仍缺图。

### 6.2 标题、来源与详情头部

- 歌单列表、网格卡片和详情标题复用已有 `ScrollingText` 能力，遵循长标题滚动开关和系统减少动画设置。静态模式保留省略，不新增另一套滚动实现。
- 来源标签独立显示或作为分类入口；内部同步标记仅从显示文本中隐藏，不删除数据库 description。长用户描述仍可打开查看。
- 手机详情封面从当前 160px 缩到约 96px，减少上方和上下留白。文字列随内容自适应，取消与封面绑定的 160px 固定高度；大字号时不能裁掉标题、标签或计数。
- 歌曲数量从当前较小的 footnote 提升到约 15–16px 对应字号，放在稳定可见的位置。宽屏沿用较大的封面和现有操作布局。

### 6.3 曲库与歌单歌曲行

现有手机收藏、更多按钮已相邻，都是 44px 点击区域，且按钮自身 margin 为 0。新增优化应核对实际容器留白，重点调整行左右 padding、序号列 28px 宽度及其 4px 间隔、标题与操作区之间的空隙。按钮图标居中且点击区域不互相覆盖，保留 44px 触控区域。

曲库与歌单复用 `SongListRow`，通过手机样式统一调整；多选、排序、三位以上序号、正在播放状态及宽屏列对齐必须保持正常。手机移除歌曲继续放在更多菜单并保留确认，长歌名滚动和时长位于副标题的已有行为保持有效。

### 6.4 完成标准

除单元与接口回归外，用至少 360 / 390px 手机宽度验证长中英文歌单名、歌曲名、多来源、多个同源连接、浅色 / 暗色与大字号。记录标题实际可用宽度、头部占用高度、首屏可见歌曲数量，并实际操作收藏、更多菜单、移除确认、标签筛选和分页。

封面须验证 Navidrome 插件相对 URL、普通远程图、本地图、无图和失败回退，以及服务器 / Bundle 两种模式。荣耀 30 Pro / 鸿蒙 4 原机的滚动和图片显示仍需设备验收；浏览器通过不能代替原机结果。

## 7. 代码改动范围

下列为实现范围，路径以当前代码为准。完整方案涉及主仓库、Lynx、Flutter、Navidrome 插件和插件工具链五个仓库。

| 模块 | 主要文件 | 预计修改 |
| --- | --- | --- |
| 标签模型与仓储 | `internal/models/song_tag.go`、`internal/models/models.go`、`internal/database/song_tag_repository.go`、`playlist_repository.go`、`queries/song_tags.sql`、`queries/playlists.sql` | 歌单标签引用、筛选、计数、批量清理；生成相应 sqlc 代码 |
| 事务、服务与 API | `internal/database/unit_of_work.go`、`sqlite.go`、`internal/services/song_tag_service.go`、`playlist_service.go`、`internal/handlers/song_tag.go`、`playlist.go`、`internal/app/app.go`、`routers.go` | 同事务删除 / 绑定，注入依赖，新增歌单标签读写端点及列表条件 |
| 歌单封面 | `internal/models/models.go`、`internal/handlers/playlist.go`、`internal/services/playlist_service.go`、必要的歌单歌曲查询 / 仓储及应用装配 | 返回可回退地址，选取封面候选，复用远程与内部 URL 解析，处理刷新 |
| 备份与插件桥接 | `internal/models/backup.go`、`internal/services/backup_service.go`、`internal/jsplugin/api_bridge.go` | 标签定义和绑定可移植导入导出；扩展 tags 歌单方法 |
| Lynx 标签与分类 | `src/models/song-tag.ts`、`src/features/library/api/song-tags-api.ts`、`data/song-tags-query.ts`、`pages/TagSongsPage.tsx`、`widgets/ManageTagsSheet.tsx`、`TagGridView.tsx`、`TagCard.tsx`，以及歌单 API / 查询 / 编辑入口 | 共享编辑器、歌单 / 歌曲分类、计数和分页、来源显示 |
| Lynx 标题与布局 | `src/features/playlist/widgets/PlaylistCard.tsx`、`PlaylistsView.tsx`、`pages/PlaylistDetailPage.tsx/.css`、`widgets/PlaylistDescPanel.tsx`、`src/shared/ui/MediaListItem.tsx/.css`、`src/features/library/widgets/SongListRow.tsx`、`SongRow.css` | 标题滚动、来源第二行、隐藏内部描述、紧凑头部、计数和歌曲行留白 |
| Flutter 能力对齐 | `lib/features/library/data/song_tags_api.dart`、`presentation/providers/song_tag_provider.dart`、`tag_songs_page.dart`、`widgets/tag_grid_view.dart`、`lib/shared/widgets/manage_tags_modal.dart`，以及歌单 API / 编辑 / 列表 / 详情相关文件 | 同步标签、分类、来源和长标题能力；手机头部按 Flutter 实际布局调整 |
| Navidrome 插件 | `src/sync.ts`、`model.ts`、`main.ts`、必要的 UI 配置入口、`tests/sync.test.mjs` | 来源标签、冲突后缀、稳定命名状态、预览及旧受管歌单迁移 |
| SDK、文案与生成物 | `plugins/toolchain/packages/plugin-sdk/src/global.d.ts`，相关包版本 / 锁文件，各客户端多语言资源，`docs/swagger.json`、`swagger.yaml`、`docs.go`，中英文使用文档 | 桥接类型、依赖兼容、文案、接口契约和验证记录 |

补充有意义的后端、插件和客户端回归测试。名称唯一索引、内置歌单身份和既有歌曲标签关系继续沿用；不增加 `playlist_tag_links` 或显示名列。本次不要求改写 MIoT、WebDAV 或 TV 的业务流程。

## 8. 实施顺序与验证

1. 后端提供共享标签绑定、筛选、统计和事务清理，完善 API 文档；完成标签备份的可移植映射。
2. 修复后端歌单封面契约和远程 / 插件回退，让所有客户端歌单入口共用。
3. 客户端复用自定义标签编辑与分类入口，支持歌曲和歌单浏览，补齐歌单长标题显示；落实详情头部、计数与歌曲行间距优化。
4. Navidrome 增加来源 / 可选连接标签、默认名称分配、预览一致性与受管旧名称迁移；同步更新 SDK 依赖和插件说明。
5. 按完整 #562 场景做端到端验收，封面与布局均是本次交付项，不以标签和命名完成代替 Issue 完成。

| 必验场景 | 预期 |
| --- | --- |
| 同名普通 / 电台歌单、直接创建或改名 | 维持现有全局唯一约束和 HTTP 409 |
| 内置收藏 + 两个远端收藏、名称中已有编号 | 分配可用唯一名称；不修改或接管已有歌单 |
| 并发占用、其他写入错误 | 仅真实重名重试；其他错误原样暴露 |
| 重复同步、原名或较短候选空出、改标签 / 连接名称 | 歌单 ID 和既有名称稳定 |
| 远端改名、显式目标名冲突、宿主手动改名 | 遵守对应命名模式；冲突不覆盖，不丢映射 |
| 旧默认名迁移、无法确认归属、预览后新增占用 | 安全保留现有数据，执行结果可解释 |
| 标签删除与绑定并发、标签 ID 复用 | 无悬空引用或误绑定；系统标记保留 |
| 标签分页 / 隐藏统计、空歌单、跨实例备份 | 列表与计数一致，引用正确映射 |
| 曲库 / 列表 / 首页 / 详情封面，插件地址及失败回退 | 各入口显示一致，独立封面优先，查询和网络请求有界 |
| 窄屏长标题、减少动画、大字号、手机 / 宽屏 | 该滚动时滚动，静态时省略，头部与计数无裁切 |
| 序号与操作留白、多选 / 排序 / 移除确认 | 标题空间增加，44px 点击区域及交互保持正常 |

实现与交付记录见 [#562 验证记录](issue-562-verification.md)。原生设备验收：Not run；按用户授权提交、推送并回复 Issue，保留 Issue 待设备验收，正式宿主版本另行发布。
