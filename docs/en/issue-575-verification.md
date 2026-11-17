# Issue #575: Plugin requests use the global HTTP proxy

- Repository / issue: [songloft-org/songloft#575](https://github.com/songloft-org/songloft/issues/575). Open with no comments or linked fixes when investigated; remote main and the local baseline were both `8177b2f208d8436f24a97e08a4ae237f372f88c5`.
- Verification date: 2026-11-17 (Asia/Shanghai). Environment: Linux x86_64, Go 1.26.0, an isolated local backend, temporary database, and proxy fixtures.
- Object: this baseline plus the uncommitted working tree. Code / test content hashes are in `/tmp/songloft-575-20261117/worktree.json`; the repository and submodule snapshot is in `review-context.json` in the same directory. Concurrent playlist fixes and Lynx submodule changes belong to separate work and are outside this fix.

## Cause and change

The lyrics plugin uses the host's global `fetch` for LRCLIB. Neither the normal nor the insecure TLS transport in `internal/jsruntime/runtime.go` had a `Proxy` function, so they read neither Songloft's proxy setting nor proxy environment variables. The setting endpoint already persisted configuration and called `SetGlobalProxy`; the missing connection was in the runtime request path.

Both transports now use `httputil.ProxyFromGlobal` to share the existing thread-safe proxy selection. This covers four client variants (follow / reject redirects × verified / permission-gated insecure TLS), plus `songloft.net.httpStream`, which shares the request entry point. Each request reads the current setting while retaining separate TLS pools, permission checks, control-header stripping, and loopback bypass. No lyrics plugin change or reload is needed. The plugin guide and business constraints were updated in both languages.

## Verification

| Command / scenario | Result and evidence |
| --- | --- |
| `gofmt -w .`, `git diff --check` | Passed |
| `go test ./internal/jsruntime -run 'Test(FetchGlobalProxy\|HTTPStreamGlobalProxy)' -count=1 -v`, original implementation | Four groups failed: client variants, HTTPS CONNECT, live proxy switching in an existing JS environment, and streaming; proxy hit counts were zero. Loopback bypass control passed. `before.log` |
| Same command after the fix | All passed, including redirects, certificate rejection, TLS permission denial / allowance, switching / clearing the proxy, loopback bypass, and response streaming. `after.log` |
| `make check` | Formatting, repository-wide vet and tests passed. `check.log` |
| `go test -race ./internal/httputil ./internal/jsruntime -count=1` | Passed. `race.log` |
| `go build -tags dev,lite -o /tmp/songloft-575-20261117/songloft .` | Passed. `build.log` |
| `python /tmp/songloft-575-20261117/integration.py` | Released plugin's real HTTP API → QuickJS → `fetch` → HTTPS CONNECT path passed. `integration.json` / `backend.log` |

All logs above are in `/tmp/songloft-575-20261117/`. Regression tests use a reserved `.invalid` hostname so proxy failures cannot contact real LRCLIB.

The integration downloaded the released lyrics `v2026.10.17.0` from GitHub, ZIP SHA-256 `fc4a1bc7345982a67daac48a18b98305f5a3617adab0d9c555a40755796f3635`, then installed and enabled it through the upload API. With the plugin environment already running, it saved the fixture proxy through `/settings/http-proxy`, then searched for `Bohemian Rhapsody` / `Queen` through `/jsplugin/lyrics/test-search`.

The fixture accepted CONNECT for `lrclib.net:443` and negotiated TLS 1.3 using a temporary test CA certificate. Only the isolated backend trusted that CA through `SSL_CERT_FILE`; certificate verification remained enabled and system trust was unchanged. Three requests were observed: exact `/api/get`, then `/api/get` and `/api/search` after forcing an exact-match 404. All retained the complete title / artist query, and the plugin returned the expected fixture lyrics. Process `HTTP_PROXY` / `HTTPS_PROXY` deliberately pointed to an unavailable port; the business proxy setting still worked. Switching to a loopback custom provider returned lyrics without increasing the proxy hit count.

## Boundaries and delivery

- Not run: the reporter's Docker / mihomo / actual LRCLIB network, Web UI clicks, and physical devices. Both the lyrics service and proxy in the integration are local fixtures; this does not establish live service acceptance.
- Not run: full embedded packaging, other platform builds, remote CI, and releases. This task performed local backend builds and checks; no frontend code changed.
- No schema, SQL, handler, or route changes in this fix; sqlc and Swagger regeneration are not applicable.
- This record describes pre-commit verification; subsequent commit, push, same-SHA CI, and issue reply evidence is recorded separately in `/tmp/songloft-575-20261117/delivery.json`. Commit: `fix(jsruntime): 插件网络请求接入全局 HTTP 代理，修复 #575`.
