---
# https://vitepress.dev/reference/default-theme-home-page
layout: home

hero:
  name: "Songloft"
  text: "自托管个人音乐服务器"
  tagline: 数据自主 · 插件化扩展 · 仅管理你合法拥有的音乐
  image:
    src: /logo.png
    alt: Songloft
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
  - icon: 🔒
    title: 数据完全自主
    details: 所有音乐、账号、播放记录只存在你的设备上，Apache-2.0 开源，无遥测无广告
  - icon: ⚡
    title: 极致轻量
    details: Go 编写，单文件部署，无 C 依赖，10MB 内存即可运行，NAS / 树莓派 / 迷你主机皆宜
  - icon: 📱
    title: 六端覆盖
    details: Flutter 跨平台客户端一套代码跑 Android、iOS、macOS、Windows、Linux、Web
  - icon: 🧩
    title: 沙箱插件体系
    details: QuickJS 隔离运行，按需扩展音源 / 元数据 / 设备控制，权限模型 + 热更新 + 健康检查
  - icon: 🎵
    title: 主流格式全支持
    details: MP3 / FLAC / WAV / APE / OGG / M4A 自动识别，封面、歌词、元数据一键提取
  - icon: 🔁
    title: 网络歌曲落盘
    details: 一键把网络歌曲离线下载到本地，按歌单分目录，自动回写标签 / 封面 / 歌词
  - icon: 🚀
    title: 一键部署 & 升级
    details: Docker / 二进制两种方式秒启动，容器内支持在线热升级，无需重建
  - icon: 🎨
    title: 沉浸式播放体验
    details: 从专辑封面实时提取配色，每首歌拥有独特视觉氛围，支持后台播放与媒体控制
  - icon: 🔌
    title: 完整 REST API
    details: 内置 Swagger 文档，所有功能均可通过 API 调用，方便集成与二次开发
  - icon: 🔑
    title: 安全认证
    details: JWT 双 Token 机制，支持多设备同时在线，Token 自动刷新无感续期
  - icon: 📡
    title: 网络电台
    details: 在本地音乐之外，支持添加网络电台流，统一播放界面一站式管理
  - icon: 🌐
    title: Web 界面开箱即用
    details: 完整版内置响应式 Web 前端，打开浏览器即可播放，无需安装客户端
---
