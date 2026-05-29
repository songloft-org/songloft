# Songloft 前端架构说明

> **独立仓库**: [https://github.com/songloft-org/songloft-player](https://github.com/songloft-org/songloft-player)

Songloft 前端是一个基于 Flutter 的跨平台音乐播放器，支持 **Android、iOS、macOS、Windows、Linux、Web** 六个平台。Flutter Web 构建产物可嵌入到 Go 后端二进制中一起分发。

## 技术栈

- **框架**: Flutter 3.29+ / Dart 3.7+
- **状态管理**: flutter_riverpod ^3.1.0（手写 Provider，不使用 code generation）
- **路由**: go_router ^17.1.0（声明式路由 + ShellRoute）
- **HTTP 客户端**: dio ^5.7.0
- **音频播放**: just_audio ^0.10.5 + audio_service ^0.18.17
- **Windows/Linux 音频后端**: just_audio_media_kit（基于 libmpv）
- **本地存储**: shared_preferences ^2.3.4
- **图片缓存**: cached_network_image ^3.4.1
- **颜色提取**: palette_generator ^0.3.3+4
- **WebView**: flutter_inappwebview ^6.1.5（JS 插件页面加载）
- **权限管理**: permission_handler ^12.0.1
- **UI 框架**: Material 3（seedColor: indigo-500）

## 设计理念

- **以音乐播放为核心**：播放器始终可见，随时可控
- **响应式四端适配**：Mobile / Tablet / Desktop / TV 自适应布局
- **Feature-First 架构**：按功能模块组织代码，每个模块包含 data / domain / presentation 三层
- **跨平台一致体验**：一套代码适配 6 个平台，针对各平台特性做优化

## 目录结构

```
songloft-player/lib/
├── config/                          # 应用配置
│   ├── app_config.dart              # API 配置、部署模式、版本号
│   └── constants.dart               # 应用常量
├── core/                            # 核心基础设施
│   ├── audio/
│   │   ├── audio_service.dart       # SongloftAudioHandler（音频播放、通知栏控制）
│   │   └── system_volume_provider.dart  # 系统音量 Provider（基于 volume_controller）
│   ├── network/
│   │   ├── api_client.dart          # Dio HTTP 客户端封装
│   │   ├── api_exceptions.dart      # API 异常定义
│   │   └── auth_interceptor.dart    # JWT Token 自动刷新拦截器
│   ├── router/
│   │   └── app_router.dart          # GoRouter 路由配置（含认证守卫）
│   ├── storage/
│   │   ├── app_preferences.dart     # SharedPreferences 封装
│   │   ├── lyric_cache_service.dart # 歌词本地缓存
│   │   └── secure_storage.dart      # 安全存储（Token 缓存）
│   ├── theme/
│   │   ├── app_theme.dart           # Material 3 主题（亮色/暗色，响应式）
│   │   ├── app_dimensions.dart      # 尺寸和圆角常量
│   │   ├── responsive.dart          # 响应式断点和工具扩展
│   │   └── tv_theme.dart            # TV 端专用主题常量
│   └── utils/
│       ├── color_extraction.dart    # 封面颜色提取
│       ├── formatters.dart          # 格式化工具（时长、文件大小等）
│       ├── platform_utils.dart      # 平台检测工具
│       └── url_helper.dart          # URL 构建辅助（base_url 拼接、token 附加等）
├── features/                        # 功能模块
│   ├── auth/                        # 认证模块
│   │   ├── data/
│   │   │   ├── auth_api.dart        # 认证 API
│   │   │   └── auth_repository.dart # 认证仓储
│   │   ├── domain/
│   │   │   └── auth_state.dart      # 认证状态定义
│   │   └── presentation/
│   │       ├── login_page.dart      # 登录页面
│   │       └── providers/
│   │           └── auth_provider.dart
│   ├── home/                        # 首页模块
│   │   └── presentation/
│   │       ├── home_page.dart       # 首页（歌单轮播、JS 插件网格）
│   │       ├── plugin_webview_page.dart      # JS 插件 WebView 页面（条件导入）
│   │       ├── plugin_webview_page_native.dart  # 原生平台 WebView 实现
│   │       ├── plugin_webview_page_stub.dart    # Web 平台 stub
│   │       └── widgets/
│   │           └── playlist_carousel.dart   # 歌单轮播组件
│   ├── jsplugin/                    # JS 插件模块
│   │   ├── data/
│   │   │   └── jsplugin_api.dart    # JS 插件 API（含 JSPlugin 模型、上传、更新检查）
│   │   └── presentation/
│   │       ├── providers/
│   │       │   └── jsplugin_provider.dart   # JSPluginApi Provider / jsPluginsProvider
│   │       └── widgets/
│   │           ├── jsplugin_grid.dart       # JS 插件入口网格（首页用）
│   │           └── jsplugin_manager.dart    # JS 插件管理面板（设置页用）
│   ├── library/                     # 歌曲库模块
│   │   ├── data/
│   │   │   ├── songs_api.dart       # 歌曲 API
│   │   │   └── songs_repository.dart
│   │   └── presentation/
│   │       ├── library_page.dart    # 歌曲库页面
│   │       ├── song_edit_page.dart  # 歌曲编辑页面
│   │       ├── providers/
│   │       │   ├── songs_provider.dart
│   │       │   └── favorite_provider.dart
│   │       └── widgets/
│   │           ├── song_list_tile.dart   # 歌曲列表项
│   │           └── song_filter_bar.dart  # 歌曲筛选栏
│   ├── player/                      # 播放器模块
│   │   ├── domain/
│   │   │   ├── player_state.dart    # 播放器状态定义
│   │   │   └── lyric_parser.dart    # LRC 歌词解析器
│   │   └── presentation/
│   │       ├── queue_page.dart      # 播放队列页面
│   │       ├── providers/
│   │       │   └── player_provider.dart
│   │       └── widgets/
│   │           ├── desktop_player.dart    # 桌面端播放器（迷你/侧栏形式）
│   │           ├── desktop_full_player.dart  # 桌面端全屏播放器
│   │           ├── mobile_player.dart     # 移动端全屏播放器
│   │           ├── tv_player.dart         # TV 端播放器
│   │           ├── mini_player.dart       # 迷你播放器条
│   │           ├── play_controls.dart     # 播放控制按钮
│   │           ├── popup_controls.dart    # 弹出式控制面板
│   │           ├── progress_bar.dart      # 进度条
│   │           ├── volume_control.dart    # 音量控制
│   │           ├── lyrics_view.dart       # 歌词显示
│   │           └── playlist_drawer.dart   # 播放列表抽屉
│   ├── playlist/                    # 歌单模块
│   │   ├── data/
│   │   │   ├── playlist_api.dart    # 含歌单 CRUD + 网络歌曲转本地 (convertPlaylistToLocal / getConvertProgress / cancelConvert) + 自动转换开关
│   │   │   └── playlist_repository.dart
│   │   ├── domain/
│   │   │   └── playlist.dart        # 歌单模型
│   │   └── presentation/
│   │       ├── playlists_page.dart   # 歌单列表页
│   │       ├── playlist_detail_page.dart  # 歌单详情页(含"转换为本地"入口 + 进度 banner + 跨页面恢复轮询)
│   │       ├── providers/
│   │       │   ├── playlist_provider.dart
│   │       │   └── playlist_view_provider.dart
│   │       └── widgets/
│   │           ├── playlist_card.dart         # 歌单卡片
│   │           ├── playlist_list_item.dart     # 歌单列表项
│   │           └── song_cover_picker_modal.dart  # 歌曲封面选择弹窗
│   └── settings/                    # 设置模块
│       ├── data/
│       │   ├── cache_api.dart       # 音乐缓存 API（容量查询、清理）
│       │   ├── config_api.dart      # 配置 API
│       │   ├── directory_api.dart   # 目录浏览 API（音乐目录选择器用）
│       │   ├── frontend_version_api.dart  # 前端版本检查 API
│       │   ├── scan_api.dart        # 扫描 API
│       │   └── upgrade_api.dart     # 升级 API
│       └── presentation/
│           ├── settings_page.dart   # 设置页面
│           ├── providers/
│           │   └── settings_provider.dart
│           └── widgets/
│               ├── cache_manager.dart        # 音乐缓存管理面板
│               ├── config_manager.dart       # 配置管理
│               ├── exclude_dir_manager.dart  # 扫描排除目录管理
│               ├── frontend_upgrade_dialog.dart  # 前端升级对话框
│               ├── scan_manager.dart         # 扫描管理
│               ├── theme_selector.dart       # 主题选择器
│               ├── token_manager.dart        # 令牌管理
│               └── upgrade_dialog.dart       # 后端升级对话框
│               // settings_page 在"音乐库管理"分组下含 SwitchListTile:网络歌曲自动转为本地,
│               // 对应 providers/settings_provider.dart::autoConvertEnabledProvider
└── shared/                          # 共享模块
    ├── layouts/
    │   ├── shell_layout.dart        # ShellRoute 主布局（导航 + 播放器）
    │   └── adaptive_scaffold.dart   # 自适应脚手架
    ├── models/
    │   ├── song.dart                # 歌曲模型
    │   ├── pagination.dart          # 分页模型
    │   └── api_response.dart        # API 响应模型
    ├── utils/
    │   └── responsive_snackbar.dart # 响应式 SnackBar
    └── widgets/                     # 共享组件（11 个）
        ├── cover_image.dart         # 封面图片组件
        ├── favorite_button.dart     # 收藏按钮
        ├── scrolling_text.dart      # 滚动文本
        ├── confirm_dialog.dart      # 确认对话框
        ├── add_to_playlist_modal.dart  # 添加到歌单弹窗
        ├── song_picker_modal.dart   # 歌曲选择弹窗
        ├── empty_state.dart         # 空状态
        ├── error_view.dart          # 错误视图
        ├── loading_indicator.dart   # 加载指示器
        ├── tv_focusable.dart        # TV 焦点组件
        └── tv_grid_view.dart        # TV 网格视图
```

## 页面结构

### 路由配置

| 页面 | 路由 | 说明 |
|------|------|------|
| 登录 | `/login` | 登录页面（独立路由，不使用 ShellRoute） |
| 首页 | `/` | 歌单轮播、JS 插件网格 |
| 歌曲库 | `/library` | 所有歌曲列表、搜索、筛选 |
| 歌单 | `/playlists` | 歌单列表 |
| 歌单详情 | `/playlists/:id` | 歌单详情和歌曲列表 |
| 设置 | `/settings` | 主题、扫描、JS 插件、令牌、升级、关于 |
| 插件 | `/plugin?url=&name=` | JS 插件 WebView 页面（全屏，独立路由） |

### 认证守卫

路由使用 GoRouter 的 `redirect` 机制实现认证守卫：
- 未认证 → 重定向到 `/login`
- 已认证且在登录页 → 重定向到 `/`
- 认证状态未确定（正在恢复 Token）→ 不做跳转

## 响应式布局

### 断点定义

| 屏幕类型 | 宽度范围 | 说明 |
|---------|---------|------|
| **Mobile** | < 600px | 底部导航 + 迷你播放器 |
| **Tablet** | 600 - 900px | 底部导航 + 迷你播放器（更宽） |
| **Desktop** | 900 - 1920px | 侧边导航 + 底部播放器栏 |
| **TV** | ≥ 1920px | 焦点导航 + 大尺寸 UI + D-pad 支持 |

### 布局架构

```
ShellLayout (ShellRoute builder)
├── AdaptiveScaffold
│   ├── Mobile/Tablet: NavigationBar (底部) + MiniPlayer
│   ├── Desktop: NavigationRail (侧边) + DesktopPlayer (底部)
│   └── TV: 顶部 Tab 导航 + TvPlayer
└── 内容区域 (GoRouter child)
```

## 主题系统

### Material 3 配色

- **主色调**: indigo-500 (`#6366F1`)
- **配色方案**: `ColorScheme.fromSeed(seedColor: indigo-500)`
- **主题模式**: 亮色 / 暗色 / 跟随系统
- **字体回退**: NotoSansSC（中文支持）

### 响应式主题

主题根据屏幕类型动态调整组件尺寸：
- **SnackBar**: Desktop/TV 使用固定宽度居中显示
- **FilledButton**: TV 端使用更大的最小尺寸
- **对话框**: 根据屏幕类型调整最大宽度

### TV 端专用主题

`TvTheme` 类定义了 TV 端的尺寸常量：
- 字体大小：标题 24sp、正文 20sp、副标题 16sp
- 焦点效果：3px 边框 + 1.05x 缩放
- 网格布局：4 列、24px 间距、48px 内边距

## 部署模式

### 嵌入模式（Embedded）

```bash
flutter build web --dart-define=DEPLOY_MODE=embedded
```

- Flutter Web 嵌入 Go 后端，同域访问
- `AppConfig.baseUrl` 自动设为 `Uri.base.origin`
- **隐藏** 登录页 API 地址输入框和设置页 API 配置
- `AppConfig.isEmbedded` 是编译时常量，tree-shaking 会移除 API 地址 UI 代码

### 独立部署模式（Standalone，默认）

```bash
flutter build web --dart-define=DEPLOY_MODE=standalone
```

- 前后端分离部署
- **显示** API 地址配置 UI，支持用户手动填写后端地址
- API 地址持久化到本地存储

## 音频播放架构

```
SongloftAudioHandler (extends BaseAudioHandler)
├── just_audio (核心播放引擎)
│   ├── Web: HTML5 Audio
│   ├── Android/iOS: 原生播放器
│   └── Windows/Linux: media_kit (libmpv)
├── audio_service (系统通知栏/锁屏控制)
└── audio_session (音频焦点管理)
```

### 平台适配

- **Android**: 前台服务持续运行（`androidStopForegroundOnPause: false`），兼容 HyperOS3 等激进回收策略
- **Android 13+**: 运行时请求通知权限
- **macOS**: secure_storage 未签名时自动降级到 SharedPreferences
- **Windows/Linux**: 使用 `just_audio_media_kit` 作为音频后端

## 开发命令

```bash
cd songloft-player
flutter pub get                    # 安装依赖
flutter run -d chrome              # Web 调试（standalone 模式）
flutter run -d chrome --dart-define=DEPLOY_MODE=embedded  # 模拟嵌入模式
flutter run -d macos               # macOS 调试
flutter run -d windows             # Windows 调试
flutter run -d linux               # Linux 调试
flutter analyze                    # 静态分析
flutter test                       # 运行测试
```

### 构建命令

```bash
# Web 嵌入模式（输出至 songloft-player-build/web-embedded，供 Go 二进制 //go:embed）
make build-frontend-web-embedded

# Web 独立部署版
make build-frontend-web

# 桌面版
make build-frontend-linux
make build-frontend-windows
make build-frontend-macos

# Android 版（APK + AAB）
make build-frontend-android

# iOS 版（仅 macOS）
make build-frontend-ios

# 当前系统支持的所有平台
make build-frontend-all
```

预编译安装包下载: [https://github.com/songloft-org/songloft-player/releases](https://github.com/songloft-org/songloft-player/releases)
