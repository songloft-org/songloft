# #573 扫描排除诊断

关联 [songloft-org/songloft#573](https://github.com/songloft-org/songloft/issues/573#issuecomment-6946173537)。2026-11-17，报告人补充确认定向扫描命中 `yt-dlp` 的排除路径。用户选择保留排除规则，补充解释与校验，不增加临时豁免。

## 行为与接口

- `POST /scan/validate` 复用路径规整与扫描排除匹配，返回 `paths`（未被排除的扫描起点）、`excluded_directories`（`path/rule_type/rule`）。省略或空 `paths` 检查音乐根目录；不遍历子目录、不启动扫描、不修改配置。
- `POST /scan` 再次检查当前规则：所有扫描起点被排除时返回 400，进度状态不变；部分排除仍按原作用域启动，响应列出被排除起点。原有越界校验、目录去重、增量/重导入及清理作用域保留。
- 实际遍历遇到排除目录时剪枝，将路径和命中规则写入 `/scan/progress.excluded_directories`。一个被剪枝的子树只记一个目录，不估算或遍历其中的歌曲数量。新一轮扫描重置诊断。
- Flutter 与 Lynx 在选择目录后显示排除原因，开始前重新检查；全部排除或检查失败时不发送扫描请求，仍保留可调整的控件。混合目录提示跳过情况，扫描结果单独展示目录与规则，不混入“已有歌曲跳过”统计。
- 设置说明区分扫描排除与自动创建歌单排除。只想避免普通单曲目录产生大量歌单时，使用 `auto_create_exclude_dirs`。既有扫描排除的配置保存后清理语义不变。
- Swagger 已重新生成。客户端的新诊断界面需要配套更新服务端；不提供忽略排除的参数。

## 验证对象与证据

时间：2026-11-17；Linux amd64，Go 1.26.0，Flutter 3.44.8 / Dart 3.12.2，Node 22.17.1、pnpm 11.15.1，Docker 无头 Chrome 121。主仓库基线 `4e73b078d86472e0c3409d24a87a6aca2b82380a`，Flutter `c333cf80a035020e38672f9c041fd907e4f65c67`，Lynx `034cf838a46e59a26d1c24df91bf6326f0562e6b`，均加本批未提交工作树。主仓库父进程监视与 Lynx 桌面候选改动属于既有工作，未由本批重置或提交。验证源码文件摘要见 `/tmp/songloft-573-exclusions/source-identities.json`。

| 验证 | 结果 / 日志（均在 `/tmp/songloft-573-exclusions/`） |
| --- | --- |
| `make check` | fmt、vet、全量后端与脚本检查通过，`make-check-final.log` |
| 扫描服务与 handler 定向测试 | 通过，`go-targeted-final.log`、`go-handler-final.log`；覆盖名称/路径/继承/前缀边界、混合目录、越界、全排除不启动、进度重置 |
| `go test -race ./internal/services -run TestScanExclusionDiagnostics -count=1` | 通过，`scan-race.log` |
| Go overlay 分别移除启动拦截、遍历诊断 | 两项回归均按目标行为失败，`negative-handler.log`、`negative-scanner.log`，未覆盖工作树 |
| `make swagger`、完整嵌入后端及 lite 构建 | 通过，`swagger-final.log`、`full-backend-build.log`、`backend-build-final.log`；已核对 `/scan/validate` 和诊断响应类型 |
| Flutter 全量 / 最终定向 / 静态分析 | 853 项全量、4 项最终定向、静态分析通过，`flutter-tests.log`、`flutter-targeted-final.log`、`flutter-analyze-delivery.log` |
| Flutter 隔离旧扫描页面 | 新回归因缺少排除提示失败，`negative-flutter-final.log`；初次隔离副本缺少资源/本地依赖的编译失败不计反向验证 |
| Lynx 相关 / 全量 / 类型 / 构建 | 230 项相关、368 文件/3854 项全量、类型及 Lynx/Web 双产物与 embedded Web 构建通过，`lynx-targeted-final.log`、`lynx-tests.log`、`lynx-types-final.log`、`lynx-build.log`、`lynx-web-build.log`；首轮类型检查曾遇到桌面候选错误，后续检查通过 |
| Lynx 隔离移除拦截与提示 | 两项界面回归因缺少诊断失败，`negative-lynx.log`；隔离环境初次依赖解析失败不计反向验证 |
| 临时真实后端 HTTP | 全排除返回 400 且任务仍 idle；混合目录导入 1 首并报告 1 个排除目录；全库扫描报告名称/路径两种排除；改用歌单排除后歌曲可导入且不创建对应目录歌单。`http-results.json`、`http_probe.py` |
| Flutter Web 产物 | 成功构建，`flutter-web-build.log` |
| Flutter 实际浏览器点击 | 登录、设置导航、目录树勾选、全排除拒绝、混合目录扫描及结果显示通过；两次点击仅产生一次真实扫描请求。`ui-flutter-results.json`、`ui-flutter.js`、`flutter-{blocked,result}-desktop.png` |
| Lynx 浏览器页面事件与后端联调 | 选择被排除子目录后显示继承规则，全排除不启动；混合目录只产生一次扫描请求，终态保留规则，无 pageerror。`ui-lynx-results.json`、`ui-lynx-final.js`、`lynx-{blocked,result}-desktop.png`。Worker 请求由真实后端日志核对，不能仅依赖 page 的 request 事件 |

Lynx 鼠标/触摸驱动未通过：Chrome 121 的焦点委托会使点击跳到页面下方的排除名称输入框；`ui-lynx-debug-raw.json`、`ui-lynx-touch-raw.json`、`lynx-debug.png` 保留失败。后续页面事件联调使用 DOM `click()` 避开该默认焦点动作，只证明扫描交互及后端行为，不作为真实鼠标验收；未在本批修改通用焦点处理。

临时曲库位于 `/home/hanxi/.cache/songloft-573-exclusions-music`，避开默认排除名称 `tmp`，使用合成 MP3 和隔离 SQLite，没有操作用户曲库或生产配置。

Not run：Windows 原生包、Android/iOS/HarmonyOS 真机、报告人的 Docker 挂载与私人音频。用户随后已授权本批提交、推送及回复 Issue；具体提交、子模块指针、同 SHA CI 与评论链接见 `/tmp/songloft-573-exclusions/delivery.json`。不创建正式版本，既有桌面候选与父进程监视改动不在本批提交范围内。
