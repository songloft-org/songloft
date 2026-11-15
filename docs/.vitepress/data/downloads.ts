// 落地页「安装选择器」的结构化数据。数据来自仓库 README.md 的下载表。
// 服务端与 Bundle 资产走主仓库 releases/latest/download，独立客户端走各自 Release。

export type Lang = 'zh' | 'en'
export interface L { zh: string; en: string }
export const pick = (l: L, lang: Lang): string => l[lang] ?? l.zh

export const RELEASE_BASE =
  'https://github.com/songloft-org/songloft/releases/latest/download'

export interface Asset {
  arch: string
  archLabel: L
  file: string
  url: string
}
export interface OSGroup {
  os: string
  osLabel: L
  icon: string // devicon/简单标识，用于小图标
  assets: Asset[]
}
export interface Edition {
  id: string
  label: L
  desc: L
  groups: OSGroup[]
}
export interface CommandBlock {
  title: L
  code: string
  group?: L // 同一 group 的命令块归为一种方式，渲染时按 group 分隔
}
export interface ExternalLink {
  label: L
  url: string | L
  primary?: boolean
}
export interface InstallMethod {
  id: string
  label: L
  tagline: L
  icon: string
  kind: 'download' | 'command' | 'external'
  note?: L
  noteLink?: ExternalLink
  editions?: Edition[]
  groups?: OSGroup[]
  commands?: CommandBlock[]
  external?: ExternalLink[]
}

const a = (arch: string, archLabel: L, file: string): Asset => ({
  arch,
  archLabel,
  file,
  url: `${RELEASE_BASE}/${file}`,
})

// 二进制各版本的 OS/arch 分组；suffix 用于区分完整版('')与精简版('-lite')。
const binaryGroups = (suffix: string): OSGroup[] => [
  {
    os: 'linux',
    osLabel: { zh: 'Linux', en: 'Linux' },
    icon: 'linux',
    assets: [
      a('amd64', { zh: 'x86_64', en: 'x86_64' }, `songloft-linux-amd64${suffix}`),
      a('arm64', { zh: 'ARM64', en: 'ARM64' }, `songloft-linux-arm64${suffix}`),
      a('armv7', { zh: 'ARMv7', en: 'ARMv7' }, `songloft-linux-armv7${suffix}`),
    ],
  },
  {
    os: 'macos',
    osLabel: { zh: 'macOS', en: 'macOS' },
    icon: 'apple',
    assets: [
      a('amd64', { zh: 'Intel', en: 'Intel' }, `songloft-darwin-amd64${suffix}`),
      a('arm64', { zh: 'Apple Silicon', en: 'Apple Silicon' }, `songloft-darwin-arm64${suffix}`),
    ],
  },
  {
    os: 'windows',
    osLabel: { zh: 'Windows', en: 'Windows' },
    icon: 'windows',
    assets: [
      a('amd64', { zh: 'x86_64', en: 'x86_64' }, `songloft-windows-amd64${suffix}.exe`),
      a('arm64', { zh: 'ARM64', en: 'ARM64' }, `songloft-windows-arm64${suffix}.exe`),
    ],
  },
]

export const INSTALL: InstallMethod[] = [
  {
    id: 'binary',
    label: { zh: '二进制', en: 'Binary' },
    tagline: { zh: '下载即用 · 单文件', en: 'Single file · run anywhere' },
    icon: 'terminal',
    kind: 'download',
    note: {
      zh: 'macOS 从 GitHub 下载后首次运行需先解除隔离：xattr -d com.apple.quarantine ./songloft',
      en: 'On macOS run `xattr -d com.apple.quarantine ./songloft` before first launch.',
    },
    editions: [
      {
        id: 'full',
        label: { zh: '完整版', en: 'Full' },
        desc: { zh: '内置 Web 前端，开箱即用', en: 'Bundled web UI, ready to go' },
        groups: binaryGroups(''),
      },
      {
        id: 'lite',
        label: { zh: '精简版', en: 'Lite' },
        desc: { zh: '不含 Web 前端，搭配播放器客户端', en: 'No web UI, pair with a player client' },
        groups: binaryGroups('-lite'),
      },
    ],
  },
  {
    id: 'docker',
    label: { zh: 'Docker', en: 'Docker' },
    tagline: { zh: '一行命令拉起容器', en: 'One-line container' },
    icon: 'docker',
    kind: 'command',
    note: {
      zh: 'ARMv7 用户：旧版 Armbian / Debian 的宿主机 libseccomp 可能导致容器启动崩溃。遇到 futexwakeup / SIGSEGV 时，请参阅 FAQ 排查步骤。',
      en: 'ARMv7 users: An outdated host libseccomp on older Armbian / Debian systems may cause the container to crash at startup. If you see futexwakeup / SIGSEGV, see the FAQ troubleshooting steps.',
    },
    noteLink: {
      label: { zh: 'FAQ 排查步骤', en: 'FAQ troubleshooting steps' },
      url: { zh: '/faq#docker-armv7-seccomp', en: '/en/faq#docker-armv7-seccomp' },
    },
    commands: [
      {
        group: { zh: '方式 A · docker run', en: 'Option A · docker run' },
        title: { zh: '拉取镜像（完整版 / 精简版）', en: 'Pull image (full / lite)' },
        code: 'docker pull songloft/songloft:latest\ndocker pull songloft/songloft:lite',
      },
      {
        group: { zh: '方式 A · docker run', en: 'Option A · docker run' },
        title: { zh: '运行容器', en: 'Run container' },
        code:
          'docker run -d --name songloft -p 58091:58091 \\\n' +
          '  -v /path/to/music:/app/music \\\n' +
          '  -v /path/to/data:/app/data \\\n' +
          "  -e ADMIN_USERNAME=admin -e ADMIN_PASSWORD='your_strong_password' \\\n" +
          '  songloft/songloft:latest',
      },
      {
        group: { zh: '方式 B · Docker Compose（推荐）', en: 'Option B · Docker Compose (recommended)' },
        title: { zh: 'docker-compose.yml', en: 'docker-compose.yml' },
        code:
          'services:\n' +
          '  songloft:\n' +
          '    image: songloft/songloft:latest\n' +
          '    container_name: songloft\n' +
          '    restart: always\n' +
          '    ports:\n' +
          '      - "58091:58091"\n' +
          '    volumes:\n' +
          '      - /path/to/music:/app/music\n' +
          '      - /path/to/data:/app/data\n' +
          '    environment:\n' +
          '      - ADMIN_USERNAME=admin\n' +
          '      - ADMIN_PASSWORD=your_strong_password\n' +
          '      - LISTEN_PORT=58091',
      },
      {
        group: { zh: '方式 B · Docker Compose（推荐）', en: 'Option B · Docker Compose (recommended)' },
        title: { zh: '启动 / 查看日志 / 停止', en: 'Up / logs / down' },
        code: 'docker compose up -d\ndocker compose logs -f\ndocker compose down',
      },
    ],
  },
  {
    id: 'homeassistant',
    label: { zh: 'Home Assistant', en: 'Home Assistant' },
    tagline: { zh: 'HAOS 加载项 · 一键安装', en: 'HAOS add-on · one-click' },
    icon: 'home',
    kind: 'external',
    note: {
      zh: '点下方按钮在你的 HA 里弹出「添加加载项仓库」；或手动在「加载项商店 → 仓库」添加 https://github.com/songloft-org/home-assistant-addon，再安装 Songloft。音乐放入 /media，数据持久化在加载项 /data。',
      en: 'Click below to open the "Add add-on repository" dialog in your HA; or add https://github.com/songloft-org/home-assistant-addon under "Add-on Store → Repositories", then install Songloft. Put music in /media; data persists in the add-on /data.',
    },
    external: [
      {
        label: { zh: '一键添加到 Home Assistant', en: 'Add to Home Assistant' },
        url: 'https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https%3A%2F%2Fgithub.com%2Fsongloft-org%2Fhome-assistant-addon',
        primary: true,
      },
      {
        label: { zh: '加载项文档', en: 'Add-on docs' },
        url: 'https://github.com/songloft-org/home-assistant-addon',
      },
    ],
  },
  {
    id: 'router',
    label: { zh: '路由器 / 光猫', en: 'Router / ONT' },
    tagline: { zh: 'OpenWrt / Entware / 梅林', en: 'OpenWrt / Entware / Merlin' },
    icon: 'router',
    kind: 'external',
    note: {
      zh: '社区项目 songloft-for-router 提供路由器与光猫安装包，支持 OpenWrt、Entware 和梅林（SWRTdev / Koolshare 软件中心）。请按固件生态与 CPU 架构选择安装包，安装与服务管理步骤见项目文档。',
      en: 'The community project songloft-for-router provides packages for routers and ONTs running OpenWrt, Entware, or Merlin (SWRTdev / Koolshare software centers). Choose a package for your firmware and CPU architecture; see the project docs for installation and service management.',
    },
    external: [
      {
        label: { zh: 'OpenWrt 安装包', en: 'OpenWrt packages' },
        url: 'https://github.com/songloft-org/songloft-for-router/releases?q=openwrt',
        primary: true,
      },
      {
        label: { zh: 'Entware 安装包', en: 'Entware packages' },
        url: 'https://github.com/songloft-org/songloft-for-router/releases?q=entware',
      },
      {
        label: { zh: '梅林安装包', en: 'Merlin packages' },
        url: 'https://github.com/songloft-org/songloft-for-router/releases?q=merlin',
      },
      {
        label: { zh: '安装与使用文档', en: 'Installation guide (Chinese)' },
        url: 'https://github.com/songloft-org/songloft-for-router#安装文档',
      },
    ],
  },
  {
    id: 'bundle',
    label: { zh: 'Bundle 版', en: 'Bundle' },
    tagline: { zh: '内嵌后端 · 免部署服务器', en: 'Embedded backend · no server' },
    icon: 'package',
    kind: 'download',
    note: {
      zh: '首次启动在登录页点「使用本地模式」→ 选择音乐目录即可，随时可在设置切换本地/远程。',
      en: 'On first launch tap "Local mode" on the login page and pick a music folder.',
    },
    groups: [
      {
        os: 'android',
        osLabel: { zh: 'Android', en: 'Android' },
        icon: 'android',
        assets: [a('arm64', { zh: 'ARM64 + ARMv7', en: 'ARM64 + ARMv7' }, 'songloft-bundled-android-arm64-v8a.apk')],
      },
      {
        os: 'ios',
        osLabel: { zh: 'iOS', en: 'iOS' },
        icon: 'apple',
        assets: [a('arm64', { zh: 'ARM64', en: 'ARM64' }, 'songloft-bundled-ios-arm64.ipa')],
      },
      {
        os: 'macos',
        osLabel: { zh: 'macOS', en: 'macOS' },
        icon: 'apple',
        assets: [a('arm64', { zh: 'Apple Silicon', en: 'Apple Silicon' }, 'songloft-bundled-macos-arm64.dmg')],
      },
      {
        os: 'windows',
        osLabel: { zh: 'Windows', en: 'Windows' },
        icon: 'windows',
        assets: [a('amd64', { zh: 'x86_64', en: 'x86_64' }, 'songloft-bundled-windows-x64.zip')],
      },
      {
        os: 'linux',
        osLabel: { zh: 'Linux', en: 'Linux' },
        icon: 'linux',
        assets: [a('amd64', { zh: 'x86_64', en: 'x86_64' }, 'songloft-bundled-linux-x64.tar.gz')],
      },
    ],
  },
  {
    id: 'flutter',
    label: { zh: 'Flutter 客户端', en: 'Flutter Client' },
    tagline: { zh: '六端原生 · 后台播放 / 媒体控制', en: 'Native on 6 platforms' },
    icon: 'smartphone',
    kind: 'external',
    note: {
      zh: 'Windows 用户也可用 Scoop 一键安装 / 更新，见下方命令。',
      en: 'Windows users can install & update in one line via Scoop — see the commands below.',
    },
    external: [
      {
        label: { zh: '标准版 Releases', en: 'Standalone releases' },
        url: 'https://github.com/songloft-org/songloft-player/releases/latest',
        primary: true,
      },
      {
        label: { zh: '源码仓库', en: 'Source repo' },
        url: 'https://github.com/songloft-org/songloft-player',
      },
    ],
    commands: [
      {
        group: { zh: 'Windows · Scoop', en: 'Windows · Scoop' },
        title: { zh: '添加 Bucket 并安装', en: 'Add bucket and install' },
        code:
          'scoop bucket add songloft https://github.com/songloft-org/songloft-scoop\n' +
          'scoop install songloft-player    # Flutter 客户端（GUI，需连接服务器）\n' +
          'scoop install songloft-server    # 服务端（CLI，仅命令行程序）\n' +
          'scoop install songloft-bundled   # Bundle 版（服务端 + 客户端一体化）',
      },
      {
        group: { zh: 'Windows · Scoop', en: 'Windows · Scoop' },
        title: { zh: '更新 / 卸载', en: 'Update / uninstall' },
        code:
          'scoop update songloft-player     # 其余同理\n' +
          'scoop uninstall songloft-player  # 加 --purge 一并删除配置数据',
      },
      {
        group: { zh: 'Windows · Scoop', en: 'Windows · Scoop' },
        title: {
          zh: '⚠️ 注意：songloft-bundled 与 songloft-server、songloft-player 不能同时安装',
          en: '⚠️ Note: songloft-bundled conflicts with songloft-server and songloft-player',
        },
        code:
          '# songloft-bundled 与 songloft-server、songloft-player 存在文件冲突，不能同时安装\n' +
          '# songloft-server 与 songloft-player 可以共存',
      },
    ],
  },
  {
    id: 'lynx',
    label: { zh: 'Lynx 客户端', en: 'Lynx Client' },
    tagline: { zh: '预览版 · Android / iOS / HarmonyOS / Web', en: 'Preview · Android / iOS / HarmonyOS / Web' },
    icon: 'smartphone',
    kind: 'external',
    note: {
      zh: '需连接服务器，无 Bundle 或桌面版。iOS IPA 需自行重签；HarmonyOS 为实验性支持。dev 仅在五包全部成功后更新，首次发版前可能没有正式包。Web 核心流程支持普通 HTTP，不强制 COOP/COEP；剪贴板和常亮等能力需安全上下文，详见安装指南。',
      en: 'Requires a server; no Bundle or desktop edition. iOS IPA needs re-signing; HarmonyOS is experimental. Dev updates only after all five packages succeed; stable packages may not exist yet. Core Web flows support ordinary HTTP without mandatory COOP/COEP; clipboard and wake-lock features need a secure context. See the installation guide.',
    },
    external: [
      {
        label: { zh: '开发版下载', en: 'Development downloads' },
        url: 'https://github.com/songloft-org/songloft-player-lynx/releases/tag/dev',
        primary: true,
      },
      {
        label: { zh: '正式版本', en: 'Stable releases' },
        url: 'https://github.com/songloft-org/songloft-player-lynx/releases/latest',
      },
      {
        label: { zh: '安装指南', en: 'Installation guide' },
        url: { zh: '/player-lynx/installation', en: '/en/player-lynx/installation' },
      },
      {
        label: { zh: '源码仓库', en: 'Source repository' },
        url: 'https://github.com/songloft-org/songloft-player-lynx',
      },
    ],
  },
  {
    id: 'desktop',
    label: { zh: 'Electron 桌面客户端', en: 'Electron Desktop Client' },
    tagline: { zh: '官方客户端 · Windows / macOS / Linux', en: 'Official client · Windows / macOS / Linux' },
    icon: 'monitor',
    kind: 'external',
    note: {
      zh: 'Songloft Desktop 需连接 Songloft 服务端，不含 Bundle 本地后端。各平台安装方式以仓库说明和 Release 产物为准。',
      en: 'Songloft Desktop requires a Songloft server and does not include a bundled local backend. See the repository instructions and Release assets for installation on each platform.',
    },
    external: [
      {
        label: { zh: '下载桌面客户端', en: 'Download desktop client' },
        url: 'https://github.com/songloft-org/songloft-desktop/releases/latest',
        primary: true,
      },
      {
        label: { zh: '源码仓库', en: 'Source repository' },
        url: 'https://github.com/songloft-org/songloft-desktop',
      },
      {
        label: { zh: '问题反馈', en: 'Report issues' },
        url: 'https://github.com/songloft-org/songloft-desktop/issues',
      },
    ],
  },
  {
    id: 'tv',
    label: { zh: 'TV 客户端', en: 'TV Client' },
    tagline: { zh: 'Android TV · 遥控器操作', en: 'Android TV · remote-friendly' },
    icon: 'tv',
    kind: 'external',
    external: [
      {
        label: { zh: '下载 TV 客户端', en: 'Download TV client' },
        url: 'https://github.com/songloft-org/songloft-tv/releases/latest',
        primary: true,
      },
      {
        label: { zh: '源码仓库', en: 'Source repo' },
        url: 'https://github.com/songloft-org/songloft-tv',
      },
    ],
  },
  {
    id: 'kodi',
    label: { zh: 'Kodi 插件', en: 'Kodi Add-on' },
    tagline: { zh: '大屏 / 客厅 · 遥控器优化', en: 'TV / living room' },
    icon: 'tv',
    kind: 'external',
    external: [
      {
        label: { zh: '下载插件', en: 'Download add-on' },
        url: 'https://github.com/songloft-org/plugin.audio.songloft/releases/latest',
        primary: true,
      },
    ],
  },
]
