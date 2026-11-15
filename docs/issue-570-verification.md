# Issue #570：远程音频缓存校验

关联 [songloft-org/songloft#570](https://github.com/songloft-org/songloft/issues/570)。英文记录见 [English](en/issue-570-verification.md)。

## 验证对象

- 日期：2026-11-15。
- 主仓库基线：`6b3f38c923cf15358a1e7156c6290cac9d43b8c2`；本节验证对象为该基线上的提交前工作树。
- 本次 8 个 Go 文件的内容清单及 SHA-256：`/tmp/songloft-570-code-object.json`，合并摘要 `4d10f4ea8db24823cbf213686715e6dd2551fc16bd4b434b067d2fa28afc64c5`。该摘要覆盖实现与测试，不含本文等文档。
- 环境：Linux x86_64、Go 1.26.0、FFmpeg 5.1.9；临时目录、真实 SQLite 内存数据库、真实本地 HTTP 上游，无生产数据写入。

## 行为与边界

- `moveToCache` 在移动文件前统一执行 `ProbeForValidation` 与现有 `source.Validate`，覆盖外链 `Get`、200 流式代理收尾、206 后台全量下载及缓存格式转换。失败不写入 `cache_path`，临时文件由调用方清理；`FinalizeCache` 返回 `InvalidAudioError`。
- `FindCachedFileBySong` 与 `Get` 在命中时校验，结构化路径和旧目录共用入口。无效文件及同歌曲转码衍生文件被淘汰，数据库路径清空，调用方重新获取音频。播放、音轨探测及显式持久化复用这一入口，移除了播放接口直接返回 `song.CachePath` 的旁路。
- 最多记忆 1024 个文件的技术参数，以文件身份、大小和修改时间判断是否需要重新探测；每次仍用当前配置及歌曲预期时长执行校验。取消请求不被归类为坏音频，不因此淘汰缓存。
- 插件源校验失败记录 `OutcomeValidationFail`，经应用注入的回调进入已有、按歌曲去重的 `AsyncReassign`；其 `ModeFallback` 负责同源重试、备用源搜索和成功后的数据库回写。纯外链没有可自动搜索的插件源，只拒绝坏缓存。
- 保持首播流式交付，整首校验发生在后台完成下载后。因此首次上游提示音仍可能被听到；修复阻止其持久缓存及再次命中，不承诺识别所有内容错误，也不改变所有音源均失败时的播放器重试策略。`source_validation.enabled=false` 保留关闭校验的语义。
- 未修改 lxmusic / miot 插件，没有新增配置或 API；本地歌曲、CUE 片段及按需转码衍生文件不新增整曲时长限制。

## 回归与反向验证

- 服务层：真实 PCM 覆盖短提示音、时长偏短/偏长、低码率、无法解析、外链正常下载、旧缓存重新拉取、衍生文件淘汰与数据库路径清理。
- 命中：覆盖配置热改、预期时长改变、同大小/同修改时间但文件身份变化、原地改写、并发命中及取消请求。
- HTTP：FFmpeg 生成有效 10 秒及 40 秒 MP3；真实 HTTP 200 与 Range 206 代理均保持原始响应字节，10 秒样本不缓存，40 秒样本完整落盘。实际 `GetSongPlay` 请求确认旧提示音被移除并重新缓存正常音频。
- 换源：使用真实 `SourceFetcher`、`SourceResolver`、`SourceOrchestrator`、SQLite 与 HTTP 音频；插件调用为可控夹具。落盘拒绝和旧缓存拒绝均触发主源重试、备用源成功回写及下次获取正常音频，并检查健康度失败记录和临时文件回收。
- 反向 overlay 恢复落盘/命中不校验及原播放接口旁路，保持当前类型签名以运行同一测试。13 个失败子场景准确复现错误落盘、返回旧文件或未换源；正常音频对照通过。日志：`/tmp/songloft-570-before.log`；overlay 路径记录于 `/tmp/songloft-570-overlay-path`。未覆盖当前工作树。
- 既有 seek 参数测试原本使用纯文本假音频，启用校验后按预期无法命中；改用完整 200 秒 PCM 夹具，继续验证原有 seek 行为。

## 检查记录

| 命令 | 结果 / 日志 |
| --- | --- |
| `gofmt -w .`、`git diff --check` | 通过 |
| `node docs/node_modules/vitepress/bin/vitepress.js build docs` | 通过；`/tmp/songloft-570-docs.log`（保留既有包体积告警） |
| `CGO_ENABLED=0 go test -tags dev,lite ./...` | 通过；`/tmp/songloft-570-tests.log` |
| `CGO_ENABLED=0 go vet -tags dev,lite ./...` | 通过；`/tmp/songloft-570-vet.log` |
| `CGO_ENABLED=1 go test -race -tags dev,lite ./internal/services ./internal/handlers -run TestCacheValidation -count=3` | 通过；`/tmp/songloft-570-race.log` |

Not run：macOS/OrbStack arm64 镜像、真实网易音源、小米音箱和原报告实例验收；当前环境为 Linux，无这些设备或实例。客户端 UI、原生包和发布 CI 在本地验证阶段未运行；本次只修改后端缓存行为。提交后的 CI 和版本可用性见 #570 的交付回复。脚本测试不适用（未改脚本）。

最终工作树证据由 `scripts/ai-review-context.mjs commits` 生成，记录于 `/tmp/songloft-570-worktree-20261115-final.json`；源码清单摘要与上述验证对象一致。

建议提交信息：`fix(cache): 统一校验远程音频缓存并淘汰无效文件 (#570)`。
