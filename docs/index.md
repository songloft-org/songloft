---
# https://vitepress.dev/reference/default-theme-home-page
layout: home

hero:
  name: "Songloft"
  text: "自托管个人音乐服务器"
  tagline: 数据自主 · 插件化扩展 · 仅管理你合法拥有的音乐
  actions:
    - theme: brand
      text: 快速开始
      link: /quick-start
    - theme: alt
      text: 客户端下载
      link: /issues/8
    - theme: alt
      text: 插件合集
      link: /issues/4

features:
  - icon: 🆓
    title: 完全免费开源
    details: Apache-2.0 协议，数据留在自己的设备上，自主可控，无需担心隐私泄露
  - icon: 🚀
    title: 一键部署
    details: 支持 Docker 一键部署，兼容各大 NAS 平台（群晖、威联通等），也支持直接运行二进制文件，快速上线
  - icon: 🧩
    title: JS 插件体系（沙箱隔离）
    details: 基于 QuickJS 沙箱运行 JavaScript 插件，权限模型 / 热更新 / 健康检查齐备，可自由扩展音源、元数据、设备控制等能力
  - icon: 📱
    title: 跨平台客户端
    details: 提供基于 Flutter 的跨平台客户端，支持 Android、iOS、macOS、Windows、Linux 和 Web，一套代码六端运行
  - icon: 🎵
    title: 丰富音频格式支持
    details: 支持 MP3、FLAC、WAV、APE、OGG、M4A 等主流音频格式，自动提取专辑封面和歌曲元数据
  - icon: 📡
    title: 网络歌曲与电台
    details: 在本地音乐之外，支持添加网络歌曲与网络电台，与本地库统一管理与播放
  - icon: 🔁
    title: 歌单转本地
    details: 把歌单中的网络歌曲离线下载到本地音乐库，按歌单分目录、自动回写元数据 / 封面 / 歌词（请确保拥有合法使用权）
  - icon: 🔑
    title: JWT 双 Token 认证
    details: Access Token + Refresh Token 双 Token 机制，支持多设备登录与统一管理
  - icon: ⚡
    title: Go 编写 · 轻量高效
    details: 使用 Go 构建，CGO-free 无 C 依赖，资源占用极低，启动快，适合在 NAS、迷你主机、树莓派等低功耗设备上运行
  - icon: 🔌
    title: 完整 REST API
    details: 提供完整的 RESTful API，内置 Swagger 文档，方便集成和二次开发
---
