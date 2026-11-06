# Songloft Flutter 前端架构说明

> **独立仓库**: [https://github.com/songloft-org/songloft-player](https://github.com/songloft-org/songloft-player)

本文描述 Flutter 客户端。ReactLynx + TypeScript 的 Lynx 预览客户端支持 Android/iOS/HarmonyOS/Web，详见[客户端选择](clients.md)及 [Lynx 指南](player-lynx/index.md)；它尚无桌面或 Bundle 本地后端，不沿用下面的 Flutter 目录与构建命令。

Songloft Flutter 客户端支持 **Android、iOS、macOS、Windows、Linux、Web** 六个平台。完整版服务器默认嵌入独立的 Lynx Web（`clients/player-lynx`）；旧 Flutter Web 嵌入版仍可显式选择。同时支持 **Bundle 本地模式**：将 Go 后端嵌入 Flutter 客户端（移动端通过 gomobile 原生库，桌面端通过子进程），无需单独部署服务器即可播放本地音乐。

## 技术栈

- **框架**: Flutter 3.41+ / Dart 3.7+
- **状态管理**: flutter_riverpod ^3.1.0（手写 Provider，不使用 code generation）
- **路由**: go_router ^17.1.0（声明式路由 + ShellRoute）
- **HTTP 客户端**: dio ^5.7.0
- **音频播放**: just_audio ^0.10.5 + audio_service ^0.18.17
- **音频后端**: 所有原生平台（Win/Linux/macOS/Android/iOS）统一 just_audio_media_kit（libmpv，无回退）；Web 用 just_audio_web + 自接 hls.js
- **视频画面**: media_kit_video（原生平台复用同一 libmpv Player 派生 VideoController；Web 用静音 `<video>` 同步，songloft-org/songloft#76）
- **本地存储**: shared_preferences ^2.3.4
- **图片缓存**: cached_network_image ^3.4.1
- **颜色提取**: palette_generator ^0.3.3+4
- **WebView**: flutter_inappwebview ^6.1.5（JS 插件页面加载）
- **权限管理**: permission_handler ^12.0.1
- **UI 框架**: Material 3（seedColor: M3 Blue baseline `#415F91`）

## 设计理念

- **以音乐播放为核心**：播放器始终可见，随时可控
- **响应式三端适配**：Mobile / Tablet / Desktop 自适应布局
- **Feature-First 架构**：按功能模块组织代码，每个模块包含 data / domain / presentation 三层
- **跨平台一致体验**：一套代码适配 6 个平台，针对各平台特性做优化

## 目录结构

```
clients/player/lib/
├── config/                          # 应用配置
│   ├── app_config.dart              # API 配置、部署模式、版本号
│   └── constants.dart               # 应用常量
├── core/                            # 核心基础设施
│   ├── a11y/                        # 无障碍辅助
│   │   ├── web_semantics_controller.dart  # Web 端语义树控制器
│   │   └── semantics_pointer_override*.dart  # 指针事件语义覆盖（条件导入）
│   ├── audio/                       # 音频播放引擎（20 个文件）
│   │   ├── audio_service.dart       # SongloftAudioHandler（音频播放、通知栏控制）
│   │   ├── songloft_just_audio_platform.dart  # 自定义 JustAudio 平台注册
│   │   ├── songloft_mediakit_player.dart      # libmpv 原生播放器封装
│   │   ├── songloft_web_audio_player.dart     # Web 端 HTML5 Audio + hls.js 播放器
│   │   ├── equalizer_service*.dart            # 均衡器服务（mpv / web 双实现）
│   │   ├── smtc_service*.dart                 # Windows SMTC 媒体控制（条件导入）
│   │   ├── video_controller_provider.dart     # 视频画面控制器 Provider
│   │   ├── system_volume_provider.dart        # 系统音量 Provider
│   │   └── media_browse_data_source.dart      # Android Auto 媒体浏览数据源
│   ├── backend/                     # Bundle 本地模式（嵌入后端抽象层）
│   │   ├── embedded_backend_service.dart   # 统一接口（移动端 MethodChannel / 桌面端子进程分发）
│   │   ├── desktop_backend_service.dart    # 桌面端：启动 songloft-server 子进程
│   │   ├── native_contract_service.dart    # 原生端服务协议
│   │   ├── run_mode_provider.dart          # RunMode 枚举（local/remote）+ 持久化 Provider
│   │   └── backend_lifecycle.dart          # WidgetsBindingObserver：前台恢复自动重启后端
│   ├── network/                     # 网络层（19 个文件）
│   │   ├── api_client.dart          # Dio HTTP 客户端封装
│   │   ├── api_exceptions.dart      # API 异常定义
│   │   ├── auth_interceptor.dart    # JWT Token 自动刷新拦截器
│   │   ├── base_url_provider.dart   # 基础 URL Provider
│   │   ├── server_probe.dart        # 服务器探测
│   │   ├── servers_provider.dart    # 多服务器管理 Provider
│   │   ├── lan_address.dart         # 局域网地址发现
│   │   ├── github_proxy_fallback.dart  # GitHub 代理回退
│   │   └── ...                      # TLS / 重定向 / 媒体代理等条件导入
│   ├── platform/
│   │   ├── live_activity_service.dart   # iOS 灵动岛 / 实时活动集成
│   │   └── home_widget_service.dart     # 桌面小组件服务
│   ├── router/
│   │   └── app_router.dart          # GoRouter 路由配置（含认证守卫）
│   ├── storage/                     # 本地存储（6 个文件）
│   │   ├── app_preferences.dart     # SharedPreferences 封装
│   │   ├── secure_storage.dart      # 安全存储（Token 缓存）
│   │   ├── lyric_cache_service.dart # 歌词本地缓存
│   │   ├── song_cache_service.dart  # 歌曲缓存服务
│   │   ├── playback_state_storage.dart  # 播放状态持久化
│   │   └── preference_sync_service.dart # 偏好同步服务
│   ├── theme/
│   │   ├── app_theme.dart           # Material 3 主题 + SongloftThemeExtension（含 Liquid Glass 令牌）
│   │   ├── app_dimensions.dart      # 尺寸和圆角常量
│   │   ├── responsive.dart          # 响应式断点和工具扩展
│   │   └── widgets/                 # 主题组件
│   │       ├── glass_surface.dart   # 玻璃效果容器
│   │       └── glass_capsule_bar.dart  # 玻璃胶囊导航栏
│   ├── tracely/
│   │   └── tracely_client.dart      # Tracely 前端监控上报客户端
│   ├── updater/                     # 应用更新（5 个文件）
│   │   ├── backend_patch_service.dart   # 后端热补丁服务
│   │   ├── patch_update_service.dart    # 补丁更新服务
│   │   ├── patch_update_dialog.dart     # 补丁更新对话框
│   │   ├── channel_release_resolver.dart  # 更新渠道解析
│   │   └── version_compare.dart         # 版本号比较
│   ├── utils/                       # 工具库（31 个文件）
│   │   ├── color_extraction.dart    # 封面颜色提取
│   │   ├── formatters.dart          # 格式化工具（时长、文件大小等）
│   │   ├── platform_utils.dart      # 平台检测工具
│   │   ├── url_helper.dart          # URL 构建辅助（base_url 拼接、token 附加等）
│   │   ├── audio_format_helper.dart # 音频格式辅助
│   │   ├── window_tray_manager.dart # 窗口托盘管理
│   │   └── ...                      # Web 平台相关辅助（缓存清理、全屏、WebGL 恢复等）
│   ├── plugin_iframe_gate.dart      # 插件 iframe 门控（条件导入入口）
│   ├── plugin_iframe_gate_web.dart  # Web 端 iframe 门控实现
│   └── plugin_iframe_gate_stub.dart # 非 Web 平台 stub
├── features/                        # 功能模块
│   ├── auth/                        # 认证模块
│   │   ├── data/
│   │   │   ├── auth_api.dart        # 认证 API
│   │   │   └── auth_repository.dart # 认证仓储
│   │   ├── domain/
│   │   │   └── auth_state.dart      # 认证状态定义
│   │   └── presentation/
│   │       ├── login_page.dart      # 登录页面（含「使用本地模式」按钮）
│   │       └── providers/
│   │           └── auth_provider.dart
│   ├── startup/                     # 启动流程模块
│   │   └── presentation/
│   │       ├── startup_gate.dart    # 启动守门：本地模式自动引导 / 远程模式探测服务器
│   │       └── web_update_gate.dart # Web 端强制更新门控
│   ├── home/                        # 首页模块
│   │   ├── domain/
│   │   │   └── home_grid_config.dart  # 首页网格布局配置
│   │   └── presentation/
│   │       ├── home_page.dart         # 首页（歌单轮播、统计条、JS 插件网格）
│   │       ├── plugin_host_bridge.dart     # 插件宿主桥接（原生 WebView callHandler 注册）
│   │       ├── plugin_host_dispatch.dart   # 插件宿主分发（传输无关、web-safe）
│   │       ├── plugin_webview_page.dart    # JS 插件 WebView 页面（条件导入）
│   │       ├── plugin_tab_page.dart        # 插件 Tab 页面（条件导入）
│   │       ├── providers/
│   │       │   └── home_grid_config_provider.dart
│   │       ├── render/              # 插件渲染引擎（WebView）
│   │       │   ├── plugin_render_view.dart
│   │       │   ├── plugin_render_controller.dart
│   │       │   ├── plugin_render_surface_webview.dart
│   │       │   └── ...              # 渲染辅助（配色等）
│   │       └── widgets/
│   │           ├── playlist_carousel.dart  # 歌单轮播组件
│   │           ├── hero_card.dart          # Hero 卡片
│   │           ├── stats_strip.dart        # 统计条
│   │           └── section_header.dart     # 区块标题
│   ├── jsplugin/                    # JS 插件模块
│   │   ├── data/
│   │   │   ├── jsplugin_api.dart    # JS 插件 API（含 JSPlugin 模型、上传、更新检查）
│   │   │   └── plugin_order.dart    # 插件排序
│   │   └── presentation/
│   │       ├── providers/
│   │       │   └── jsplugin_provider.dart
│   │       └── widgets/
│   │           ├── jsplugin_grid.dart      # JS 插件入口网格（首页用）
│   │           ├── jsplugin_manager.dart   # JS 插件管理面板（设置页用）
│   │           ├── plugin_registry.dart    # 插件仓库浏览
│   │           ├── plugin_icon.dart        # 插件图标
│   │           └── plugin_icon_utils.dart  # 图标辅助
│   ├── library/                     # 歌曲库模块（含分类 / 文件夹 / 标签浏览）
│   │   ├── data/
│   │   │   ├── songs_api.dart       # 歌曲 API
│   │   │   ├── songs_repository.dart
│   │   │   └── song_tags_api.dart   # 歌曲标签 API
│   │   ├── domain/
│   │   │   ├── repositories/
│   │   │   │   └── songs_repository_interface.dart
│   │   │   └── use_cases/
│   │   │       └── favorite_service.dart  # 收藏服务
│   │   └── presentation/
│   │       ├── library_page.dart          # 歌曲库页面（多视图切换：歌曲 / 歌单 / 分类 / 文件夹 / 标签）
│   │       ├── song_edit_page.dart        # 歌曲编辑页面
│   │       ├── category_songs_page.dart   # 分类歌曲列表页
│   │       ├── folder_content_page.dart   # 文件夹内容页
│   │       ├── tag_songs_page.dart        # 标签歌曲列表页
│   │       ├── providers/
│   │       │   ├── songs_provider.dart
│   │       │   ├── favorite_provider.dart
│   │       │   ├── category_provider.dart # 分类浏览 Provider
│   │       │   ├── folder_provider.dart   # 文件夹浏览 Provider
│   │       │   └── song_tag_provider.dart # 歌曲标签 Provider
│   │       └── widgets/
│   │           ├── song_list_tile.dart        # 歌曲列表项
│   │           ├── library_view_switcher.dart # 视图切换器
│   │           ├── facet_grid_view.dart       # 分面网格视图（专辑 / 歌手等）
│   │           ├── folder_browse_view.dart    # 文件夹浏览视图
│   │           └── tag_grid_view.dart         # 标签网格视图
│   ├── player/                      # 播放器模块
│   │   ├── data/
│   │   │   └── play_history_api.dart  # 播放历史 API
│   │   ├── domain/
│   │   │   ├── player_state.dart      # 播放器状态定义
│   │   │   ├── lyric_parser.dart      # LRC 歌词解析器
│   │   │   ├── playback_context.dart  # 播放上下文
│   │   │   ├── equalizer_setting.dart # 均衡器设置
│   │   │   └── use_cases/             # 领域用例（8 个）
│   │   │       ├── play_queue.dart        # 播放队列管理
│   │   │       ├── queue_loader.dart      # 队列加载器
│   │   │       ├── play_mode_resolver.dart  # 播放模式解析
│   │   │       ├── prefetch_strategy.dart   # 预取策略
│   │   │       ├── sleep_timer_logic.dart   # 睡眠定时器
│   │   │       └── ...                     # 重试策略、恢复状态等
│   │   └── presentation/
│   │       ├── queue_page.dart        # 播放队列页面
│   │       ├── lyric_adjust_page.dart # 歌词时间偏移调整页
│   │       ├── providers/             # Provider（10 个）
│   │       │   ├── player_provider.dart     # 核心播放 Provider
│   │       │   ├── lyric_provider.dart      # 歌词 Provider
│   │       │   ├── equalizer_provider.dart  # 均衡器 Provider
│   │       │   ├── audio_track_provider.dart  # 音轨切换 Provider
│   │       │   ├── play_history_provider.dart # 播放历史 Provider
│   │       │   └── ...                       # Web 视频同步等
│   │       ├── utils/
│   │       │   ├── full_player_route.dart   # 全屏播放器路由辅助
│   │       │   └── player_song_actions.dart # 播放器歌曲操作
│   │       └── widgets/               # 播放器组件（25 个）
│   │           ├── desktop_player.dart       # 桌面端播放器栏
│   │           ├── desktop_full_player.dart  # 桌面端全屏播放器
│   │           ├── mobile_player.dart        # 移动端全屏播放器
│   │           ├── mini_player.dart          # 迷你播放器条
│   │           ├── side_player.dart          # 侧边播放器面板
│   │           ├── play_controls.dart        # 播放控制按钮
│   │           ├── popup_controls.dart       # 弹出式控制面板
│   │           ├── progress_bar.dart         # 进度条
│   │           ├── volume_control.dart       # 音量控制
│   │           ├── lyrics_view.dart          # 歌词显示
│   │           ├── karaoke_line.dart         # 卡拉OK 逐字高亮
│   │           ├── playlist_drawer.dart      # 播放列表抽屉
│   │           ├── equalizer_panel.dart      # 均衡器面板
│   │           ├── video_player_surface.dart # 视频播放器画面
│   │           ├── play_history_sheet.dart   # 播放历史面板
│   │           ├── vinyl_ring.dart           # 黑胶唱片动画
│   │           └── ...                       # 音轨控制、视频舞台、快捷键域等
│   ├── playlist/                    # 歌单模块
│   │   ├── data/
│   │   │   ├── playlist_api.dart    # 歌单 CRUD
│   │   │   └── playlist_repository.dart
│   │   ├── domain/
│   │   │   ├── playlist.dart          # 歌单模型
│   │   │   ├── repositories/
│   │   │   │   └── playlist_repository_interface.dart
│   │   │   └── use_cases/
│   │   │       ├── playlist_sort.dart      # 歌单排序
│   │   │       └── pinyin_comparator.dart  # 拼音比较器
│   │   └── presentation/
│   │       ├── playlist_detail_page.dart  # 歌单详情页
│   │       ├── providers/
│   │       │   ├── playlist_provider.dart
│   │       │   └── playlist_view_provider.dart
│   │       └── widgets/               # 歌单组件（10 个）
│   │           ├── playlist_browse_view.dart     # 歌单浏览视图
│   │           ├── playlist_card.dart            # 歌单卡片
│   │           ├── playlist_list_item.dart       # 歌单列表项
│   │           ├── playlist_search_field.dart    # 歌单搜索框
│   │           ├── playlist_edit_dialog.dart     # 歌单编辑对话框
│   │           ├── playlist_form_dialog.dart     # 歌单表单对话框
│   │           ├── playlist_song_tile.dart       # 歌单内歌曲项
│   │           ├── song_cover_picker_modal.dart  # 歌曲封面选择弹窗
│   │           └── ...                          # 封面编辑、适配器等
│   ├── settings/                    # 设置模块
│   │   ├── data/                    # 数据层（12 个文件）
│   │   │   ├── cache_api.dart       # 音乐缓存 API
│   │   │   ├── config_api.dart      # 通用配置 API（/configs/{key}）
│   │   │   ├── settings_api.dart    # 业务设置 API（/settings/* 端点封装）
│   │   │   ├── directory_api.dart   # 目录浏览 API
│   │   │   ├── scan_api.dart        # 扫描 API
│   │   │   ├── theme_pack_api.dart  # 主题包 API（含 ThemePack / ThemePackColors 模型）
│   │   │   ├── upgrade_api.dart     # 升级 API
│   │   │   ├── frontend_version_api.dart  # 前端版本检查 API
│   │   │   ├── log_export_service.dart    # 日志导出服务
│   │   │   └── ...                        # 日志分享条件导入
│   │   ├── domain/
│   │   │   ├── key_binding.dart            # 快捷键绑定
│   │   │   └── player_shortcut_action.dart # 播放器快捷操作
│   │   └── presentation/
│   │       ├── settings_page.dart           # 设置页面（入口）
│   │       ├── servers_page.dart            # 服务器管理页面
│   │       ├── duplicate_check_page.dart    # 重复歌曲检查页面
│   │       ├── client_download_page.dart    # 客户端下载页面
│   │       ├── licenses_page.dart           # 开源许可证页面
│   │       ├── shortcut_settings_page.dart  # 快捷键设置页面
│   │       ├── providers/
│   │       │   ├── settings_provider.dart
│   │       │   ├── theme_pack_provider.dart
│   │       │   ├── shortcut_settings_provider.dart
│   │       │   ├── cache_download_provider.dart
│   │       │   └── song_cache_provider.dart
│   │       └── widgets/             # 设置组件（18 个）
│   │           ├── settings_master_detail.dart    # 主从布局
│   │           ├── settings_category_content.dart # 分类内容
│   │           ├── section_card.dart              # 区块卡片
│   │           ├── scan_manager.dart              # 扫描管理
│   │           ├── cache_manager.dart             # 缓存管理
│   │           ├── config_manager.dart            # 配置管理
│   │           ├── exclude_dir_manager.dart       # 排除目录管理
│   │           ├── theme_selector.dart            # 主题选择器
│   │           ├── theme_pack_manager.dart        # 主题包管理
│   │           ├── theme_catalog.dart             # 在线主题目录
│   │           ├── token_manager.dart             # 令牌管理
│   │           ├── language_selector.dart         # 语言选择器
│   │           ├── home_grid_selector.dart        # 首页布局选择器
│   │           ├── upgrade_dialog.dart            # 后端升级对话框
│   │           ├── frontend_upgrade_dialog.dart   # 前端升级对话框
│   │           ├── github_proxy_dialog.dart       # GitHub 代理对话框
│   │           ├── metadata_refresh_manager.dart  # 元数据刷新管理
│   │           └── shortcut_recorder.dart         # 快捷键录制器
│   ├── desktop_lyric/               # 桌面歌词模块
│   │   ├── desktop_lyric_controller.dart    # 桌面歌词控制器
│   │   ├── desktop_lyric_main.dart          # 桌面歌词入口
│   │   ├── desktop_lyric_ipc.dart           # 桌面歌词进程间通信
│   │   ├── desktop_lyric_font_size.dart     # 字号配置
│   │   ├── android_floating_lyric_controller.dart  # Android 悬浮歌词控制器
│   │   └── presentation/
│   │       ├── desktop_lyric_app.dart       # 桌面歌词窗口应用
│   │       └── desktop_lyric_view.dart      # 桌面歌词视图
│   └── dlna/                        # DLNA 投屏模块
│       ├── data/
│       │   └── dlna_service.dart    # DLNA/UPnP 设备发现与投屏服务
│       ├── domain/
│       │   └── dlna_state.dart      # 投屏状态定义
│       └── presentation/
│           ├── providers/
│           │   └── dlna_provider.dart
│           └── widgets/
│               ├── cast_button.dart       # 投屏按钮
│               └── device_sheet.dart      # 设备选择面板
├── l10n/                            # 国际化
│   ├── app_en.arb                   # 英文
│   ├── app_zh.arb                   # 中文
│   ├── app_es.arb                   # 西班牙文
│   ├── app_localizations.dart       # 生成的本地化类
│   └── l10n_holder.dart             # 全局 l10n 访问器
├── shared/                          # 共享模块
│   ├── constants/
│   │   └── github_proxy.dart        # GitHub 代理常量
│   ├── layouts/
│   │   ├── shell_layout.dart        # ShellRoute 主布局（导航 + 播放器）
│   │   ├── adaptive_scaffold.dart   # 自适应脚手架
│   │   └── active_destinations.dart # 动态导航目的地
│   ├── mixins/
│   │   └── song_list_actions.dart   # 歌曲列表操作 Mixin
│   ├── models/
│   │   ├── song.dart                # 歌曲模型
│   │   ├── artist.dart              # 艺术家模型
│   │   ├── library_stats.dart       # 曲库统计模型
│   │   ├── pagination.dart          # 分页模型
│   │   └── api_response.dart        # API 响应模型
│   ├── utils/
│   │   └── responsive_snackbar.dart # 响应式 SnackBar
│   └── widgets/                     # 共享组件（22 个）
│       ├── cover_image.dart         # 封面图片组件
│       ├── network_cover_image.dart # 网络封面图片
│       ├── favorite_button.dart     # 收藏按钮
│       ├── scrolling_text.dart      # 滚动文本
│       ├── confirm_dialog.dart      # 确认对话框
│       ├── delete_song_dialog.dart  # 删除歌曲对话框
│       ├── add_to_playlist_modal.dart  # 添加到歌单弹窗
│       ├── song_picker_modal.dart   # 歌曲选择弹窗
│       ├── manage_tags_modal.dart   # 标签管理弹窗
│       ├── browse_card.dart         # 浏览卡片
│       ├── browse_collection_view.dart  # 浏览集合视图
│       ├── entity_detail_scaffold.dart  # 实体详情脚手架
│       ├── song_tile.dart           # 歌曲磁贴
│       ├── filter_pill.dart         # 筛选胶囊
│       ├── directory_picker_sheet.dart  # 目录选择面板
│       ├── directory_tree_selector.dart # 目录树选择器
│       ├── draggable_scrollbar_overlay.dart  # 可拖拽滚动条
│       ├── scroll_to_top_fab.dart   # 回到顶部按钮
│       ├── selection_action_button.dart  # 多选操作按钮
│       ├── empty_state.dart         # 空状态
│       ├── error_view.dart          # 错误视图
│       └── loading_indicator.dart   # 加载指示器
└── main.dart                        # 应用入口
```

## 页面结构

### 路由配置

| 页面 | 路由 | 说明 |
|------|------|------|
| 登录 | `/login` | 登录页面（独立路由，不使用 ShellRoute） |
| 首页 | `/` | 歌单轮播、统计条、JS 插件网格 |
| 曲库 | `/library` | 多视图：歌曲 / 歌单 / 分类 / 文件夹 / 标签 |
| 分类歌曲 | `/library/categories/:field?value=` | 按专辑、歌手等分类查看歌曲列表 |
| 文件夹 | `/library/folders?path=` | 文件夹内容下钻页 |
| 标签歌曲 | `/library/tags/:tagId?name=` | 标签下的歌曲列表 |
| 歌单列表 | `/playlists` | **已合并至曲库**，访问时重定向到 `/library` |
| 歌单详情 | `/playlists/:id` | 歌单详情和歌曲列表 |
| 设置 | `/settings` | 主从布局，按分类展示（外观、扫描、缓存、网络等） |
| 设置分类 | `/settings/category/:index` | 设置分类详情（移动端二级页） |
| 服务器管理 | `/settings/servers` | 多服务器管理 |
| 重复检查 | `/settings/duplicate-check` | 重复歌曲检测 |
| 快捷键 | `/settings/shortcuts` | 键盘快捷键设置（桌面端） |
| 客户端下载 | `/settings/download` | 客户端下载（Web 端可见） |
| 插件商店 | `/settings/plugin-registry` | 插件仓库浏览与安装 |
| 许可证 | `/settings/licenses` | 开源许可证 |
| 插件页 | `/plugin?url=&name=` | JS 插件 WebView 页面（全屏，独立路由） |
| 插件 Tab | `/plugin-tab/:entryPath` | 插件 Tab 页面（由 ShellLayout 管理） |
| 全屏播放器 | `/player` | 全屏播放器（独立路由，按屏幕类型分派 Mobile/Desktop） |

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
| **Tablet** | 600 - 900px | 底部导航 + 播放器（更宽） |
| **Desktop** | 900px+ | 侧边导航 + 播放器 |

### 布局架构

```
ShellLayout (ShellRoute builder)
├── AdaptiveScaffold
│   ├── Mobile/Tablet: NavigationBar (底部) + MiniPlayer
│   └── Desktop: NavigationRail (侧边) + DesktopPlayer (底部)
└── 内容区域 (GoRouter child)
```

### 播放器形态（`navigationStyle`）

底部播放器有**两种形态**，由主题扩展 `SongloftThemeExtension.navigationStyle` 决定，与断点正交：

| 形态 | 手机 | 平板 / 桌面 |
|------|------|-------------|
| **`capsule`**（胶囊 / 玻璃主题） | 浮起胶囊条：全圆角 pill、精简控制区，**无真模糊**（半透填充 + 内高光） | **浮起玻璃胶囊条**：pill 高 64、半径 = 高度 / 2、真毛玻璃（`GlassSurface` + `BackdropFilter`，sigma 20）、顶边 3px 圆角进度 + 上方 16px 可点按 / 拖拽热区、常驻「播放模式 · 音量 · 歌词 · 队列 · 更多」 |
| **standard**（默认） | 底部迷你播放器（2px 进度 + 64px 主体，占 `bottomNavigationBar` 槽位） | 占布局高度的底栏：平板为 `MiniPlayer`，桌面为 90px `DesktopPlayer`（含 `border-top`） |

胶囊形态在平板 / 桌面上**不是** `Column` 里的兄弟节点，而是由 `AdaptiveScaffold._overlayBottomPlayer` 用 `Stack` **浮在内容之上**，横跨「内容列 + 播放列表抽屉」整宽；桌面胶囊条同样不覆盖左侧 240px 毛玻璃侧栏。播放列表抽屉本身不感知胶囊条，其表面仍贯通到屏幕底部，抽屉下沿会被浮起的胶囊条遮住一条。因为播放器浮起后不占布局高度，滚动内容改由 `ResponsiveContext.navScrollInset` 自行预留底部空间（大屏胶囊档 `bottom + 84 = 64 + 12 + 8`，手机档仍为 `bottom + 16`）。

尺寸常量集中在 `lib/core/theme/app_dimensions.dart` 的 `AppCapsulePlayer`（手机高 59 = 11 + 48，大屏高 64 = 16 + 48；pill 半径由高度推导，不引入新圆角 token）。内容行（封面 / 标题 / 控制区）在胶囊内**垂直居中**：整条 = 顶部进度热区 + 内容行，热区高度与底部呼吸空间相等，因此封面与按钮在胶囊上下留白等分；顶边进度条是独立覆盖层，仍整宽贴着胶囊顶边框。胶囊形态的唯一实现是 `CapsuleMiniPlayer`（`lib/features/player/presentation/widgets/capsule_mini_player.dart`），手机档（`CapsuleMiniPlayer.compact`）与大屏档共用同一套骨架，只差尺寸、是否真模糊与控制区密度。`widescreen` 的 `WidescreenSidePlayer` 不在此列。

## 主题系统

### Material 3 配色

- **主色调**: M3 Blue baseline (`#415F91`)
- **配色方案**: `ColorScheme.fromSeed(seedColor: Color(0xFF415F91))`
- **主题模式**: 亮色 / 暗色 / 跟随系统
- **字体回退**: NotoSansSC（中文）/ NotoSansKR（韩文）

### 响应式主题

主题根据屏幕类型动态调整组件尺寸：
- **SnackBar**: Desktop 使用固定宽度居中显示
- **对话框**: 根据屏幕类型调整最大宽度

## UI 组件设计规范

### 播放按钮形状（PlayControls）

核心播放控制组件 `PlayControls`（`lib/features/player/presentation/widgets/play_controls.dart`）通过 `useRoundedRect` 参数控制按钮形状：

```dart
final borderRadius =
    useRoundedRect ? AppRadius.xxlAll : BorderRadius.circular(size);
```

- **圆形**（`useRoundedRect = false`，默认）：`BorderRadius.circular(size)` 在正方形容器上产生正圆
- **大圆角矩形**（`useRoundedRect = true`）：固定 28px 圆角（`AppRadius.xxl`），呈超椭圆/胶囊形

**设计原则：按钮尺寸决定形状**

| 平台 / 场景 | 尺寸 | 形状 | 原因 |
|-------------|------|------|------|
| 桌面全屏播放器 | 52px | 圆形 | 尺寸适中，圆形视觉比例协调 |
| 桌面底部播放栏 | 40px | 圆形 | 小尺寸，圆形紧凑 |
| 车机侧边面板 | 52px | 圆形 | 同桌面 |
| **移动端全屏播放器** | 76px | 大圆角矩形 | 大尺寸圆形会显得笨重，圆角矩形更现代紧凑 |
| **视频播放器** | 60px | 大圆角矩形 | 与视频画面比例协调，避免圆形过于突兀 |

**规则**：60px 及以上的主播放按钮使用 `useRoundedRect: true`；52px 及以下使用默认圆形。这不是 bug，是有意为之的视觉平衡决策。新增播放界面时按此规则选择形状。

### 其他播放按钮变体

- **CompactPlayButton**：纯图标按钮（`IconButton`），无背景填充，用于迷你播放栏等空间紧凑场景
- **Browse Card FAB**：`Material(shape: CircleBorder())` 悬浮圆形按钮，带 elevation，用于网格卡片悬停时的快速播放
- **"全部播放" FilledButton**：`FilledButton.icon` 药丸形按钮，含文字标签，用于歌单/分类页头部

<a id="deployment-modes"></a>

## 部署模式

### 嵌入模式（Embedded）

```bash
flutter build web --dart-define=DEPLOY_MODE=embedded
```

- 显式选择旧 Flutter Web 嵌入版时，与 Go 后端同域访问
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

### Bundle 本地模式

```bash
# 编译时启用（配合 Go 后端原生库 / 可执行文件）
flutter build apk --dart-define=HAS_BACKEND=true     # Android
flutter build ios --dart-define=HAS_BACKEND=true      # iOS
flutter build macos --dart-define=HAS_BACKEND=true    # macOS
flutter build linux --dart-define=HAS_BACKEND=true    # Linux
flutter build windows --dart-define=HAS_BACKEND=true  # Windows
```

- Go 后端嵌入客户端，无需单独部署服务器
- `AppConfig.hasEmbeddedBackend` 编译时常量控制是否显示「使用本地模式」入口
- 支持 `local`（本地）和 `remote`（远程）两种运行模式，持久化到 SharedPreferences
- **移动端**：Go 后端通过 gomobile 编译为 `.aar`（Android）/ `.xcframework`（iOS），Flutter 通过 `MethodChannel('com.songloft/backend')` 调用 `Start/Stop/IsRunning/GetPort`
- **桌面端**：Go 后端编译为 `songloft-server` 可执行文件，Flutter 启动时作为子进程运行，通过 stdout 解析监听端口
- **Web**：不支持 Bundle 模式
- 本地模式启动流程：申请存储权限 → 启动嵌入后端 `127.0.0.1:<port>` → 健康检查轮询（最多 10 次 × 300ms）→ 自动 `admin/admin` 登录
- `BackendLifecycle`（WidgetsBindingObserver）监听 App 生命周期，前台恢复时自动重启后端
- **Android 路径**：Bundle 模式 CWD 是 `/`，covers 等目录必须相对 `DBPath` 解析，不能按 CWD 解析
- **移动端临时目录**：`mobile.Start` 在服务初始化前创建 `<dataDir>/tmp` 并设置 Go 进程的 `TMPDIR`，供直播转码、下载和封面等临时文件使用；Android 默认 `/data/local/tmp` 对普通应用不可写。`Stop` 保留该环境配置，HLS 服务关闭时清理自己的会话目录。
- **原生桥接**：Android 用 `Class.forName("mobile.Mobile")` 反射调用 gomobile 生成类，未打包 `.aar` 时 `isAvailable()` 返回 false（优雅降级）；iOS 用 Swift 调用 `MobileStart` 等 Objective-C 函数
- **桌面子进程**：`DesktopBackendService` 在 Flutter 可执行文件同目录查找 `songloft-server`（macOS 为 `Contents/Resources/`），并从 stdout 解析实际监听端口
- **CI 产物**：统一以 `songloft-bundled-` 开头；Android/Linux/Windows 带架构，Apple 为 `songloft-bundled-macos.zip`、`songloft-bundled-ios-nosign.ipa`。4 个并行 Job（Android/Linux/Apple/Windows）失败不阻塞主 Release

## 音频播放架构

```
SongloftAudioHandler (extends BaseAudioHandler)
├── just_audio (核心播放引擎)
│   ├── Web: HTML5 Audio + hls.js (自定义 SongloftWebJustAudioPlugin)
│   └── Win/Linux/macOS/Android/iOS: media_kit (libmpv)，所有原生平台统一，无回退
├── audio_service (系统通知栏/锁屏控制)
└── audio_session (音频焦点管理)
```

<a id="platform-adaptation"></a>

### 平台适配

- **Android**: 前台服务持续运行（`androidStopForegroundOnPause: false`），兼容 HyperOS3 等激进回收策略
- **Android 13+**: 运行时请求通知权限
- **macOS**: secure_storage 未签名时自动降级到 SharedPreferences
- **音频后端**: 所有原生平台（Win/Linux/macOS/Android/iOS）统一用 `just_audio_media_kit`（libmpv），支持应用内视频画面，无回退（已移除 kill-switch，不再支持回退 AVPlayer/ExoPlayer）

## 开发命令

```bash
cd clients/player
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
# Web 嵌入模式（输出至 clients/player-build/web-embedded，供 Go 二进制 //go:embed）
make build-frontend-flutter-web-embedded

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

# Bundle 本地模式（先编译 Go 后端，再构建 Flutter 客户端）
# 1. 编译 Go 后端为移动端库 / 桌面端可执行文件
make build-go-mobile-android       # → clients/player/android/app/libs/songloft.aar
make build-go-mobile-ios           # → clients/player/ios/Songloft.xcframework（仅 macOS）
make build-go-desktop-linux        # → clients/player/linux/songloft-server
make build-go-desktop-windows      # → clients/player/windows/songloft-server.exe
make build-go-desktop-macos-arm64  # → clients/player/macos/Runner/songloft-server

# 2. 构建 Flutter 客户端（需加 --dart-define=HAS_BACKEND=true）
# CI 中由 release.yml 的 build-bundled-{android,linux,apple,windows} Job 自动完成
```

预编译安装包下载:
- 标准版（需连接服务器）: [https://github.com/songloft-org/songloft-player/releases](https://github.com/songloft-org/songloft-player/releases)
- Bundle 版（内嵌后端）: [https://github.com/songloft-org/songloft/releases](https://github.com/songloft-org/songloft/releases)（`songloft-bundled-*` 文件）
