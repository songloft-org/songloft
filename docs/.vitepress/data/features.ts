// 落地页「交替图文特性行」的数据。image 指向 docs/public/screenshots/ 下的截图，
// frame 决定用浏览器窗口还是手机边框包裹；reverse 控制左右布局。
import type { L } from './downloads'

export interface FeatureRow {
  id: string
  title: L
  desc: L
  bullets: L[]
  image: string
  frame: 'browser' | 'phone'
  reverse: boolean
}

export const FEATURES: FeatureRow[] = [
  {
    id: 'library',
    title: { zh: '本地音乐，尽在掌握', en: 'Your local library, fully in hand' },
    desc: {
      zh: '扫描本地目录，自动识别 MP3 / FLAC / WAV / APE / OGG / Opus / M4A / M4B / WMA / DSF（DSD）等音频及 MP4 / MKV / FLV / MPG 等视频格式，提取封面、歌词与元数据。支持手动扫描与可选的定时自动扫描，同步目录变化。',
      en: 'Scan local folders and auto-detect MP3 / FLAC / WAV / APE / OGG / Opus / M4A / M4B / WMA / DSF (DSD) audio and MP4 / MKV / FLV / MPG video formats — covers, lyrics and metadata included. Sync folder changes with manual scans or optional scheduled scans.',
    },
    bullets: [
      { zh: '主流格式全支持', en: 'All common formats' },
      { zh: '封面 / 歌词 / 元数据一键提取', en: 'Covers · lyrics · tags' },
      { zh: '手动扫描 · 定时同步目录变化', en: 'Manual · scheduled scans' },
    ],
    image: '/screenshots/library-desktop.png',
    frame: 'browser',
    reverse: false,
  },
  {
    id: 'player',
    title: { zh: '沉浸式播放体验', en: 'Immersive now playing' },
    desc: {
      zh: '从专辑封面实时提取配色，让每首歌拥有独特的视觉氛围。支持后台播放、锁屏与通知栏媒体控制。',
      en: 'Colors are extracted live from album art so every track gets its own mood. Background playback with lock-screen and notification controls.',
    },
    bullets: [
      { zh: '封面动态取色 · 视频画面同源播放', en: 'Live color · in-app video' },
      { zh: '后台播放 · 锁屏 / 通知栏媒体控制', en: 'Background · lock-screen controls' },
      { zh: '桌面歌词 · DLNA 投屏 · 均衡器', en: 'Desktop lyrics · DLNA · equalizer' },
    ],
    image: '/screenshots/player-mobile.png',
    frame: 'phone',
    reverse: true,
  },
  {
    id: 'playlists',
    title: { zh: '歌单随心组织', en: 'Organize with playlists' },
    desc: {
      zh: '本地与网络歌曲统一管理，歌曲与歌单共用自定义标签。普通歌单独立记住歌曲、播放位置与模式，有声书可继续收听。',
      en: 'Manage local and remote tracks with shared custom tags for songs and playlists. Normal playlists remember the track, position and mode, with resume listening for audiobooks.',
    },
    bullets: [
      { zh: '共享标签 · 收藏与电台收藏', en: 'Shared tags · music & radio favorites' },
      { zh: '歌单续播 · 有声书模式', en: 'Playlist resume · audiobooks' },
      { zh: '大歌单分批加载 · 一键播放', en: 'Batched queues · one-tap playback' },
    ],
    image: '/screenshots/playlist-detail-desktop.png',
    frame: 'browser',
    reverse: false,
  },
  {
    id: 'themes',
    title: { zh: '明暗随心 · 六端同行', en: 'Light, dark & every platform' },
    desc: {
      zh: '深浅色主题一键切换，跟随系统自动变化。一套 Flutter 客户端覆盖 Android / iOS / macOS / Windows / Linux / Web。',
      en: 'Switch light and dark themes, or follow the system. One Flutter client runs on Android / iOS / macOS / Windows / Linux / Web.',
    },
    bullets: [
      { zh: '浅色 / 深色 / 跟随系统', en: 'Light · dark · system' },
      { zh: '自定义主题包 · 在线主题目录', en: 'Custom theme packs · online catalog' },
      { zh: 'Web 界面开箱即用', en: 'Web UI out of the box' },
    ],
    image: '/screenshots/home-desktop.png',
    frame: 'browser',
    reverse: true,
  },
  {
    id: 'plugins',
    title: { zh: '插件化扩展，能力无限', en: 'Extend it with plugins' },
    desc: {
      zh: '基于 QuickJS 沙箱运行 JS 插件，按需扩展音源 / 元数据 / 设备控制。权限模型 + 热更新 + 健康检查，安全可控。',
      en: 'Run JS plugins in a QuickJS sandbox to add sources, metadata and device control — with a permission model, hot-reload and health checks.',
    },
    bullets: [
      { zh: 'QuickJS 沙箱隔离', en: 'QuickJS sandbox' },
      { zh: '权限模型 + 热更新', en: 'Permissions + hot-reload' },
      { zh: '完整 REST API + Swagger', en: 'Full REST API + Swagger' },
    ],
    image: '/screenshots/home-mobile.png',
    frame: 'phone',
    reverse: false,
  },
  {
    id: 'control',
    title: { zh: '掌控每一处细节', en: 'Control every detail' },
    desc: {
      zh: 'Flutter 客户端可切换多个服务器及本地 / 远程模式。可选定时扫描，按需计算指纹查找重复歌曲；播放历史按上下文记忆，远程歌曲可缓存，歌单与标签可导出备份。',
      en: 'Switch servers and local/remote modes in the Flutter client. Enable scheduled scans and calculate fingerprints on demand to find duplicates. Keep history per context, cache remote tracks, and export playlists with tags.',
    },
    bullets: [
      { zh: '多服务器 · 本地/远程双模式', en: 'Multi-server · local & remote' },
      { zh: '可选自动扫描 · 按需指纹查重', en: 'Optional scans · fingerprint duplicate search' },
      { zh: '播放历史 · 本地缓存 · 备份', en: 'History · cache · backup' },
    ],
    image: '/screenshots/settings-desktop.png',
    frame: 'browser',
    reverse: true,
  },
]
