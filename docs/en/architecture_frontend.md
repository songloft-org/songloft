# Songloft Flutter Frontend Architecture

> **Standalone repository**: [https://github.com/songloft-org/songloft-player](https://github.com/songloft-org/songloft-player)

This page describes Flutter. The ReactLynx + TypeScript preview client supports Android/iOS/HarmonyOS/Web; see [client selection](clients.md) and [Lynx guides](player-lynx/index.md). It has no desktop or bundled local backend and does not use the Flutter layout or build commands below.

The Songloft Flutter client supports six platforms: **Android, iOS, macOS, Windows, Linux, and Web**. The full server defaults to the separate Lynx Web client (`clients/player-lynx`); legacy embedded Flutter Web remains an explicit option. Flutter also supports **Bundle local mode**: embedding the Go backend into the client (as a native library via gomobile on mobile, and as a subprocess on desktop), so users can play local music without deploying a separate server.

## Tech Stack

- **Framework**: Flutter 3.41+ / Dart 3.7+
- **State management**: flutter_riverpod ^3.1.0 (hand-written Providers, no code generation)
- **Routing**: go_router ^17.1.0 (declarative routing + ShellRoute)
- **HTTP client**: dio ^5.7.0
- **Audio playback**: just_audio ^0.10.5 + audio_service ^0.18.17
- **Audio backend**: all native platforms (Win/Linux/macOS/Android/iOS) uniformly use just_audio_media_kit (libmpv, no fallback); Web uses just_audio_web + self-integrated hls.js
- **Video picture**: media_kit_video (native platforms derive a VideoController from the same libmpv Player; Web uses a muted `<video>` synced to playback, songloft-org/songloft#76)
- **Local storage**: shared_preferences ^2.3.4
- **Image caching**: cached_network_image ^3.4.1
- **Color extraction**: palette_generator ^0.3.3+4
- **WebView**: flutter_inappwebview ^6.1.5 (loading JS plugin pages)
- **Permission management**: permission_handler ^12.0.1
- **UI framework**: Material 3 (seedColor: M3 Blue baseline `#415F91`)

## Design Philosophy

- **Music playback at the core**: the player is always visible and controllable at any time
- **Responsive three-way adaptation**: adaptive layouts for Mobile / Tablet / Desktop
- **Feature-First architecture**: code organized by feature module, each with three layers — data / domain / presentation
- **Consistent cross-platform experience**: a single codebase adapts to six platforms, with optimizations tailored to each platform's characteristics

## Directory Structure

```
clients/player/lib/
├── config/                          # App configuration
│   ├── app_config.dart              # API config, deployment mode, version number
│   └── constants.dart               # App constants
├── core/                            # Core infrastructure
│   ├── a11y/                        # Accessibility
│   │   ├── web_semantics_controller.dart  # Web semantics tree controller
│   │   └── semantics_pointer_override*.dart  # Pointer event semantics override (conditional import)
│   ├── audio/                       # Audio playback engine (20 files)
│   │   ├── audio_service.dart       # SongloftAudioHandler (audio playback, notification bar controls)
│   │   ├── songloft_just_audio_platform.dart  # Custom JustAudio platform registration
│   │   ├── songloft_mediakit_player.dart      # libmpv native player wrapper
│   │   ├── songloft_web_audio_player.dart     # Web HTML5 Audio + hls.js player
│   │   ├── equalizer_service*.dart            # Equalizer service (mpv / web dual implementations)
│   │   ├── smtc_service*.dart                 # Windows SMTC media controls (conditional import)
│   │   ├── video_controller_provider.dart     # Video surface controller Provider
│   │   ├── system_volume_provider.dart        # System volume Provider
│   │   └── media_browse_data_source.dart      # Android Auto media browse data source
│   ├── backend/                     # Bundle local mode (embedded backend abstraction layer)
│   │   ├── embedded_backend_service.dart   # Unified interface (mobile MethodChannel / desktop subprocess dispatch)
│   │   ├── desktop_backend_service.dart    # Desktop: start the songloft-server subprocess
│   │   ├── native_contract_service.dart    # Native service contract
│   │   ├── run_mode_provider.dart          # RunMode enum (local/remote) + persistence Provider
│   │   └── backend_lifecycle.dart          # WidgetsBindingObserver: auto-restart backend on foreground resume
│   ├── network/                     # Network layer (19 files)
│   │   ├── api_client.dart          # Dio HTTP client wrapper
│   │   ├── api_exceptions.dart      # API exception definitions
│   │   ├── auth_interceptor.dart    # JWT Token auto-refresh interceptor
│   │   ├── base_url_provider.dart   # Base URL Provider
│   │   ├── server_probe.dart        # Server probing
│   │   ├── servers_provider.dart    # Multi-server management Provider
│   │   ├── lan_address.dart         # LAN address discovery
│   │   ├── github_proxy_fallback.dart  # GitHub proxy fallback
│   │   └── ...                      # TLS / redirect / media proxy conditional imports
│   ├── platform/
│   │   ├── live_activity_service.dart   # iOS Dynamic Island / Live Activity integration
│   │   └── home_widget_service.dart     # Desktop widget service
│   ├── router/
│   │   └── app_router.dart          # GoRouter route configuration (with auth guard)
│   ├── storage/                     # Local storage (6 files)
│   │   ├── app_preferences.dart     # SharedPreferences wrapper
│   │   ├── secure_storage.dart      # Secure storage (Token caching)
│   │   ├── lyric_cache_service.dart # Local lyric caching
│   │   ├── song_cache_service.dart  # Song cache service
│   │   ├── playback_state_storage.dart  # Playback state persistence
│   │   └── preference_sync_service.dart # Preference sync service
│   ├── theme/
│   │   ├── app_theme.dart           # Material 3 theme + SongloftThemeExtension (incl. Liquid Glass tokens)
│   │   ├── app_dimensions.dart      # Size and border-radius constants
│   │   ├── responsive.dart          # Responsive breakpoints and utility extensions
│   │   └── widgets/                 # Theme components
│   │       ├── glass_surface.dart   # Glass effect container
│   │       └── glass_capsule_bar.dart  # Glass capsule navigation bar
│   ├── tracely/
│   │   └── tracely_client.dart      # Tracely frontend monitoring-report client
│   ├── updater/                     # App updates (5 files)
│   │   ├── backend_patch_service.dart   # Backend hot-patch service
│   │   ├── patch_update_service.dart    # Patch update service
│   │   ├── patch_update_dialog.dart     # Patch update dialog
│   │   ├── channel_release_resolver.dart  # Update channel resolver
│   │   └── version_compare.dart         # Version number comparison
│   ├── utils/                       # Utilities (31 files)
│   │   ├── color_extraction.dart    # Cover color extraction
│   │   ├── formatters.dart          # Formatting utilities (duration, file size, etc.)
│   │   ├── platform_utils.dart      # Platform detection utilities
│   │   ├── url_helper.dart          # URL building helpers (base_url concatenation, token appending, etc.)
│   │   ├── audio_format_helper.dart # Audio format helper
│   │   ├── window_tray_manager.dart # Window tray management
│   │   └── ...                      # Web platform helpers (cache clearing, fullscreen, WebGL recovery, etc.)
│   ├── plugin_iframe_gate.dart      # Plugin iframe gate (conditional import entry)
│   ├── plugin_iframe_gate_web.dart  # Web iframe gate implementation
│   └── plugin_iframe_gate_stub.dart # Non-web platform stub
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
│   │       ├── startup_gate.dart    # Startup gate: local mode auto-bootstrap / remote mode server probe
│   │       └── web_update_gate.dart # Web forced-update gate
│   ├── home/                        # Home module
│   │   ├── domain/
│   │   │   └── home_grid_config.dart  # Home grid layout configuration
│   │   └── presentation/
│   │       ├── home_page.dart         # Home page (playlist carousel, stats strip, JS plugin grid)
│   │       ├── plugin_host_bridge.dart     # Plugin host bridge (native WebView callHandler registration)
│   │       ├── plugin_host_dispatch.dart   # Plugin host dispatch (transport-agnostic, web-safe)
│   │       ├── plugin_webview_page.dart    # JS plugin WebView page (conditional import)
│   │       ├── plugin_tab_page.dart        # Plugin tab page (conditional import)
│   │       ├── providers/
│   │       │   └── home_grid_config_provider.dart
│   │       ├── render/              # Plugin render engine (WebView)
│   │       │   ├── plugin_render_view.dart
│   │       │   ├── plugin_render_controller.dart
│   │       │   ├── plugin_render_surface_webview.dart
│   │       │   └── ...              # Render helpers (color scheme, etc.)
│   │       └── widgets/
│   │           ├── playlist_carousel.dart  # Playlist carousel component
│   │           ├── hero_card.dart          # Hero card
│   │           ├── stats_strip.dart        # Stats strip
│   │           └── section_header.dart     # Section header
│   ├── jsplugin/                    # JS plugin module
│   │   ├── data/
│   │   │   ├── jsplugin_api.dart    # JS plugin API (with JSPlugin model, upload, update check)
│   │   │   └── plugin_order.dart    # Plugin ordering
│   │   └── presentation/
│   │       ├── providers/
│   │       │   └── jsplugin_provider.dart
│   │       └── widgets/
│   │           ├── jsplugin_grid.dart      # JS plugin entry grid (used on home page)
│   │           ├── jsplugin_manager.dart   # JS plugin management panel (used on settings page)
│   │           ├── plugin_registry.dart    # Plugin repository browser
│   │           ├── plugin_icon.dart        # Plugin icon
│   │           └── plugin_icon_utils.dart  # Icon utilities
│   ├── library/                     # Song library module (incl. category / folder / tag browsing)
│   │   ├── data/
│   │   │   ├── songs_api.dart       # Song API
│   │   │   ├── songs_repository.dart
│   │   │   └── song_tags_api.dart   # Song tag API
│   │   ├── domain/
│   │   │   ├── repositories/
│   │   │   │   └── songs_repository_interface.dart
│   │   │   └── use_cases/
│   │   │       └── favorite_service.dart  # Favorite service
│   │   └── presentation/
│   │       ├── library_page.dart          # Library page (multi-view: songs / playlists / categories / folders / tags)
│   │       ├── song_edit_page.dart        # Song edit page
│   │       ├── category_songs_page.dart   # Category song list page
│   │       ├── folder_content_page.dart   # Folder content page
│   │       ├── tag_songs_page.dart        # Tag song list page
│   │       ├── providers/
│   │       │   ├── songs_provider.dart
│   │       │   ├── favorite_provider.dart
│   │       │   ├── category_provider.dart # Category browsing Provider
│   │       │   ├── folder_provider.dart   # Folder browsing Provider
│   │       │   └── song_tag_provider.dart # Song tag Provider
│   │       └── widgets/
│   │           ├── song_list_tile.dart        # Song list item
│   │           ├── library_view_switcher.dart # View switcher
│   │           ├── facet_grid_view.dart       # Facet grid view (albums, artists, etc.)
│   │           ├── folder_browse_view.dart    # Folder browse view
│   │           └── tag_grid_view.dart         # Tag grid view
│   ├── player/                      # Player module
│   │   ├── data/
│   │   │   └── play_history_api.dart  # Play history API
│   │   ├── domain/
│   │   │   ├── player_state.dart      # Player state definitions
│   │   │   ├── lyric_parser.dart      # LRC lyric parser
│   │   │   ├── playback_context.dart  # Playback context
│   │   │   ├── equalizer_setting.dart # Equalizer settings
│   │   │   └── use_cases/             # Domain use cases (8)
│   │   │       ├── play_queue.dart        # Play queue management
│   │   │       ├── queue_loader.dart      # Queue loader
│   │   │       ├── play_mode_resolver.dart  # Play mode resolver
│   │   │       ├── prefetch_strategy.dart   # Prefetch strategy
│   │   │       ├── sleep_timer_logic.dart   # Sleep timer
│   │   │       └── ...                     # Retry policy, resume state, etc.
│   │   └── presentation/
│   │       ├── queue_page.dart        # Play queue page
│   │       ├── lyric_adjust_page.dart # Lyric time offset adjustment page
│   │       ├── providers/             # Providers (10)
│   │       │   ├── player_provider.dart     # Core playback Provider
│   │       │   ├── lyric_provider.dart      # Lyric Provider
│   │       │   ├── equalizer_provider.dart  # Equalizer Provider
│   │       │   ├── audio_track_provider.dart  # Audio track switch Provider
│   │       │   ├── play_history_provider.dart # Play history Provider
│   │       │   └── ...                       # Web video sync, etc.
│   │       ├── utils/
│   │       │   ├── full_player_route.dart   # Full-screen player route helper
│   │       │   └── player_song_actions.dart # Player song actions
│   │       └── widgets/               # Player widgets (25)
│   │           ├── desktop_player.dart       # Desktop player bar
│   │           ├── desktop_full_player.dart  # Desktop fullscreen player
│   │           ├── mobile_player.dart        # Mobile fullscreen player
│   │           ├── mini_player.dart          # Mini player bar
│   │           ├── side_player.dart          # Side player panel
│   │           ├── play_controls.dart        # Playback control buttons
│   │           ├── popup_controls.dart       # Popup control panel
│   │           ├── progress_bar.dart         # Progress bar
│   │           ├── volume_control.dart       # Volume control
│   │           ├── lyrics_view.dart          # Lyrics display
│   │           ├── karaoke_line.dart         # Karaoke word-by-word highlight
│   │           ├── playlist_drawer.dart      # Playlist drawer
│   │           ├── equalizer_panel.dart      # Equalizer panel
│   │           ├── video_player_surface.dart # Video player surface
│   │           ├── play_history_sheet.dart   # Play history sheet
│   │           ├── vinyl_ring.dart           # Vinyl record animation
│   │           └── ...                       # Audio track control, video stage, shortcut scope, etc.
│   ├── playlist/                    # Playlist module
│   │   ├── data/
│   │   │   ├── playlist_api.dart    # Playlist CRUD
│   │   │   └── playlist_repository.dart
│   │   ├── domain/
│   │   │   ├── playlist.dart          # Playlist model
│   │   │   ├── repositories/
│   │   │   │   └── playlist_repository_interface.dart
│   │   │   └── use_cases/
│   │   │       ├── playlist_sort.dart      # Playlist sorting
│   │   │       └── pinyin_comparator.dart  # Pinyin comparator
│   │   └── presentation/
│   │       ├── playlist_detail_page.dart  # Playlist detail page
│   │       ├── providers/
│   │       │   ├── playlist_provider.dart
│   │       │   └── playlist_view_provider.dart
│   │       └── widgets/               # Playlist widgets (10)
│   │           ├── playlist_browse_view.dart     # Playlist browse view
│   │           ├── playlist_card.dart            # Playlist card
│   │           ├── playlist_list_item.dart       # Playlist list item
│   │           ├── playlist_search_field.dart    # Playlist search field
│   │           ├── playlist_edit_dialog.dart     # Playlist edit dialog
│   │           ├── playlist_form_dialog.dart     # Playlist form dialog
│   │           ├── playlist_song_tile.dart       # Playlist song item
│   │           ├── song_cover_picker_modal.dart  # Song cover picker modal
│   │           └── ...                          # Cover editing, adapters, etc.
│   ├── settings/                    # Settings module
│   │   ├── data/                    # Data layer (12 files)
│   │   │   ├── cache_api.dart       # Music cache API
│   │   │   ├── config_api.dart      # Generic config API (/configs/{key})
│   │   │   ├── settings_api.dart    # Business settings API (/settings/* endpoint wrapper)
│   │   │   ├── directory_api.dart   # Directory browsing API
│   │   │   ├── scan_api.dart        # Scan API
│   │   │   ├── theme_pack_api.dart  # Theme pack API (incl. ThemePack / ThemePackColors models)
│   │   │   ├── upgrade_api.dart     # Upgrade API
│   │   │   ├── frontend_version_api.dart  # Frontend version check API
│   │   │   ├── log_export_service.dart    # Log export service
│   │   │   └── ...                        # Log share conditional imports
│   │   ├── domain/
│   │   │   ├── key_binding.dart            # Key binding
│   │   │   └── player_shortcut_action.dart # Player shortcut actions
│   │   └── presentation/
│   │       ├── settings_page.dart           # Settings page (entry)
│   │       ├── servers_page.dart            # Server management page
│   │       ├── duplicate_check_page.dart    # Duplicate song check page
│   │       ├── client_download_page.dart    # Client download page
│   │       ├── licenses_page.dart           # Open source licenses page
│   │       ├── shortcut_settings_page.dart  # Shortcut settings page
│   │       ├── providers/
│   │       │   ├── settings_provider.dart
│   │       │   ├── theme_pack_provider.dart
│   │       │   ├── shortcut_settings_provider.dart
│   │       │   ├── cache_download_provider.dart
│   │       │   └── song_cache_provider.dart
│   │       └── widgets/             # Settings widgets (18)
│   │           ├── settings_master_detail.dart    # Master-detail layout
│   │           ├── settings_category_content.dart # Category content
│   │           ├── section_card.dart              # Section card
│   │           ├── scan_manager.dart              # Scan management
│   │           ├── cache_manager.dart             # Cache management
│   │           ├── config_manager.dart            # Config management
│   │           ├── exclude_dir_manager.dart       # Exclude directory management
│   │           ├── theme_selector.dart            # Theme selector
│   │           ├── theme_pack_manager.dart        # Theme pack management
│   │           ├── theme_catalog.dart             # Online theme catalog
│   │           ├── token_manager.dart             # Token management
│   │           ├── language_selector.dart         # Language selector
│   │           ├── home_grid_selector.dart        # Home layout selector
│   │           ├── upgrade_dialog.dart            # Backend upgrade dialog
│   │           ├── frontend_upgrade_dialog.dart   # Frontend upgrade dialog
│   │           ├── github_proxy_dialog.dart       # GitHub proxy dialog
│   │           ├── metadata_refresh_manager.dart  # Metadata refresh management
│   │           └── shortcut_recorder.dart         # Shortcut recorder
│   ├── desktop_lyric/               # Desktop lyrics module
│   │   ├── desktop_lyric_controller.dart    # Desktop lyric controller
│   │   ├── desktop_lyric_main.dart          # Desktop lyric entry point
│   │   ├── desktop_lyric_ipc.dart           # Desktop lyric IPC
│   │   ├── desktop_lyric_font_size.dart     # Font size configuration
│   │   ├── android_floating_lyric_controller.dart  # Android floating lyric controller
│   │   └── presentation/
│   │       ├── desktop_lyric_app.dart       # Desktop lyric window app
│   │       └── desktop_lyric_view.dart      # Desktop lyric view
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
├── l10n/                            # Internationalization
│   ├── app_en.arb                   # English
│   ├── app_zh.arb                   # Chinese
│   ├── app_es.arb                   # Spanish
│   ├── app_localizations.dart       # Generated localization class
│   └── l10n_holder.dart             # Global l10n accessor
├── shared/                          # Shared modules
│   ├── constants/
│   │   └── github_proxy.dart        # GitHub proxy constants
│   ├── layouts/
│   │   ├── shell_layout.dart        # ShellRoute main layout (navigation + player)
│   │   ├── adaptive_scaffold.dart   # Adaptive scaffold
│   │   └── active_destinations.dart # Dynamic navigation destinations
│   ├── mixins/
│   │   └── song_list_actions.dart   # Song list actions mixin
│   ├── models/
│   │   ├── song.dart                # Song model
│   │   ├── artist.dart              # Artist model
│   │   ├── library_stats.dart       # Library stats model
│   │   ├── pagination.dart          # Pagination model
│   │   └── api_response.dart        # API response model
│   ├── utils/
│   │   └── responsive_snackbar.dart # Responsive SnackBar
│   └── widgets/                     # Shared components (22)
│       ├── cover_image.dart         # Cover image component
│       ├── network_cover_image.dart # Network cover image
│       ├── favorite_button.dart     # Favorite button
│       ├── scrolling_text.dart      # Scrolling text
│       ├── confirm_dialog.dart      # Confirmation dialog
│       ├── delete_song_dialog.dart  # Delete song dialog
│       ├── add_to_playlist_modal.dart  # Add-to-playlist modal
│       ├── song_picker_modal.dart   # Song picker modal
│       ├── manage_tags_modal.dart   # Tag management modal
│       ├── browse_card.dart         # Browse card
│       ├── browse_collection_view.dart  # Browse collection view
│       ├── entity_detail_scaffold.dart  # Entity detail scaffold
│       ├── song_tile.dart           # Song tile
│       ├── filter_pill.dart         # Filter pill
│       ├── directory_picker_sheet.dart  # Directory picker sheet
│       ├── directory_tree_selector.dart # Directory tree selector
│       ├── draggable_scrollbar_overlay.dart  # Draggable scrollbar overlay
│       ├── scroll_to_top_fab.dart   # Scroll-to-top FAB
│       ├── selection_action_button.dart  # Multi-select action button
│       ├── empty_state.dart         # Empty state
│       ├── error_view.dart          # Error view
│       └── loading_indicator.dart   # Loading indicator
└── main.dart                        # App entry point
```

## Page Structure

### Route Configuration

| Page | Route | Description |
|------|------|------|
| Login | `/login` | Login page (standalone route, does not use ShellRoute) |
| Home | `/` | Playlist carousel, stats strip, JS plugin grid |
| Library | `/library` | Multi-view: songs / playlists / categories / folders / tags |
| Category songs | `/library/categories/:field?value=` | Song list by album, artist, etc. |
| Folders | `/library/folders?path=` | Folder content drill-down page |
| Tag songs | `/library/tags/:tagId?name=` | Song list for a tag |
| Playlists | `/playlists` | **Merged into Library**, redirects to `/library` |
| Playlist detail | `/playlists/:id` | Playlist details and song list |
| Settings | `/settings` | Master-detail layout, categorized (appearance, scan, cache, network, etc.) |
| Settings category | `/settings/category/:index` | Settings category detail (mobile secondary page) |
| Servers | `/settings/servers` | Multi-server management |
| Duplicate check | `/settings/duplicate-check` | Duplicate song detection |
| Shortcuts | `/settings/shortcuts` | Keyboard shortcuts (desktop) |
| Client download | `/settings/download` | Client download (visible on Web) |
| Plugin registry | `/settings/plugin-registry` | Plugin repository browser and install |
| Licenses | `/settings/licenses` | Open source licenses |
| Plugin | `/plugin?url=&name=` | JS plugin WebView page (fullscreen, standalone route) |
| Plugin tab | `/plugin-tab/:entryPath` | Plugin tab page (managed by ShellLayout) |
| Full player | `/player` | Full-screen player (standalone route, dispatches Mobile/Desktop by screen type) |

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
| **Tablet** | 600 - 900px | Bottom navigation + player (wider) |
| **Desktop** | 900px+ | Side navigation + player |

### Layout Architecture

```
ShellLayout (ShellRoute builder)
├── AdaptiveScaffold
│   ├── Mobile/Tablet: NavigationBar (bottom) + MiniPlayer
│   └── Desktop: NavigationRail (side) + DesktopPlayer (bottom)
└── Content area (GoRouter child)
```

### Player Form (`navigationStyle`)

The bottom player has **two forms**, selected by the theme extension field `SongloftThemeExtension.navigationStyle` and orthogonal to the breakpoints:

| Form | Mobile | Tablet / Desktop |
|------|--------|------------------|
| **`capsule`** (capsule / glass theme) | Floating capsule bar: fully rounded pill, slimmed-down controls, **no real blur** (translucent fill + inner highlight) | **Floating glass capsule bar**: pill height 64, radius = height / 2, real frosted glass (`GlassSurface` + `BackdropFilter`, sigma 20), a 3px rounded progress track along the top edge plus a 16px tappable / draggable hit area above it, and a persistent toolbar of play mode · volume · lyrics · queue · more |
| **standard** (default) | Bottom mini player (2px progress + 64px body, occupying the `bottomNavigationBar` slot) | A bar that takes up layout height: `MiniPlayer` on tablet, the 90px `DesktopPlayer` on desktop (with `border-top`) |

In the capsule form, tablet / desktop do **not** render the player as a `Column` sibling; `AdaptiveScaffold._overlayBottomPlayer` **floats it above the content** with a `Stack`, spanning the full width of "content column + playlist drawer"; on desktop the capsule bar likewise does not cover the 240px frosted-glass sidebar. The playlist drawer itself is unaware of the capsule bar: its surface still extends to the bottom of the screen, and the floating capsule bar covers a sliver of the drawer's lower edge. Because the player no longer occupies layout height, scrolling content reserves its own bottom space via `ResponsiveContext.navScrollInset` (wide capsule tier: `bottom + 84 = 64 + 12 + 8`; mobile tier stays `bottom + 16`).

Size constants live in `AppCapsulePlayer` in `lib/core/theme/app_dimensions.dart` (mobile height 59 = 11 + 48, wide height 64 = 16 + 48; the pill radius is derived from the height, so no new radius token is introduced). The content row (cover / title / controls) is **vertically centered** inside the pill: the pill height is the top progress hit area plus the content row, and the hit area is exactly as tall as the bottom breathing space, so the cover and the buttons split the remaining space evenly above and below. The top-edge progress bar is a separate overlay layer and still hugs the pill's top border at full width. The single implementation of the capsule form is `CapsuleMiniPlayer` (`lib/features/player/presentation/widgets/capsule_mini_player.dart`); the mobile tier (`CapsuleMiniPlayer.compact`) and the wide tier share one skeleton and differ only in size, real blur, and control density. `WidescreenSidePlayer` for `widescreen` is out of scope.

## Theme System

### Material 3 Color Scheme

- **Primary color**: M3 Blue baseline (`#415F91`)
- **Color scheme**: `ColorScheme.fromSeed(seedColor: Color(0xFF415F91))`
- **Theme mode**: light / dark / follow system
- **Font fallback**: NotoSansSC (Chinese) / NotoSansKR (Korean)

### Responsive Theme

The theme dynamically adjusts component sizes based on screen type:
- **SnackBar**: fixed width, centered on Desktop
- **Dialogs**: maximum width adjusted by screen type

## UI Component Design Guidelines

### Play Button Shape (PlayControls)

The core play control component `PlayControls` (`lib/features/player/presentation/widgets/play_controls.dart`) uses the `useRoundedRect` parameter to control button shape:

```dart
final borderRadius =
    useRoundedRect ? AppRadius.xxlAll : BorderRadius.circular(size);
```

- **Circle** (`useRoundedRect = false`, default): `BorderRadius.circular(size)` produces a perfect circle on a square container
- **Rounded rectangle** (`useRoundedRect = true`): fixed 28px corner radius (`AppRadius.xxl`), producing a superellipse/capsule shape

**Design principle: button size determines shape**

| Platform / Context | Size | Shape | Rationale |
|-------------------|------|-------|-----------|
| Desktop full player | 52px | Circle | Medium size, circle is visually balanced |
| Desktop bottom bar | 40px | Circle | Small size, circle is compact |
| Car-mode side panel | 52px | Circle | Same as desktop |
| **Mobile full player** | 76px | Rounded rect | Large circle looks bulky; rounded rect is more modern and compact |
| **Video player** | 60px | Rounded rect | Better proportion with video frame, avoids an overly prominent circle |

**Rule**: Primary play buttons at 60px or above use `useRoundedRect: true`; those at 52px or below use the default circle. This is an intentional visual balance decision, not a bug. Follow this rule when adding new player UIs.

### Other Play Button Variants

- **CompactPlayButton**: Icon-only button (`IconButton`), no background fill, used in compact spaces like the mini player bar
- **Browse Card FAB**: `Material(shape: CircleBorder())` floating circular button with elevation, shown on grid card hover for quick play
- **"Play All" FilledButton**: `FilledButton.icon` pill-shaped button with text label, used in playlist/category page headers

<a id="deployment-modes"></a>

## Deployment Modes

### Embedded Mode

```bash
flutter build web --dart-define=DEPLOY_MODE=embedded
```

- Explicitly selected legacy embedded Flutter Web shares the backend origin
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
- Local mode startup flow: request storage permission → start the embedded backend at `127.0.0.1:<port>` → poll the health check (up to 10 × 300ms) → auto login with `admin/admin`
- `BackendLifecycle` (WidgetsBindingObserver) listens to the app lifecycle and automatically restarts the backend when the app resumes to the foreground
- **Android paths**: in Bundle mode the CWD is `/`, so directories such as covers must resolve relative to `DBPath`, not the CWD
- **Mobile temporary directory**: before initializing services, `mobile.Start` creates `<dataDir>/tmp` and sets the Go process's `TMPDIR` for temporary HLS, download, and cover files. Android's default `/data/local/tmp` is not writable by ordinary apps. `Stop` retains this environment setting; the HLS service removes its own session directory when closed.
- **Native bridging**: Android calls the gomobile-generated classes by reflection via `Class.forName("mobile.Mobile")`; when the `.aar` is not bundled, `isAvailable()` returns false (graceful degradation). iOS uses Swift to call Objective-C functions such as `MobileStart`
- **Desktop subprocess**: `DesktopBackendService` looks for `songloft-server` in the same directory as the Flutter executable (on macOS, `Contents/Resources/`) and parses the actual listening port from stdout
- **CI artifacts**: all start with `songloft-bundled-`; Android/Linux/Windows include the architecture, while Apple uses `songloft-bundled-macos.zip` and `songloft-bundled-ios-nosign.ipa`. The 4 parallel jobs (Android/Linux/Apple/Windows) do not block the main release on failure

## Audio Playback Architecture

```
SongloftAudioHandler (extends BaseAudioHandler)
├── just_audio (core playback engine)
│   ├── Web: HTML5 Audio + hls.js (custom SongloftWebJustAudioPlugin)
│   └── Win/Linux/macOS/Android/iOS: media_kit (libmpv), unified across all native platforms, no fallback
├── audio_service (system notification bar / lock screen controls)
└── audio_session (audio focus management)
```

<a id="platform-adaptation"></a>

### Platform Adaptation

- **Android**: foreground service runs continuously (`androidStopForegroundOnPause: false`), compatible with aggressive reclamation strategies such as HyperOS3
- **Android 13+**: requests notification permission at runtime
- **macOS**: secure_storage automatically falls back to SharedPreferences when unsigned
- **Audio backend**: all native platforms (Win/Linux/macOS/Android/iOS) uniformly use `just_audio_media_kit` (libmpv), enabling in-app video, with no fallback (the kill-switch has been removed; falling back to AVPlayer/ExoPlayer is no longer supported)

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
# Web embedded mode (output to clients/player-build/web-embedded, for Go binary //go:embed)
make build-frontend-flutter-web-embedded

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
make build-go-mobile-android       # → clients/player/android/app/libs/songloft.aar
make build-go-mobile-ios           # → clients/player/ios/Songloft.xcframework (macOS only)
make build-go-desktop-linux        # → clients/player/linux/songloft-server
make build-go-desktop-windows      # → clients/player/windows/songloft-server.exe
make build-go-desktop-macos-arm64  # → clients/player/macos/Runner/songloft-server

# 2. Build the Flutter client (add --dart-define=HAS_BACKEND=true)
# In CI this is done automatically by release.yml's build-bundled-{android,linux,apple,windows} Jobs
```

Prebuilt installer downloads:
- Standard edition (requires connecting to a server): [https://github.com/songloft-org/songloft-player/releases](https://github.com/songloft-org/songloft-player/releases)
- Bundle edition (backend embedded): [https://github.com/songloft-org/songloft/releases](https://github.com/songloft-org/songloft/releases) (`songloft-bundled-*` files)
