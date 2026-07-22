import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'
import AutoSidebar from 'vite-plugin-vitepress-auto-sidebar';
import taskLists from 'markdown-it-task-lists'

export default async () => {
  return withMermaid(defineConfig({
    title: "Songloft",
    description: "Songloft - 自托管个人音乐服务器，支持 JS 插件扩展，跨平台 Flutter 客户端",
    lang: 'zh-Hans',

    // 中英双语：中文为根（/），英文在 /en/。目前仅落地页（index.md / en/index.md）
    // 做了双语；文档正文暂保持中文，英文导航指向中文文档（后续再补译）。
    locales: {
      root: {
        label: '简体中文',
        lang: 'zh-Hans',
      },
      en: {
        label: 'English',
        lang: 'en',
        link: '/en/',
        themeConfig: {
          nav: [
            { text: 'Get Started', link: '/en/quick-start' },
            { text: 'Client', link: 'https://github.com/songloft-org/songloft/releases/latest' },
            {
              text: 'Plugins',
              items: [
                { text: 'Plugin List', link: 'https://github.com/songloft-org/songloft/issues/4' },
                { text: 'Plugin Dev Guide', link: '/en/js-plugin-development-guide' },
                { text: 'Plugin Registry Guide', link: '/en/plugin_registry' },
              ],
            },
            { text: 'RepoWiki', link: '/en/repowiki/项目概述' },
            { text: 'FAQ', link: '/en/faq' },
            { text: 'Changelog', link: 'https://github.com/songloft-org/songloft/releases' },
            {
              text: 'More',
              items: [
                { text: 'API Docs', link: '/swagger-api/' },
                { text: 'Docker Hub', link: 'https://hub.docker.com/r/songloft/songloft' },
                { text: 'Privacy', link: '/en/PRIVACY' },
                { text: 'NOTICE', link: '/en/NOTICE' },
              ],
            },
          ],
        },
      },
    },

    head: [
      ['link', { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' }],
      ['meta', { property: 'og:type', content: 'website' }],
      ['meta', { property: 'og:title', content: 'Songloft - 自托管个人音乐服务器' }],
      ['meta', { property: 'og:description', content: '简单、自由、插件化的个人音乐服务器，支持 JS 插件扩展' }],
      ['meta', { property: 'og:image', content: 'https://songloft.hanxi.cc/logo.png' }],
    ],

    themeConfig: {
      logo: '/logo.png',

      nav: [
        { text: '快速开始', link: '/quick-start' },
        { text: '客户端', link: '/issues/8' },
        {
          text: '插件',
          items: [
            { text: '插件列表', link: '/issues/4' },
            { text: '插件开发指南', link: '/js-plugin-development-guide' },
            { text: '插件源制作指南', link: '/plugin_registry' },
            { text: '插件工具链', link: '/plugin-toolchain/' },
          ],
        },
        { text: '源码解析', link: '/repowiki/项目概述' },
        {
          text: '子项目',
          items: [
            { text: 'Flutter 客户端', link: '/player/architecture' },
            { text: 'Tracely 可观测', link: '/tracely/' },
            { text: 'HA 加载项', link: '/addon/' },
          ],
        },
        { text: 'FAQ', link: '/faq' },
        { text: '更新日志', link: '/changelog' },
        {
          text: '更多',
          items: [
            { text: 'API 文档', link: '/swagger-api/' },
            { text: 'Docker Hub', link: 'https://hub.docker.com/r/songloft/songloft' },
            { text: '隐私说明', link: '/PRIVACY' },
            { text: 'NOTICE', link: '/NOTICE' },
          ],
        },
      ],

      socialLinks: [
        { icon: 'github', link: 'https://github.com/songloft-org/songloft' },
      ],

      footer: {
        message: '基于 <a href="https://github.com/songloft-org/songloft/blob/main/LICENSE">Apache 2.0</a> 协议开源',
        copyright: `Copyright © 2025-${new Date().getFullYear()} <a href="https://github.com/hanxi">涵曦</a>`,
      },

      search: {
        provider: 'local',
      },

      editLink: {
        pattern: 'https://github.com/songloft-org/songloft/issues',
        text: '在 GitHub 上提问',
      },
    },

    sitemap: {
      hostname: 'https://songloft.hanxi.cc',
    },

    ignoreDeadLinks: [/^https?:\/\/localhost/],

    lastUpdated: true,

    markdown: {
      lineNumbers: false,
      // 关闭 attrs 语法：repowiki 的 REST 标题（如 `### GET /playlists/{id}`）
      // 结尾的 `{id}` 会被 attrs 当成空的自定义锚点导致 id 冲突。全站未使用 {#锚点}/{.class} 语法。
      attrs: { disable: true },
      config: (md) => {
        md.use(taskLists)
      },
    },

    vite: {
      plugins: [
        AutoSidebar({
          path: '.',
          collapsed: true,
          titleFromFile: true,
          ignoreIndexItem: true, // 首页 index.md（落地页）不进侧边栏
        }),
      ],
    },
  }))
}
