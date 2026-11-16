# 专辑音轨排序验证

目标：[songloft-org/songloft#571](https://github.com/songloft-org/songloft/issues/571)。2026-11-16，Linux amd64 / Go 1.26.0 / Node 22.17.1，临时数据库与本地合成音频。

## 验证对象

| 仓库 | 基线 |
| --- | --- |
| songloft-org/songloft | `7180df38176ca7b44c1a7d8cf2b3ec269d35ac7d` |
| hanxi/tag（pkg/tag） | `64e12e525cc5c9a5ee66826c98dbaed42534af8b` |
| songloft-org/songloft-player-lynx | `b406af75043d2c2d898b7f73b0f282248d293d33` |

本地验证基于以上基线加本批工作树；提交前各仓库 tracked diff、暂存区及 untracked 文件 SHA-256 记录在工作树外的 `/tmp/songloft-571-snapshot.json`。交付时先提交并推送两个子仓库，再提交主仓库及 gitlink；最终提交与同 SHA CI 结果见 #571 的交付回复。本批不创建正式版本。

## 行为及原因

- 原先 HTTP 默认排序固定为添加时间，仓储没有轨号排序；Lynx 专辑页还显式发送添加时间倒序。更改标签不会改变这条排序规则。
- 新增 `sort=track`；带 `album` 且未指定排序时默认使用它，方向缺省升序。显式指定其他排序仍生效。
- 排序在 SQLite 的 LIMIT/OFFSET 之前执行，完整歌曲及 ID 列表共用：碟号数字 → 音轨号数字 → 文件路径 → ID。缺失或无效碟号视为第 1 碟；同碟无有效音轨号排在末尾；已有 CUE 虚拟轨用 `cue_track_index` 回退。
- goose 0042 新增 `disc`，扫描、重新导入、仓储、歌曲响应及标签写入均传递碟号。成功重新提取后同步文件的轨号和碟号，包括外部删除标签的情形。升级不会自动重扫曲库；既有专辑需执行一次重新导入，补齐碟号。
- 标签库支持 MP3、FLAC、M4A、OGG、APE、AIFF 碟号写回；未传碟号的旧调用者保留文件原碟号。WAV 没有新增碟号写入映射。
- FLAC/OGG 原读取器对合写的 `TRACKNUMBER=02/12`、`DISCNUMBER=1/2` 使用整数转换，丢失编号；现接受合写、拆分字段及 totaltracks/totaldiscs 别名。
- Flutter 分类页与补齐队列没有显式排序，直接使用服务端的新专辑默认值；Flutter 源码未改动。Lynx 专辑页改为显式请求 `track/asc`。

## 实际检查

| 检查 | 结果与证据 |
| --- | --- |
| `CGO_ENABLED=0 go test -tags dev,lite ./...` | 通过；`/tmp/songloft-571-tests-final.log` |
| `CGO_ENABLED=0 go vet -tags dev,lite ./...` | 通过；`/tmp/songloft-571-vet-final.log` |
| 标签库 `go test ./...` | 通过，包含六种格式碟号读写及旧调用者保留；`/tmp/songloft-571-tag.log` |
| 真实 SQLite + HTTP 回归 | 数字 1/2/10、多碟、缺失/非法轨号、CUE、分页、ID 列表、显式排序和标签修改后重排通过 |
| 扫描与重新导入回归 | 真实 FLAC → 提取 → 事务入库 → 修改文件 → 重新提取/入库 → 标签写回；补验外部删除编号后数据库清空；`/tmp/songloft-571-service-final.log` |
| race | 目标 handler/service 回归三轮；`/tmp/songloft-571-race-final.log` |
| Lynx `pnpm exec tsc -b` | 通过；`/tmp/songloft-571-lynx-types.log` |
| Lynx `pnpm test` | 363 文件、3821 测试通过；`/tmp/songloft-571-lynx-tests.log`；最终分类页定向 12 项通过 |
| Lynx `pnpm run build` | Lynx/Web 双产物通过；`/tmp/songloft-571-lynx-build.log`；保留原有不支持 CSS 属性等构建警告 |
| `make sqlc` / `make swagger` | 已重新生成；Swagger 的 /songs、/songs/ids 包含 track，歌曲及写标签请求包含 disc |

反向验证均使用隔离副本/Go overlay，不覆盖当前工作树：

- 恢复旧的排序机制后，专辑顺序、分页和 ID 列表等断言失败：`/tmp/songloft-571-negative/sort.log`。
- 移除碟号提取后，扫描回归因碟号为空失败：`metadata.log`。
- 移除 MP3 的 TPOS 写入后，碟号读回/保留失败：`tag.log`。
- 使用旧 Vorbis 读取器，合写及别名回归失败：`vorbis.log`。
- Lynx 隔离源码恢复旧专辑页，新增渲染回归因仍请求 added_at/desc 失败：`lynx.log`。

## 浏览器实用验证

六首带真实音轨号与碟号的合成 MP3，经临时服务真实扫描后，Docker Chrome 中点击曲库 → 专辑 → Issue571 → 播放全部 → 队列。列表与队列均为：

`D1T1 → D1T2 → D1T10 → D2T1 → D2T2 → D2T10`。

音频实际时间推进至 0.688924 秒，播放地址对应 D1T1；页面异常为零。结果及截图：`/tmp/songloft-571-browser-queue.json`、同名 PNG。HTTP 扫描与标签证据在 `/tmp/songloft-571-http.json`。

通过真实标签接口把 D1T10 改为第 2 碟第 3 轨，返回 `file_write=written`；重新进入专辑后，界面顺序变成 `D1T1 → D1T2 → D2T1 → D2T2 → D1T10 → D2T10`，页面异常为零。证据 `/tmp/songloft-571-browser-edit-final.json` 及 PNG。轨号编辑通过 API 完成，未宣称客户端新增了轨号编辑控件。

## 边界

Not run（本地验证阶段）：Android/iOS/HarmonyOS 真机、Flutter 界面与 Bundle 构建、报告人原始专辑（Issue 未附样本或平台）、远端 CI 和发布。推送后 CI 结果另按同 SHA 核对并回复 Issue。浏览器和真实 SQLite 回归不代表真机验收。临时浏览器脚本首轮因选择器/音频对象探测错误超时，修正后取得上述成功证据；不将超时当通过。
