# Songloft Frontend Architecture

> **Standalone repository**: [https://github.com/songloft-org/songloft-player](https://github.com/songloft-org/songloft-player)

The Songloft frontend is a Flutter-based cross-platform music player supporting six platforms: **Android, iOS, macOS, Windows, Linux, and Web**. The Flutter Web build output can be embedded into the Go backend binary and shipped together. It also supports **Bundle local mode**: embedding the Go backend into the client (as a native library via gomobile on mobile, and as a subprocess on desktop), so users can play local music without deploying a separate server.

## Tech Stack

- **Framework**: Flutter 3.29+ / Dart 3.7+
- **State management**: flutter_riverpod ^3.1.0 (hand-written Providers, no code generation)
- **Routing**: go_router ^17.1.0 (declarative routing + ShellRoute)
- **HTTP client**: dio ^5.7.0
- **Audio playback**: just_audio ^0.10.5 + audio_service ^0.18.17
- **Audio backend**: Win/Linux/macOS/Android/iOS default to just_audio_media_kit (libmpv, with a compile-time fallback to native); Web uses just_audio_web + self-integrated hls.js
- **Video picture**: media_kit_video (native platforms derive a VideoController from the same libmpv Player; Web uses a muted `<video>` synced to playback, songloft-org/songloft#76)
- **Local storage**: shared_preferences ^2.3.4
- **Image caching**: cached_network_image ^3.4.1
- **Color extraction**: palette_generator ^0.3.3+4
- **WebView**: flutter_inappwebview ^6.1.5 (loading JS plugin pages)
- **Permission management**: permission_handler ^12.0.1
- **UI framework**: Material 3 (seedColor: M3 Blue baseline `#415F91`)

## Design Philosophy

- **Music playback at the core**: the player is always visible and controllable at any time
- **Responsive four-way adaptation**: adaptive layouts for Mobile / Tablet / Desktop / TV
- **Feature-First architecture**: code organized by feature module, each with three layers — data / domain / presentation
- **Consistent cross-platform experience**: a single codebase adapts to six platforms, with optimizations tailored to each platform's characteristics

## Directory Structure

```
songloft-player/lib/
├── config/                          # App configuration
│   ├── app_config.dart              # API config, deployment mode, version number
│   └── constants.dart               # App constants
├── core/                            # Core infrastructure
│   ├── audio/
│   │   ├── audio_service.dart       # SongloftAudioHandler (audio playback, notification bar controls)
│   │   └── system_volume_provider.dart  # System volume Provider (based on volume_controller)
│   ├── backend/                     # Bundle local mode (embedded backend abstraction layer)
│   │   ├── embedded_backend_service.dart   # Unified interface (mobile MethodChannel / desktop subprocess dispatch)
│   │   ├── desktop_backend_service.dart    # Desktop: start the songloft-server subprocess
│   │   ├── run_mode_provider.dart          # RunMode enum (local/remote) + persistence Provider
│   │   └── backend_lifecycle.dart          # WidgetsBindingObserver: auto-restart backend on foreground resume
│   ├── env/
│   │   └── tv_detector.dart          # TV device detection
│   ├── platform/
│   │   └── live_activity_service.dart  # iOS Dynamic Island / Live Activity integration
│   ├── tracely/
│   │   └── tracely_client.dart       # Tracely frontend monitoring-report client
│   ├── network/
│   │   ├── api_client.dart          # Dio HTTP client wrapper
│   │   ├── api_exceptions.dart      # API exception definitions
│   │   └── auth_interceptor.dart    # JWT Token auto-refresh interceptor
│   ├── router/
│   │   └── app_router.dart          # GoRouter route configuration (with auth guard)
│   ├── storage/
│   │   ├── app_preferences.dart     # SharedPreferences wrapper
│   │   ├── lyric_cache_service.dart # Local lyric caching
│   │   └── secure_storage.dart      # Secure storage (Token caching)
│   ├── theme/
│   │   ├── app_theme.dart           # Material 3 theme (light/dark, responsive)
│   │   ├── app_dimensions.dart      # Size and border-radius constants
│   │   ├── responsive.dart          # Responsive breakpoints and utility extensions
│   │   └── tv_theme.dart            # TV-specific theme constants
│   └── utils/
│       ├── color_extraction.dart    # Cover color extraction
│       ├── formatters.dart          # Formatting utilities (duration, file size, etc.)
│       ├── platform_utils.dart      # Platform detection utilities
│       └── url_helper.dart          # URL building helpers (base_url concatenation, token appending, etc.)
├── features/                        # Feature modules
│   ├── auth/                        # Authentication module
│   │   ├── data/
│   │   │   ├── auth_api.dart        # Authentication API
│   │   │   └── auth_repository.dart # Authentication repository
│   │   ├── domain/
│   │   │   └── auth_state.dart      # Authentication state definitions
│   │   └── presentation/
│   │       ├── login_page.dart      # Login page (with "Use local mode" button)
│   │       └── providers/
│   │           └── auth_provider.dart
│   ├── startup/                     # Startup flow module
│   │   └── presentation/
│   │       └── startup_gate.dart    # Startup gate: local mode auto-bootstrap / remote mode server probe
│   ├── home/                        # Home module
│   │   └── presentation/
│   │       ├── home_page.dart       # Home page (playlist carousel, JS plugin grid)
│   │       ├── plugin_webview_page.dart      # JS plugin WebView page (conditional import)
│   │       ├── plugin_webview_page_native.dart  # Native platform WebView implementation
│   │       ├── plugin_webview_page_stub.dart    # Web platform stub
│   │       └── widgets/
│   │           └── playlist_carousel.dart   # Playlist carousel component
│   ├── jsplugin/                    # JS plugin module
│   │   ├── data/
│   │   │   └── jsplugin_api.dart    # JS plugin API (with JSPlugin model, upload, update check)
│   │   └── presentation/
│   │       ├── providers/
│   │       │   └── jsplugin_provider.dart   # JSPluginApi Provider / jsPluginsProvider
│   │       └── widgets/
│   │           ├── jsplugin_grid.dart       # JS plugin entry grid (used on home page)
│   │           └── jsplugin_manager.dart    # JS plugin management panel (used on settings page)
│   ├── library/                     # Song library module
│   │   ├── data/
│   │   │   ├── songs_api.dart       # Song API
│   │   │   └── songs_repository.dart
│   │   └── presentation/
│   │       ├── library_page.dart    # Song library page
│   │       ├── song_edit_page.dart  # Song edit page
│   │       ├── providers/
│   │       │   ├── songs_provider.dart
│   │       │   └── favorite_provider.dart
│   │       └── widgets/
│   │           ├── song_list_tile.dart   # Song list item
│   │           └── song_filter_bar.dart  # Song filter bar
│   ├── player/                      # Player module
│   │   ├── domain/
│   │   │   ├── player_state.dart    # Player state definitions
│   │   │   └── lyric_parser.dart    # LRC lyric parser
│   │   └── presentation/
│   │       ├── queue_page.dart      # Play queue page
│   │       ├── providers/
│   │       │   └── player_provider.dart
│   │       └── widgets/
│   │           ├── desktop_player.dart    # Desktop player (mini/sidebar form)
│   │           ├── desktop_full_player.dart  # Desktop fullscreen player
│   │           ├── mobile_player.dart     # Mobile fullscreen player
│   │           ├── tv_player.dart         # TV player
│   │           ├── mini_player.dart       # Mini player bar
│   │           ├── play_controls.dart     # Playback control buttons
│   │           ├── popup_controls.dart    # Popup control panel
│   │           ├── progress_bar.dart      # Progress bar
│   │           ├── volume_control.dart    # Volume control
│   │           ├── lyrics_view.dart       # Lyrics display
│   │           └── playlist_drawer.dart   # Playlist drawer
│   ├── playlist/                    # Playlist module
│   │   ├── data/
│   │   │   ├── playlist_api.dart    # Playlist CRUD
│   │   │   └── playlist_repository.dart
│   │   ├── domain/
│   │   │   └── playlist.dart        # Playlist model
│   │   └── presentation/
│   │       ├── playlists_page.dart   # Playlist list page
│   │       ├── playlist_detail_page.dart  # Playlist detail page
│   │       ├── providers/
│   │       │   ├── playlist_provider.dart
│   │       │   └── playlist_view_provider.dart
│   │       └── widgets/
│   │           ├── playlist_card.dart         # Playlist card
│   │           ├── playlist_list_item.dart     # Playlist list item
│   │           └── song_cover_picker_modal.dart  # Song cover picker modal
│   ├── settings/                    # Settings module
│       ├── data/
│       │   ├── cache_api.dart       # Music cache API (stats, cleanup, config, directory validation)
│       │   ├── config_api.dart      # Config API
│       │   ├── directory_api.dart   # Directory browsing API (used by the music directory picker)
│       │   ├── frontend_version_api.dart  # Frontend version check API
│       │   ├── scan_api.dart        # Scan API
│       │   └── upgrade_api.dart     # Upgrade API
│       └── presentation/
│           ├── settings_page.dart   # Settings page
│           ├── providers/
│           │   └── settings_provider.dart
│           └── widgets/
│               ├── cache_manager.dart        # Music cache management panel (with custom cache directory dialog)
│               ├── config_manager.dart       # Config management
│               ├── exclude_dir_manager.dart  # Scan exclude directory management
│               ├── frontend_upgrade_dialog.dart  # Frontend upgrade dialog
│               ├── scan_manager.dart         # Scan management
│               ├── theme_selector.dart       # Theme selector
│               ├── token_manager.dart        # Token management
│               └── upgrade_dialog.dart       # Backend upgrade dialog
│   └── dlna/                        # DLNA casting module
│       ├── data/
│       │   └── dlna_service.dart    # DLNA/UPnP device discovery and casting service
│       ├── domain/
│       │   └── dlna_state.dart      # Casting state definitions
│       └── presentation/
│           ├── providers/
│           │   └── dlna_provider.dart
│           └── widgets/
│               ├── cast_button.dart       # Cast button
│               └── device_sheet.dart      # Device picker sheet
└── shared/                          # Shared modules
    ├── layouts/
    │   ├── shell_layout.dart        # ShellRoute main layout (navigation + player)
    │   └── adaptive_scaffold.dart   # Adaptive scaffold
    ├── models/
    │   ├── song.dart                # Song model
    │   ├── pagination.dart          # Pagination model
    │   └── api_response.dart        # API response model
    ├── utils/
    │   └── responsive_snackbar.dart # Responsive SnackBar
    └── widgets/                     # Shared components (11 total)
        ├── cover_image.dart         # Cover image component
        ├── favorite_button.dart     # Favorite button
        ├── scrolling_text.dart      # Scrolling text
        ├── confirm_dialog.dart      # Confirmation dialog
        ├── add_to_playlist_modal.dart  # Add-to-playlist modal
        ├── song_picker_modal.dart   # Song picker modal
        ├── empty_state.dart         # Empty state
        ├── error_view.dart          # Error view
        ├── loading_indicator.dart   # Loading indicator
        ├── tv_focusable.dart        # TV focusable component
        └── tv_grid_view.dart        # TV grid view
```

## Page Structure

### Route Configuration

| Page | Route | Description |
|------|------|------|
| Login | `/login` | Login page (standalone route, does not use ShellRoute) |
| Home | `/` | Playlist carousel, JS plugin grid |
| Library | `/library` | List of all songs, search, filtering |
| Playlists | `/playlists` | Playlist list |
| Playlist detail | `/playlists/:id` | Playlist details and song list |
| Settings | `/settings` | Theme, scanning, JS plugins, tokens, upgrade, about |
| Plugin | `/plugin?url=&name=` | JS plugin WebView page (fullscreen, standalone route) |

### Authentication Guard

Routing implements an authentication guard using GoRouter's `redirect` mechanism:
- Unauthenticated → redirect to `/login`
- Authenticated and on the login page → redirect to `/`
- Authentication state undetermined (Token is being restored) → no redirect

## Responsive Layout

### Breakpoint Definitions

| Screen type | Width range | Description |
|---------|---------|------|
| **Mobile** | < 600px | Bottom navigation + mini player |
| **Tablet** | 600 - 900px | Bottom navigation + mini player (wider) |
| **Desktop** | 900 - 1920px | Side navigation + bottom player bar |
| **TV** | ≥ 1920px | Focus navigation + large-scale UI + D-pad support |

### Layout Architecture

```
ShellLayout (ShellRoute builder)
├── AdaptiveScaffold
│   ├── Mobile/Tablet: NavigationBar (bottom) + MiniPlayer
│   ├── Desktop: NavigationRail (side) + DesktopPlayer (bottom)
│   └── TV: top Tab navigation + TvPlayer
└── Content area (GoRouter child)
```

## Theme System

### Material 3 Color Scheme

- **Primary color**: M3 Blue baseline (`#415F91`)
- **Color scheme**: `ColorScheme.fromSeed(seedColor: Color(0xFF415F91))`
- **Theme mode**: light / dark / follow system
- **Font fallback**: NotoSansSC (Chinese support)

### Responsive Theme

The theme dynamically adjusts component sizes based on screen type:
- **SnackBar**: fixed width, centered on Desktop/TV
- **FilledButton**: larger minimum size on TV
- **Dialogs**: maximum width adjusted by screen type

### TV-Specific Theme

The `TvTheme` class defines size constants for TV:
- Font sizes: title 24sp, body 20sp, subtitle 16sp
- Focus effect: 3px border + 1.05x scale
- Grid layout: 4 columns, 24px spacing, 48px padding

## Deployment Modes

### Embedded Mode

```bash
flutter build web --dart-define=DEPLOY_MODE=embedded
```

- Flutter Web is embedded into the Go backend, accessed from the same origin
- `AppConfig.baseUrl` is automatically set to `Uri.base.origin`
- **Hides** the API address input on the login page and the API configuration on the settings page
- `AppConfig.isEmbedded` is a compile-time constant; tree-shaking removes the API address UI code

### Standalone Deployment Mode (default)

```bash
flutter build web --dart-define=DEPLOY_MODE=standalone
```

- Frontend and backend deployed separately
- **Shows** the API address configuration UI, allowing users to manually enter the backend address
- The API address is persisted to local storage

### Bundle Local Mode

```bash
# Enabled at compile time (paired with the Go backend native library / executable)
flutter build apk --dart-define=HAS_BACKEND=true     # Android
flutter build ios --dart-define=HAS_BACKEND=true      # iOS
flutter build macos --dart-define=HAS_BACKEND=true    # macOS
flutter build linux --dart-define=HAS_BACKEND=true    # Linux
flutter build windows --dart-define=HAS_BACKEND=true  # Windows
```

- The Go backend is embedded into the client, no separate server deployment needed
- The `AppConfig.hasEmbeddedBackend` compile-time constant controls whether the "Use local mode" entry is shown
- Supports two run modes, `local` and `remote`, persisted to SharedPreferences
- **Mobile**: the Go backend is compiled via gomobile into `.aar` (Android) / `.xcframework` (iOS); Flutter calls `Start/Stop/IsRunning/GetPort` through `MethodChannel('com.songloft/backend')`
- **Desktop**: the Go backend is compiled into a `songloft-server` executable; Flutter runs it as a subprocess at startup and parses the listening port from stdout
- **Web**: Bundle mode is not supported
- Local mode startup flow: request storage permission → start the embedded backend at `127.0.0.1:<port>` → poll the health check → auto login with `admin/admin`
- `BackendLifecycle` (WidgetsBindingObserver) listens to the app lifecycle and automatically restarts the backend when the app resumes to the foreground

## Audio Playback Architecture

```
SongloftAudioHandler (extends BaseAudioHandler)
├── just_audio (core playback engine)
│   ├── Web: HTML5 Audio + hls.js (custom SongloftWebJustAudioPlugin)
│   └── Win/Linux/macOS/Android/iOS: media_kit (libmpv), macOS/mobile can fall back to native
├── audio_service (system notification bar / lock screen controls)
└── audio_session (audio focus management)
```

### Platform Adaptation

- **Android**: foreground service runs continuously (`androidStopForegroundOnPause: false`), compatible with aggressive reclamation strategies such as HyperOS3
- **Android 13+**: requests notification permission at runtime
- **macOS**: secure_storage automatically falls back to SharedPreferences when unsigned
- **Audio backend**: Win/Linux/macOS/Android/iOS default to `just_audio_media_kit` (libmpv), enabling in-app video; macOS/mobile can fall back to native (AVPlayer/ExoPlayer) via `--dart-define=SONGLOFT_MEDIAKIT_MACOS/MOBILE=false`

## Development Commands

```bash
cd songloft-player
flutter pub get                    # Install dependencies
flutter run -d chrome              # Web debugging (standalone mode)
flutter run -d chrome --dart-define=DEPLOY_MODE=embedded  # Simulate embedded mode
flutter run -d macos               # macOS debugging
flutter run -d windows             # Windows debugging
flutter run -d linux               # Linux debugging
flutter analyze                    # Static analysis
flutter test                       # Run tests
```

### Build Commands

```bash
# Web embedded mode (output to songloft-player-build/web-embedded, for Go binary //go:embed)
make build-frontend-web-embedded

# Web standalone deployment build
make build-frontend-web

# Desktop builds
make build-frontend-linux
make build-frontend-windows
make build-frontend-macos

# Android build (APK + AAB)
make build-frontend-android

# iOS build (macOS only)
make build-frontend-ios

# All platforms supported by the current system
make build-frontend-all

# Bundle local mode (compile the Go backend first, then build the Flutter client)
# 1. Compile the Go backend into a mobile library / desktop executable
make build-go-mobile-android       # → songloft-player/android/app/libs/songloft.aar
make build-go-mobile-ios           # → songloft-player/ios/Songloft.xcframework (macOS only)
make build-go-desktop-linux        # → songloft-player/linux/songloft-server
make build-go-desktop-windows      # → songloft-player/windows/songloft-server.exe
make build-go-desktop-macos-arm64  # → songloft-player/macos/Runner/songloft-server

# 2. Build the Flutter client (add --dart-define=HAS_BACKEND=true)
# In CI this is done automatically by release.yml's build-bundled-{android,linux,apple,windows} Jobs
```

Prebuilt installer downloads:
- Standard edition (requires connecting to a server): [https://github.com/songloft-org/songloft-player/releases](https://github.com/songloft-org/songloft-player/releases)
- Bundle edition (backend embedded): [https://github.com/songloft-org/songloft/releases](https://github.com/songloft-org/songloft/releases) (`songloft-bundled-*` files)
