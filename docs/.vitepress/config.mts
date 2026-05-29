import { defineConfig } from 'vitepress'
import AutoSidebar from 'vite-plugin-vitepress-auto-sidebar';
import taskLists from 'markdown-it-task-lists'

export default async () => {
  return defineConfig({
    title: "Songloft",
    description: "Songloft - 自托管个人音乐服务器，支持 JS 插件扩展，跨平台 Flutter 客户端",
    // repowiki 是 Qoder IDE 自动生成的中文 wiki，含大量 <cite>/泛型尖括号等占位符，
    // 不适合 VitePress 直接编译（会触发 Vue 模板未闭合标签错误）；保留在仓库里供 GitHub 浏览即可。
    srcExclude: ['repowiki/**'],
    themeConfig: {
      // https://vitepress.dev/reference/default-theme-config
      nav: [
        { text: '快速开始', link: '/quick-start' },
        { text: '客户端', link: '/issues/8' },
        { text: '插件', link: '/issues/4' },
        { text: 'FAQ', link: '/faq' },
        { text: '更新日志', link: '/changelog' },
        { text: 'API 文档', link: 'https://petstore.swagger.io/?url=https://raw.githubusercontent.com/songloft-org/songloft/refs/heads/main/docs/swagger.json' },
      ],

      socialLinks: [
        { icon: 'github', link: 'https://github.com/songloft-org/songloft' }
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
        text: '在 GitHub 上提问'
      },
    },
    sitemap: {
      hostname: 'https://songloft.hanxi.cc'
    },
    head: [
      ['meta', { name: 'og:type', content: 'website' }],
      ['meta', { name: 'og:title', content: 'Songloft - 自托管个人音乐服务器' }],
      ['meta', { name: 'og:description', content: '简单、自由、插件化的个人音乐服务器，支持 JS 插件扩展' }],
    ],
    lastUpdated: true,
    markdown: {
      lineNumbers: false, // 关闭代码块行号显示
      // 自定义 markdown-it 插件
      config: (md) => {
        md.use(taskLists)
        md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
          const aIndex = tokens[idx].attrIndex('target');
          if (aIndex < 0) {
            tokens[idx].attrPush(['target', '_self']); // 将默认行为改为不使用 _blank
          } else {
            tokens[idx].attrs![aIndex][1] = '_self'; // 替换 _blank 为 _self
          }
          return self.renderToken(tokens, idx, options);
        };
      },
    },
    logLevel: 'warn',
    vite: {
      plugins: [
        AutoSidebar({
          path: '.',
          collapsed: true,
          titleFromFile: true,
          ignoreList: ['repowiki'],
        }),
      ],
    }
  })
}
