# #573 排查与验证记录

目标：[songloft-org/songloft#573](https://github.com/songloft-org/songloft/issues/573)。本次只确认并修复扫描影响歌单排序的缺陷；移动后歌曲不可见尚未复现，不能据此关闭 Issue。

## 验证对象

- 时间：2026-11-17；Linux amd64、Go 1.26.0、系统 ffmpeg/ffprobe。
- 主仓库基线：`8177b2f208d8436f24a97e08a4ae237f372f88c5` 加本次未提交工作树。
- 四个源码/测试文件的内容标识：`cde1222c76aa23b4e539a16c15a7ff0bae49fd532a6494d530106e2cf9ce180b`。按路径排序，将每条 `路径 + NUL + 文件内容 + NUL` 累积为 SHA-256；清单见 `/tmp/songloft-573-20261117/source-identity.json`。
- 开始时 `clients/player-lynx` 已有未提交改动，本次没有修改子模块。
- 执行期间另有代理、JS 运行时及对应文档/测试变更出现，均未由本任务修改。完整构建于 15:42 完成；`make check` 于 15:42 启动、15:43 完成，不作为期间并发变更的验收。本次四个文件的内容标识及最终数据库定向测试独立记录。

## 结论与修改

1. 默认歌单排序为 `position ASC, updated_at DESC`。自动创建原来不设置位置，落到默认的 0，可能插到已排序歌单前；现在与普通创建保持一致，在同一事务内取最大位置加一。
2. 扫描结束重建自动歌单时，无条件 UPDATE 会触发 `update_playlists_updated_at`，即使内容没有变化，也改变旧歌单位置相同时的顺序。现在 SQL 只在名称、描述或封面确实变化时更新。成员仍按扫描结果刷新；实际元数据变化仍更新时间，已有 ID、自定义位置、置顶和排序偏好保留。
3. 已阅读正文、全部评论（当时为零）、事件及 #267，检查三张截图和日志附件。报告的“录入 0、跳过 0、失败 0”指向未发现待处理音频的方向，但现有日志没有请求目录、排除配置或文件列表，不能确认具体原因，也不能把 Windows 共享目录可见当作容器内可见的证据。已向用户请求音乐根目录、排除名称/路径和所选容器内完整路径。

## 验证

| 命令或场景 | 结果与证据 |
| --- | --- |
| `make sqlc`、根目录 `gofmt -w .` | 通过，生成物同步；`sqlc.log` |
| `make check` | fmt、vet、全量 Go 测试通过；`check.log`。已有说明性 `TestSongServiceScanAndImportNote` 仍跳过，本次实际扫描另由 HTTP 场景验证 |
| `go test ./internal/database -run TestAutoCreate -count=1` | 最终测试通过；`order-after.log`、`order-final.log`。覆盖旧歌单同位置排序、成员增加、缺失封面补齐、新歌单追加、自定义位置/置顶/视图排序保留 |
| 恢复基线生成查询的 Go overlay，运行最终两个排序回归 | 两项因预期行为失败：旧歌单时间/顺序变化、新歌单跑到手动歌单前；`order-negative-final.log`，未回退当前工作树 |
| `go build -tags dev -o /tmp/songloft-573-20261117/songloft .` | 完整嵌入版构建通过 |
| 临时真实后端、SQLite 和合成 MP3 的 HTTP 操作 | 首次全扫导入 2 首；从错误目录搬回中文目录后定向扫描导入 1、跳过 1；路径筛选得到 2 首，自动歌单包含 2 首，手工加入新歌单实际添加 2 首；重复扫描保留列表顺序和修改时间；后续全扫清理 1 条原路径失效记录。`http-results.json`、`http_probe.py`、`http-server.log` |
| `git diff --check` | 通过 |

日志均在 `/tmp/songloft-573-20261117/`。临时曲库位于 `/home/hanxi/.cache/`，避开默认排除名 `tmp`；没有使用用户音频或修改现有数据库。定向扫描保留范围外的旧路径记录是既有契约，全扫才清理此次搬移留下的原路径记录。

Not run：报告人的 Docker 挂载、实际目录配置和原始音频；Windows Flutter 界面操作；前端构建和其他设备；本地验证时尚未运行本次修复的远端 CI/发布。API 未变更，无需重新生成 Swagger。提交、推送、CI 和版本可用性以 Issue 后续回复为准。提交信息：`fix(scan): 保留自动歌单扫描顺序并追加新歌单 (Ref #573)`。
