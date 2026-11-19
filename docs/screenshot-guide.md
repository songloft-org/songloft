# 落地页截图更新指南

文档站首页（落地页）中展示的产品界面截图，是用 [Playwright](https://playwright.dev/) 驱动系统 Chrome、自动登录本地运行的 Songloft 实例后抓取的。脚本为 [`docs/scripts/capture-screenshots.mjs`](https://github.com/songloft-org/songloft/blob/main/docs/scripts/capture-screenshots.mjs)，产物落在 `docs/public/screenshots/`，随仓库提交。

> 需要更新界面截图（如 UI 改版）时，按本文重跑脚本即可，无需手动截图。

## 前置条件

1. **本地跑一个 Songloft 服务**，监听默认端口 `58091`、账号密码为默认的 `admin / admin`：

   ```bash
   make run          # 或任意方式启动完整版（需内置 Web 前端）
   ```

   > 截图抓的是完整版内置的 **Flutter Web 前端**，精简版（lite）不含前端，无法用于截图。
   > 建议库里有若干带封面的歌曲和歌单，截图才好看（脚本会抓首页歌单、歌曲库、播放器等）。

2. **安装依赖**（在 `docs/` 目录，一次即可，`playwright` 已在 `docs/package.json` 里）。脚本用系统已安装的 Google Chrome，无需额外下载浏览器内核：

   ```bash
   cd docs && npm install
   # 若改用 Playwright Chromium，先安装，再将脚本 launch({ channel: "chrome" }) 改为 launch()
   npx playwright install chromium
   ```

## 运行

在 **`docs/` 目录**执行：

```bash
npm run screenshots
```

可用环境变量覆盖默认地址 / 账号：

```bash
SL_URL=http://localhost:58091 SL_USER=admin SL_PASS=admin npm run screenshots
```

脚本会打印每张图的抓取进度，产物覆盖写入 `docs/public/screenshots/`。完成后本地跑 `cd docs && npm run docs:dev` 目测落地页效果，确认无误再提交这些 PNG。

## 抓取了哪些图

每个页面都会抓**亮色 + 暗色**两套，落地页组件按文档站当前主题自动切换。命名规则：亮色无后缀，暗色带 `-dark` 后缀（如 `home-desktop.png` / `home-desktop-dark.png`）。

| 文件（`<page>-<viewport>[-dark].png`） | 说明 |
|------|------|
| `home-{desktop,mobile}` | 首页 |
| `library-{desktop,mobile}` | 歌曲库 |
| `playlists-{desktop,mobile}` | 歌单列表 |
| `playlist-detail-{desktop,mobile}` | 歌单详情 |
| `settings-{desktop,mobile}` | 设置 |
| `player-mobile` | 沉浸式全屏播放器（仅移动端有全屏 Now Playing） |

- 亮色 / 暗色通过服务端 `/settings/user-preferences` 的 `theme_mode` 切换，并同步 `flutter.theme_mode` 本地存储；结束后恢复原服务端偏好。
- 桌面视口 `1440×900`，设备像素比 `1.5`（约 2160px 宽）；移动视口 `390×844`，设备像素比 `2`（约 780px 宽）。
- 落地页组件会用 CSS 给桌面图套「浏览器窗口外框」（`BrowserFrame.vue`）、给移动图套「手机外壳」（`DeviceFrame.vue`），并根据 `isDark` 自动加载 `-dark` 变体，所以脚本只需存**干净的原始截图**。

## 工作原理（为什么不是简单截屏）

前端是 **Flutter Web（canvas 渲染）**，页面上没有常规的 DOM 表单控件，Playwright 的 DOM 选择器抓不到输入框和按钮。脚本因此采用：

1. **坐标 + 键盘登录**：等 Flutter 首帧渲染后，按登录页布局点击用户名 / 密码框并用键盘输入，再点登录按钮。坐标集中在脚本顶部的 `LOGIN` 常量里，登录页若改版需相应微调。
2. **复用登录态**：登录成功后用 `context.storageState()` 把登录信息（`localStorage` 里的 `flutter.access_token` 等）保存下来，注入到桌面 / 移动多个视口的新上下文，**无需为每种视口分别对齐登录坐标**。
3. **hash 路由直达**：各页面按 `#/library`、`#/playlists/1` 等路由直接导航截图（路由取自 `songloft-player-flutter` 的 `AppRoutes`）。
4. **亮 / 暗两套**：先更新服务端 `theme_mode`，再复制登录态并同步 `flutter.theme_mode`，把所有页面再抓一遍（暗色文件带 `-dark` 后缀）。
5. **沉浸式播放器**：移动端进入歌单详情、点第一首开始播放，再点底部迷你播放器展开全屏后截图。

## 需要调整时改哪里

脚本顶部集中了可调项：

- `LOGIN`：登录页控件坐标（登录页改版时微调）。
- `PAGES`：要抓取的页面与对应 hash 路由（增删页面）。
- `VIEWPORTS` / `SCALE`：视口尺寸与设备像素比（分辨率、清晰度）。
- 沉浸式播放器那段的点击坐标：播放页布局改版时微调。

> 提示：坐标依赖具体布局，属于「肉眼校对」的脆弱环节。跑完后务必人工过一遍产物，若某张图落在了「页面未找到」或登录页，多半是坐标或路由需要更新。
