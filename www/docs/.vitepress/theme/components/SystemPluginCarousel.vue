<script setup lang="ts">
import { computed } from "vue";
import { useData, withBase } from "vitepress";

type SystemPlugin = {
  key: string;
  href: string;
  title: {
    en: string;
    zh: string;
  };
  description: {
    en: string;
    zh: string;
  };
};

const plugins: SystemPlugin[] = [
  {
    key: "app",
    href: "/guide/plugins/system/application.html",
    title: { en: "Application", zh: "应用" },
    description: { en: "Launch apps and switch windows.", zh: "启动应用并切换窗口。" },
  },
  {
    key: "file",
    href: "/guide/plugins/system/file.html",
    title: { en: "File Search", zh: "文件搜索" },
    description: { en: "Find local files and folders.", zh: "搜索本地文件和文件夹。" },
  },
  {
    key: "clipboard",
    href: "/guide/plugins/system/clipboard.html",
    title: { en: "Clipboard", zh: "剪贴板" },
    description: { en: "Recall copied text and images.", zh: "找回复制过的文本和图片。" },
  },
  {
    key: "calculator",
    href: "/guide/plugins/system/calculator.html",
    title: { en: "Calculator", zh: "计算器" },
    description: { en: "Evaluate expressions in the launcher.", zh: "在启动器里计算表达式。" },
  },
  {
    key: "converter",
    href: "/guide/plugins/system/converter.html",
    title: { en: "Converter", zh: "转换器" },
    description: { en: "Units, currencies, dates, and time zones.", zh: "单位、货币、日期和时区转换。" },
  },
  {
    key: "websearch",
    href: "/guide/plugins/system/websearch.html",
    title: { en: "Web Search", zh: "网页搜索" },
    description: { en: "Search the web from Wox.", zh: "从 Wox 搜索网页。" },
  },
  {
    key: "folder",
    href: "/guide/plugins/system/folder.html",
    title: { en: "Folder", zh: "文件夹" },
    description: { en: "Browse folders by path.", zh: "按路径浏览文件夹。" },
  },
  {
    key: "url",
    href: "/guide/plugins/system/url.html",
    title: { en: "URL", zh: "网址" },
    description: { en: "Open typed URLs immediately.", zh: "直接打开输入的网址。" },
  },
  {
    key: "notes",
    href: "/features/notes.html",
    title: { en: "Notes", zh: "笔记" },
    description: { en: "Floating notes with lists and images.", zh: "带清单和图片的浮动笔记。" },
  },
  {
    key: "screenshot",
    href: "/features/screenshot.html",
    title: { en: "Screenshot", zh: "截图" },
    description: { en: "Capture a screen area.", zh: "截取屏幕区域。" },
  },
  {
    key: "timer",
    href: "/features/timer.html",
    title: { en: "Timer", zh: "计时器" },
    description: { en: "Start countdown timers from the launcher.", zh: "从启动器开始倒计时。" },
  },
  {
    key: "dictation",
    href: "/features/dictation.html",
    title: { en: "Dictation", zh: "听写" },
    description: { en: "Turn speech into text.", zh: "将语音转为文字。" },
  },
  {
    key: "window-layouts",
    href: "/features/window-layouts.html",
    title: { en: "Window layouts", zh: "窗口布局" },
    description: { en: "Arrange windows and restore layouts.", zh: "排列窗口并恢复布局。" },
  },
  {
    key: "bookmark",
    href: "/guide/plugins/system/browser-bookmark.html",
    title: { en: "Browser Bookmark", zh: "浏览器书签" },
    description: { en: "Find and open bookmarks.", zh: "搜索并打开书签。" },
  },
  {
    key: "browser",
    href: "/guide/plugins/system/browser.html",
    title: { en: "Browser", zh: "浏览器" },
    description: { en: "Jump to open browser tabs.", zh: "跳转到已打开的标签页。" },
  },
  {
    key: "quickjump",
    href: "/guide/plugins/system/explorer.html",
    title: { en: "Quick Jump", zh: "快速跳转" },
    description: { en: "Jump to folders in file dialogs.", zh: "在文件对话框中快速跳转。" },
  },
  {
    key: "emoji",
    href: "/guide/plugins/system/emoji.html",
    title: { en: "Emoji", zh: "Emoji" },
    description: { en: "Search and copy emoji.", zh: "搜索并复制 Emoji。" },
  },
  {
    key: "color",
    href: "/guide/plugins/system/color.html",
    title: { en: "Color", zh: "颜色" },
    description: { en: "Preview, copy, and save colors.", zh: "预览、复制并保存颜色。" },
  },
  {
    key: "mediaplayer",
    href: "/guide/plugins/system/mediaplayer.html",
    title: { en: "Media Player", zh: "媒体播放器" },
    description: { en: "Control what's playing now.", zh: "控制当前播放的媒体。" },
  },
  {
    key: "webview",
    href: "/guide/plugins/system/webview.html",
    title: { en: "WebView", zh: "网页预览" },
    description: { en: "Preview sites inside Wox.", zh: "在 Wox 内预览网页。" },
  },
  {
    key: "chat",
    href: "/guide/plugins/system/chat.html",
    title: { en: "AI Chat", zh: "AI 对话" },
    description: { en: "Talk to models and agents.", zh: "与模型和 Agent 对话。" },
  },
  {
    key: "ai-commands",
    href: "/guide/ai/commands.html",
    title: { en: "AI Commands", zh: "AI 命令" },
    description: { en: "Run reusable AI commands.", zh: "运行可复用的 AI 命令。" },
  },
  {
    key: "shell",
    href: "/guide/plugins/system/shell.html",
    title: { en: "Shell", zh: "Shell" },
    description: { en: "Run shell commands from Wox.", zh: "在 Wox 中执行命令。" },
  },
  {
    key: "sys",
    href: "/guide/plugins/system/sys.html",
    title: { en: "System Commands", zh: "系统命令" },
    description: { en: "Shutdown, lock, and system settings.", zh: "关机、锁定和系统设置。" },
  },
  {
    key: "theme",
    href: "/guide/plugins/system/theme.html",
    title: { en: "Theme", zh: "主题" },
    description: { en: "Switch themes or generate new ones.", zh: "切换主题或用 AI 生成。" },
  },
  {
    key: "wpm",
    href: "/guide/plugins/system/wpm.html",
    title: { en: "Plugin Manager", zh: "插件管理器" },
    description: { en: "Install and create plugins.", zh: "安装和创建插件。" },
  },
  {
    key: "query-history",
    href: "/guide/plugins/system/query-history.html",
    title: { en: "Query History", zh: "查询历史" },
    description: { en: "Revisit recent searches.", zh: "回顾最近的查询。" },
  },
  {
    key: "hotkeys",
    href: "/guide/plugins/system/hotkey-overview.html",
    title: { en: "Hotkeys", zh: "快捷键" },
    description: { en: "See every Wox shortcut.", zh: "查看所有 Wox 快捷键。" },
  },
  {
    key: "doctor",
    href: "/guide/plugins/system/doctor.html",
    title: { en: "Doctor", zh: "诊断" },
    description: { en: "Check system and Wox settings.", zh: "检查系统和 Wox 设置。" },
  },
  {
    key: "feedback",
    href: "/guide/plugins/system/feedback.html",
    title: { en: "Feedback", zh: "反馈" },
    description: { en: "Send diagnostics and feedback.", zh: "发送诊断信息和反馈。" },
  },
  {
    key: "update",
    href: "/guide/plugins/system/update.html",
    title: { en: "Update", zh: "更新" },
    description: { en: "Check and install updates.", zh: "检查并安装更新。" },
  },
  {
    key: "backup",
    href: "/guide/plugins/system/backup.html",
    title: { en: "Backup", zh: "备份" },
    description: { en: "Backup and restore settings.", zh: "备份和恢复设置。" },
  },
  {
    key: "cloudsync",
    href: "/guide/plugins/system/cloudsync.html",
    title: { en: "Cloud Sync", zh: "云同步" },
    description: { en: "Sync settings across devices.", zh: "跨设备同步设置。" },
  },
];

const { lang } = useData();
const isZh = computed(() => (lang.value || "").toLowerCase().startsWith("zh"));

function localize(value: { en: string; zh: string }) {
  return isZh.value ? value.zh : value.en;
}
</script>

<template>
  <section class="wox-section system-plugins">
    <div class="system-plugin-heading">
      <h2>{{ isZh ? "常用工具，开箱即用。" : "The essentials, built in." }}</h2>
      <p>{{ isZh ? "从打开应用到截图、笔记和 AI，都在 Wox 里完成。" : "Apps, files, notes, screenshots, AI, and more. All inside Wox." }}</p>
    </div>
    <div class="system-plugin-list">
      <a v-for="plugin in plugins" :key="plugin.key" :href="withBase(isZh ? `/zh${plugin.href}` : plugin.href)" class="system-plugin-item">
        <h3>{{ localize(plugin.title) }}</h3>
        <p>{{ localize(plugin.description) }}</p>
      </a>
    </div>
  </section>
</template>

<style scoped>
.system-plugins {
  border-top: 1px solid var(--wox-home-border);
  padding-top: 48px;
}

.system-plugin-heading p {
  margin: 12px 0 0;
}

.system-plugin-list {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0 24px;
  margin-top: 20px;
}

.system-plugin-item {
  display: block;
  padding: 12px 0;
  border-bottom: 1px solid var(--wox-home-border);
  text-decoration: none;
}

.system-plugin-item h3 {
  margin: 0;
  color: var(--vp-c-text-1);
  font-size: 15px;
  font-weight: 600;
  line-height: 1.3;
}

.system-plugin-item p {
  margin: 4px 0 0;
  font-size: 13px;
  line-height: 1.45;
}

.system-plugin-item:hover h3 {
  text-decoration: underline;
  text-underline-offset: 4px;
}

@media (max-width: 960px) {
  .system-plugin-list {
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 0 20px;
  }
}

@media (max-width: 720px) {
  .system-plugin-list {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0 16px;
  }
}

@media (max-width: 540px) {
  .system-plugin-list {
    grid-template-columns: 1fr;
    margin-top: 12px;
  }
}
</style>
