---
# https://vitepress.dev/reference/default-theme-home-page
layout: home

hero:
  name: "MiMusic"
  text: "自托管个人音乐服务器"
  tagline: 简单，自由，插件化 · 数据自主，随时随地享受音乐
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
    - theme: alt
      text: 在线演示（账号/密码：admin）
      link: https://examplemimusic.hanxi.cc

features:
  - icon: 🆓
    title: 完全免费开源
    details: 完全免费，数据留在自己的设备上，自主可控，无需担心隐私泄露
  - icon: 🚀
    title: 一键部署
    details: 支持 Docker 一键部署，兼容各大 NAS 平台（群晖、威联通等），也支持直接运行二进制文件，快速上线
  - icon: 🧩
    title: JS 插件体系
    details: 基于 QuickJS 沙箱运行 JavaScript 插件，按需加载，支持权限模型、热更新与健康检查，可自由扩展音源 / 元数据 / 设备控制等能力
  - icon: 🔒
    title: 插件沙箱隔离
    details: 插件运行在 QuickJS 虚拟机中，边界清晰、权限可控，降低扩展带来的安全风险
  - icon: 📱
    title: 跨平台客户端
    details: 提供基于 Flutter 的跨平台客户端，支持 Android、iOS、macOS、Windows、Linux 和 Web，一套代码六端运行
  - icon: ⚡
    title: Go 编写 · 轻量高效
    details: 使用 Go 构建，CGO-free 无 C 依赖，资源占用极低，启动快，适合在 NAS、迷你主机、树莓派等低功耗设备上运行
  - icon: 🎵
    title: 丰富音频格式支持
    details: 支持 MP3、FLAC、WAV、APE、OGG、M4A 等主流音频格式，自动提取专辑封面和歌曲元数据
  - icon: 🔌
    title: 完整 REST API
    details: 提供完整的 RESTful API，支持 JWT 认证，内置 Swagger 文档，方便集成和二次开发
---
