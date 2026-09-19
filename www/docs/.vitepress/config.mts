import { defineConfig } from "vitepress";
import { generateChangelogPages, getLatestRelease } from "../../scripts/release-meta.mjs";
import { applySeo } from "./seo";

generateChangelogPages();

const latestRelease = getLatestRelease();

export default defineConfig({
  // Custom domain https://www.woxlauncher.com/ serves this project site at the
  // domain root, so assets must not keep the old /Wox/ GitHub Pages prefix.
  base: "/",
  appearance: { initialValue: "light" },
  title: "Wox",
  titleTemplate: ":title | Wox",
  description: "A native, open-source launcher for Windows, macOS, and Linux.",
  lastUpdated: false,
  markdown: {
    anchor: {
      permalink: false,
    },
  },
  sitemap: {
    hostname: "https://www.woxlauncher.com",
  },
  transformPageData(pageData) {
    applySeo(pageData);
    if (!keepsGuideChrome(pageData.relativePath)) {
      pageData.frontmatter.aside = false;
      pageData.frontmatter.outline = false;
    }
  },
  vite: {
    define: {
      __WOX_LATEST_RELEASE__: JSON.stringify(latestRelease),
    },
  },

  locales: {
    root: {
      label: "English",
      lang: "en-US",
      title: "Wox",
      description: "A native, open-source launcher for Windows, macOS, and Linux.",
      themeConfig: {
        nav: [
          { text: "Home", link: "/" },
          { text: "Guide", link: "/guide/introduction" },
          { text: "Changelog", link: "/changelog/" },
          { text: "Development", link: "/development/" },
          { text: "Blog", link: "/blog/" },
          { text: "Plugin Store", link: "/store/plugins" },
          { text: "Theme Store", link: "/store/themes" },
        ],
        sidebar: {
          "/guide/": englishGuideSidebar(),
          "/compare/": englishGuideSidebar(),
          "/features/": englishGuideSidebar(),
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
      description: "适用于 Windows、macOS 和 Linux 的原生开源启动器。",
      themeConfig: {
        nav: [
          { text: "首页", link: "/zh/" },
          { text: "指南", link: "/zh/guide/introduction" },
          { text: "更新日志", link: "/zh/changelog/" },
          { text: "开发", link: "/zh/development/" },
          { text: "博客", link: "/zh/blog/" },
          { text: "插件商店", link: "/zh/store/plugins" },
          { text: "主题商店", link: "/zh/store/themes" },
        ],
        sidebar: {
          "/zh/guide/": chineseGuideSidebar(),
          "/zh/compare/": chineseGuideSidebar(),
          "/zh/features/": chineseGuideSidebar(),
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
        langMenuLabel: "多语言",
        returnToTopLabel: "回到顶部",
        sidebarMenuLabel: "菜单",
        darkModeSwitchLabel: "主题",
        lightModeSwitchTitle: "切换到浅色模式",
        darkModeSwitchTitle: "切换到深色模式",
      },
    },
  },

  // Google Search only shows a site icon when the homepage exposes a crawlable
  // square favicon that is a multiple of 48px. The docs site previously shipped
  // none, so results fell back to the generic globe.
  head: [
    ["link", { rel: "icon", type: "image/png", sizes: "48x48", href: "/favicon-48x48.png" }],
    ["link", { rel: "icon", type: "image/png", sizes: "192x192", href: "/favicon-192x192.png" }],
    ["link", { rel: "icon", href: "/favicon.ico", sizes: "32x32" }],
    ["link", { rel: "apple-touch-icon", sizes: "180x180", href: "/apple-touch-icon.png" }],
  ],

  themeConfig: {
    logo: "/logo.png",
    socialLinks: [{ icon: "github", link: "https://github.com/Wox-launcher/Wox" }],
    search: {
      provider: "local",
    },
  },
});

function englishGuideSidebar() {
  return [
    {
      text: "Guide",
      items: [
        { text: "Introduction", link: "/guide/introduction" },
        { text: "Installation", link: "/guide/installation" },
        { text: "FAQ", link: "/guide/faq" },
        { text: "Compare", link: "/compare/" },
      ],
    },
    {
      text: "Usage",
      items: [
        { text: "Querying", link: "/guide/usage/querying" },
        { text: "Action Panel", link: "/guide/usage/action-panel" },
        { text: "Hotkeys", link: "/guide/usage/hotkeys" },
        { text: "Preview", link: "/guide/usage/preview" },
        { text: "Deep Link", link: "/guide/usage/deep-link" },
      ],
    },
    {
      text: "Features",
      items: [
        {
          text: "System Plugins",
          items: [
            { text: "Application", link: "/guide/plugins/system/application" },
            { text: "Calculator", link: "/guide/plugins/system/calculator" },
            { text: "Converter", link: "/guide/plugins/system/converter" },
            { text: "Web Search", link: "/guide/plugins/system/websearch" },
            { text: "Clipboard", link: "/guide/plugins/system/clipboard" },
            { text: "File Search", link: "/guide/plugins/system/file" },
            { text: "Folder", link: "/guide/plugins/system/folder" },
            { text: "URL", link: "/guide/plugins/system/url" },
            { text: "Notes", link: "/features/notes" },
            { text: "Screenshot", link: "/features/screenshot" },
            { text: "Timer", link: "/features/timer" },
            { text: "Dictation", link: "/features/dictation" },
            { text: "Window layouts", link: "/features/window-layouts" },
            { text: "Browser Bookmark", link: "/guide/plugins/system/browser-bookmark" },
            { text: "Browser", link: "/guide/plugins/system/browser" },
            { text: "Quick Jump", link: "/guide/plugins/system/explorer" },
            { text: "Emoji", link: "/guide/plugins/system/emoji" },
            { text: "Color", link: "/guide/plugins/system/color" },
            { text: "Media Player", link: "/guide/plugins/system/mediaplayer" },
            { text: "WebView", link: "/guide/plugins/system/webview" },
            { text: "Plugin Manager", link: "/guide/plugins/system/wpm" },
            { text: "Theme", link: "/guide/plugins/system/theme" },
            { text: "Shell", link: "/guide/plugins/system/shell" },
            { text: "System Commands", link: "/guide/plugins/system/sys" },
            { text: "Query History", link: "/guide/plugins/system/query-history" },
            { text: "Hotkeys", link: "/guide/plugins/system/hotkey-overview" },
            { text: "Doctor", link: "/guide/plugins/system/doctor" },
            { text: "Feedback", link: "/guide/plugins/system/feedback" },
            { text: "Update", link: "/guide/plugins/system/update" },
            { text: "Backup", link: "/guide/plugins/system/backup" },
            { text: "Cloud Sync", link: "/guide/plugins/system/cloudsync" },
          ],
        },
        {
          text: "AI",
          items: [
            { text: "Settings", link: "/guide/ai/settings" },
            { text: "Chat", link: "/guide/plugins/system/chat" },
            { text: "Commands", link: "/guide/ai/commands" },
            { text: "Theme generation", link: "/guide/ai/theme" },
          ],
        },
      ],
    },
  ];
}

function chineseGuideSidebar() {
  return [
    {
      text: "指南",
      items: [
        { text: "简介", link: "/zh/guide/introduction" },
        { text: "安装", link: "/zh/guide/installation" },
        { text: "常见问题", link: "/zh/guide/faq" },
        { text: "对比", link: "/zh/compare/" },
      ],
    },
    {
      text: "使用",
      items: [
        { text: "查询", link: "/zh/guide/usage/querying" },
        { text: "操作面板", link: "/zh/guide/usage/action-panel" },
        { text: "快捷键", link: "/zh/guide/usage/hotkeys" },
        { text: "预览", link: "/zh/guide/usage/preview" },
        { text: "深度链接", link: "/zh/guide/usage/deep-link" },
      ],
    },
    {
      text: "功能",
      items: [
        {
          text: "系统插件",
          items: [
            { text: "应用", link: "/zh/guide/plugins/system/application" },
            { text: "计算器", link: "/zh/guide/plugins/system/calculator" },
            { text: "转换器", link: "/zh/guide/plugins/system/converter" },
            { text: "网页搜索", link: "/zh/guide/plugins/system/websearch" },
            { text: "剪贴板", link: "/zh/guide/plugins/system/clipboard" },
            { text: "文件搜索", link: "/zh/guide/plugins/system/file" },
            { text: "文件夹", link: "/zh/guide/plugins/system/folder" },
            { text: "网址", link: "/zh/guide/plugins/system/url" },
            { text: "笔记", link: "/zh/features/notes" },
            { text: "截图", link: "/zh/features/screenshot" },
            { text: "计时器", link: "/zh/features/timer" },
            { text: "听写", link: "/zh/features/dictation" },
            { text: "窗口布局", link: "/zh/features/window-layouts" },
            { text: "浏览器书签", link: "/zh/guide/plugins/system/browser-bookmark" },
            { text: "浏览器", link: "/zh/guide/plugins/system/browser" },
            { text: "快速跳转", link: "/zh/guide/plugins/system/explorer" },
            { text: "Emoji", link: "/zh/guide/plugins/system/emoji" },
            { text: "颜色", link: "/zh/guide/plugins/system/color" },
            { text: "媒体播放器", link: "/zh/guide/plugins/system/mediaplayer" },
            { text: "网页预览", link: "/zh/guide/plugins/system/webview" },
            { text: "插件管理器", link: "/zh/guide/plugins/system/wpm" },
            { text: "主题", link: "/zh/guide/plugins/system/theme" },
            { text: "Shell", link: "/zh/guide/plugins/system/shell" },
            { text: "系统命令", link: "/zh/guide/plugins/system/sys" },
            { text: "查询历史", link: "/zh/guide/plugins/system/query-history" },
            { text: "快捷键", link: "/zh/guide/plugins/system/hotkey-overview" },
            { text: "诊断", link: "/zh/guide/plugins/system/doctor" },
            { text: "反馈", link: "/zh/guide/plugins/system/feedback" },
            { text: "更新", link: "/zh/guide/plugins/system/update" },
            { text: "备份", link: "/zh/guide/plugins/system/backup" },
            { text: "云同步", link: "/zh/guide/plugins/system/cloudsync" },
          ],
        },
        {
          text: "AI",
          items: [
            { text: "设置", link: "/zh/guide/ai/settings" },
            { text: "对话", link: "/zh/guide/plugins/system/chat" },
            { text: "命令", link: "/zh/guide/ai/commands" },
            { text: "主题生成", link: "/zh/guide/ai/theme" },
          ],
        },
      ],
    },
  ];
}

function keepsGuideChrome(relativePath: string): boolean {
  const pagePath = relativePath.startsWith("zh/") ? relativePath.slice(3) : relativePath;
  return pagePath.startsWith("guide/") || pagePath.startsWith("compare/") || pagePath.startsWith("features/");
}
