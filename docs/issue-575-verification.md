# Issue #575：插件请求使用全局 HTTP 代理

- 仓库 / Issue：[songloft-org/songloft#575](https://github.com/songloft-org/songloft/issues/575)。调查时 open，无评论或关联修复；远端 main 与本地基线均为 `8177b2f208d8436f24a97e08a4ae237f372f88c5`。
- 验证时间：2026-11-17（Asia/Shanghai）。环境：Linux x86_64，Go 1.26.0，隔离的本地后端、临时数据库及代理夹具。
- 对象：上述基线加本次未提交工作树。代码 / 测试内容哈希见 `/tmp/songloft-575-20261117/worktree.json`，全仓库与子模块快照见同目录 `review-context.json`。同期歌单修复与 Lynx 子模块改动属于其他工作，不包含在本次修复范围内。

## 根因与修改

lyrics 的 LRCLIB 请求使用宿主全局 `fetch`；`internal/jsruntime/runtime.go` 的普通和跳过证书校验两个独立 Transport 均未设置 `Proxy`，所以既不读取 Songloft 的代理配置，也不使用代理环境变量。配置端点已经持久化并调用 `SetGlobalProxy`，缺口在运行时发请求的路径。

通过 `httputil.ProxyFromGlobal` 向两个 Transport 接入现有的并发安全代理选择逻辑。四个客户端变体（普通 / 不跟随重定向 × 正常 TLS / 获权限跳过 TLS）以及复用请求入口的 `songloft.net.httpStream` 均覆盖。每次请求读取当前代理；保留独立 TLS 连接池、权限、控制头剥离及回环直连规则。无需改 lyrics 插件，也无需重载插件；中英文插件指南及业务约束已同步。

## 验证

| 命令 / 场景 | 结果与证据 |
| --- | --- |
| `gofmt -w .`、`git diff --check` | 通过 |
| `go test ./internal/jsruntime -run 'Test(FetchGlobalProxy\|HTTPStreamGlobalProxy)' -count=1 -v`，旧实现 | 代理客户端变体、HTTPS CONNECT、已有 JS 环境的代理切换、流请求四组失败，实际代理命中为 0；回环直连对照通过。`before.log` |
| 同一命令，修复后 | 全部通过；包含重定向行为、TLS 证书拒绝、权限拒绝与授权放行、代理切换 / 清空、回环直连及流响应。`after.log` |
| `make check` | fmt、全仓库 vet / 测试通过。`check.log` |
| `go test -race ./internal/httputil ./internal/jsruntime -count=1` | 通过。`race.log` |
| `go build -tags dev,lite -o /tmp/songloft-575-20261117/songloft .` | 通过。`build.log` |
| `python /tmp/songloft-575-20261117/integration.py` | 已发布插件的真实 HTTP API → QuickJS → `fetch` → HTTPS CONNECT 链路通过。`integration.json` / `backend.log` |

上述日志均在 `/tmp/songloft-575-20261117/`。回归测试使用保留的 `.invalid` 域名；代理失败不会请求真实 LRCLIB。

集成验证从 GitHub 下载正式的 lyrics `v2026.10.17.0`，压缩包 SHA-256 为 `fc4a1bc7345982a67daac48a18b98305f5a3617adab0d9c555a40755796f3635`，经上传 API 安装、启用。在已有插件环境中调用 `/settings/http-proxy` 保存夹具代理，再通过 `/jsplugin/lyrics/test-search` 搜索 `Bohemian Rhapsody` / `Queen`。

夹具对 `lrclib.net:443` 接受 CONNECT，以临时测试 CA 签发的证书完成 TLS 1.3；仅隔离后端通过 `SSL_CERT_FILE` 信任该 CA，未打开跳过证书校验，也未更改系统信任。观察到三次请求：精确匹配 `/api/get`，强制精确匹配 404 后的 `/api/get` 和 `/api/search`；三次均保留完整歌名、歌手参数，插件返回预期夹具歌词。进程的 `HTTP_PROXY` / `HTTPS_PROXY` 故意指向不可用端口，仍按业务设置代理成功。切换为回环地址上的自定义提供者后，歌词返回成功，代理命中数不增加。

## 边界与交付

- Not run：报告人的 Docker / mihomo / 实际 LRCLIB 网络、Web 界面点击、真实设备。集成中的歌词服务和代理均为本地夹具，不能视为线上歌词服务验收。
- Not run：完整嵌入版打包、其他平台构建、远端 CI、正式发布；本次仅完成后端本地构建与检查，无前端改动。
- 不涉及 schema、SQL、handler 或路由变更，无需重新生成 sqlc 或 Swagger。
- 本记录描述提交前验证；后续提交、推送、同 SHA CI 与 Issue 回复证据另存 `/tmp/songloft-575-20261117/delivery.json`。提交信息：`fix(jsruntime): 插件网络请求接入全局 HTTP 代理，修复 #575`。
