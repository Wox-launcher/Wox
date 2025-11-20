import { defineConfig } from "vitepress";

export default defineConfig({
  base: "/Wox/",
  title: "Wox",
  description: "A cross-platform quick launcher",

  locales: {
    root: {
      label: "English",
      lang: "en-US",
      title: "Wox",
      description: "A cross-platform quick launcher",
      themeConfig: {
        nav: [
          { text: "Home", link: "/" },
          { text: "Guide", link: "/guide/installation" },
          { text: "Development", link: "/development/setup" },
          { text: "Plugin Store", link: "/store/plugins" },
          { text: "Theme Store", link: "/store/themes" },
        ],
        sidebar: {
          "/guide/": [
            {
              text: "Guide",
              items: [
                { text: "Introduction", link: "/guide/introduction" },
                { text: "Installation", link: "/guide/installation" },
                { text: "FAQ", link: "/guide/faq" },
              ],
            },
            {
              text: "Usage",
              items: [
                { text: "Querying", link: "/guide/usage/querying" },
                { text: "Action Panel", link: "/guide/usage/action-panel" },
                { text: "Deep Link", link: "/guide/usage/deep-link" },
              ],
            },
            {
              text: "AI Features",
              items: [
                { text: "Settings", link: "/guide/ai/settings" },
                { text: "Theme Generation", link: "/guide/ai/theme" },
                { text: "AI Commands", link: "/guide/ai/commands" },
              ],
            },
          ],
          "/development/": [
            {
              text: "Development",
              items: [
                { text: "Setup", link: "/development/setup" },
                { text: "Architecture", link: "/development/architecture" },
                { text: "Contributing", link: "/development/contributing" },
              ],
            },
            {
              text: "Plugin Development",
              items: [
                { text: "Overview", link: "/development/plugins/overview" },
                { text: "Specification", link: "/development/plugins/specification" },
                { text: "Query Model", link: "/development/plugins/query-model" },
                { text: "Script Plugin", link: "/development/plugins/script-plugin" },
                { text: "Full-featured Plugin", link: "/development/plugins/full-featured-plugin" },
              ],
            },
          ],
          "/store/": [
            {
              text: "Store",
              items: [
                { text: "Plugins", link: "/store/plugins" },
                { text: "Themes", link: "/store/themes" },
              ],
            },
          ],
        },
        footer: {
          message: "Released under the GPL-3.0 License.",
          copyright: "Copyright © 2013-present Wox Launcher",
        },
      },
    },
    zh: {
      label: "简体中文",
      lang: "zh-CN",
      link: "/zh/",
      title: "Wox",
      description: "跨平台快速启动器",
      themeConfig: {
        nav: [
          { text: "首页", link: "/zh/" },
          { text: "指南", link: "/zh/guide/installation" },
          { text: "开发", link: "/zh/development/setup" },
          { text: "插件商店", link: "/zh/store/plugins" },
          { text: "主题商店", link: "/zh/store/themes" },
        ],
        sidebar: {
          "/zh/guide/": [
            {
              text: "指南",
              items: [
                { text: "安装", link: "/zh/guide/installation" },
                { text: "常见问题", link: "/zh/guide/faq" },
              ],
            },
            {
              text: "使用",
              items: [
                { text: "查询", link: "/zh/guide/usage/querying" },
                { text: "操作面板", link: "/zh/guide/usage/action-panel" },
                { text: "深度链接", link: "/zh/guide/usage/deep-link" },
              ],
            },
            {
              text: "AI 功能",
              items: [
                { text: "设置", link: "/zh/guide/ai/settings" },
                { text: "主题生成", link: "/zh/guide/ai/theme" },
                { text: "AI 命令", link: "/zh/guide/ai/commands" },
              ],
            },
          ],
          "/zh/development/": [
            {
              text: "开发",
              items: [
                { text: "环境搭建", link: "/zh/development/setup" },
                { text: "架构", link: "/zh/development/architecture" },
                { text: "贡献指南", link: "/zh/development/contributing" },
              ],
            },
            {
              text: "插件开发",
              items: [
                { text: "概览", link: "/zh/development/plugins/overview" },
                { text: "规范", link: "/zh/development/plugins/specification" },
                { text: "查询模型", link: "/zh/development/plugins/query-model" },
                { text: "脚本插件", link: "/zh/development/plugins/script-plugin" },
                { text: "全功能插件", link: "/zh/development/plugins/full-featured-plugin" },
              ],
            },
          ],
          "/zh/store/": [
            {
              text: "商店",
              items: [
                { text: "插件", link: "/zh/store/plugins" },
                { text: "主题", link: "/zh/store/themes" },
              ],
            },
          ],
        },
        footer: {
          message: "基于 GPL-3.0 许可发布",
          copyright: "版权所有 © 2013-至今 Wox Launcher",
        },
        docFooter: {
          prev: "上一页",
          next: "下一页",
        },
        outline: {
          label: "页面导航",
        },
        lastUpdated: {
          text: "最后更新于",
          formatOptions: {
            dateStyle: "short",
            timeStyle: "medium",
          },
        },
        langMenuLabel: "多语言",
        returnToTopLabel: "回到顶部",
        sidebarMenuLabel: "菜单",
        darkModeSwitchLabel: "主题",
        lightModeSwitchTitle: "切换到浅色模式",
        darkModeSwitchTitle: "切换到深色模式",
      },
    },
  },

  themeConfig: {
    socialLinks: [{ icon: "github", link: "https://github.com/Wox-launcher/Wox" }],
    search: {
      provider: "local",
    },
  },
});
