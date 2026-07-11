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
      zh: '扫描本地目录，自动识别 MP3 / FLAC / WAV / APE / OGG / M4A / MOV，提取封面、歌词与元数据。目录变更实时同步，支持定时与手动扫描。',
      en: 'Scan local folders and auto-detect MP3 / FLAC / WAV / APE / OGG / M4A / MOV — covers, lyrics and metadata included. Changes sync in real time.',
    },
    bullets: [
      { zh: '主流格式全支持', en: 'All common formats' },
      { zh: '封面 / 歌词 / 元数据一键提取', en: 'Covers · lyrics · tags' },
      { zh: '新增 / 修改 / 删除自动同步', en: 'Auto change sync' },
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
      { zh: '专辑封面动态取色', en: 'Live color from art' },
      { zh: '后台播放 & 媒体控制', en: 'Background & media controls' },
      { zh: '耳机 / 锁屏 / 通知栏', en: 'Headset · lock screen' },
    ],
    image: '/screenshots/player-mobile.png',
    frame: 'phone',
    reverse: true,
  },
  {
    id: 'playlists',
    title: { zh: '歌单随心组织', en: 'Organize with playlists' },
    desc: {
      zh: '自由创建歌单，本地与网络歌曲统一管理。内置「收藏」「电台收藏」，一键播放整张歌单。',
      en: 'Create playlists freely and manage local and remote tracks together, with built-in Favorites and one-tap play-all.',
    },
    bullets: [
      { zh: '本地 + 网络统一管理', en: 'Local + remote unified' },
      { zh: '网络电台流一站式播放', en: 'Internet radio streams' },
      { zh: '一键播放整张歌单', en: 'One-tap play all' },
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
      { zh: '六平台一套代码', en: 'One codebase, 6 platforms' },
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
]
