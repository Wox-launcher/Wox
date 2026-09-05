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
    description: { en: "Launch apps, open folders, and activate running windows from one result list.", zh: "启动应用、打开所在目录，并从结果列表直接激活已运行窗口。" },
  },
  {
    key: "file",
    href: "/guide/plugins/system/file.html",
    title: { en: "File", zh: "文件" },
    description: { en: "Find local files and folders by name.", zh: "搜索已索引的文件和文件夹，并用操作面板继续处理结果。" },
  },
  {
    key: "clipboard",
    href: "/guide/plugins/system/clipboard.html",
    title: { en: "Clipboard", zh: "剪贴板" },
    description: { en: "Bring back recent text and image clipboard items without leaving the keyboard.", zh: "不用离开键盘就能找回最近的文本和图片剪贴板记录。" },
  },
  {
    key: "calculator",
    href: "/guide/plugins/system/calculator.html",
    title: { en: "Calculator", zh: "计算器" },
    description: { en: "Evaluate expressions directly in the launcher and copy formatted results.", zh: "直接在启动器里计算表达式，并复制原始值或格式化结果。" },
  },
  {
    key: "converter",
    href: "/guide/plugins/system/converter.html",
    title: { en: "Converter", zh: "转换器" },
    description: { en: "Convert units, currencies, crypto, number bases, dates, and time zones.", zh: "转换单位、货币、加密货币、进制、日期和时区。" },
  },
  {
    key: "bookmark",
    href: "/guide/plugins/system/browser-bookmark.html",
    title: { en: "Browser Bookmark", zh: "浏览器书签" },
    description: { en: "Find and open bookmarks from your browser.", zh: "搜索并打开浏览器中保存的书签。" },
  },
  {
    key: "websearch",
    href: "/guide/plugins/system/websearch.html",
    title: { en: "WebSearch", zh: "网页搜索" },
    description: { en: "Search the web with your preferred search engine.", zh: "使用常用搜索引擎搜索网页。" },
  },
  {
    key: "emoji",
    href: "/guide/plugins/system/emoji.html",
    title: { en: "Emoji", zh: "Emoji" },
    description: { en: "Search emoji in a grid layout, with optional AI matching for natural descriptions.", zh: "用网格结果搜索 Emoji，也可以开启 AI 匹配自然语言描述。" },
  },
  {
    key: "chat",
    href: "/guide/plugins/system/chat.html",
    title: { en: "AI Chat", zh: "AI 对话" },
    description: { en: "Talk to configured models and agents with tools from inside Wox.", zh: "在 Wox 内与已配置的模型和 Agent 对话，并使用工具。" },
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
      <p>{{ isZh ? "从打开应用到找回剪贴板记录，直接在 Wox 里完成。" : "From opening an app to finding something you copied. All inside Wox." }}</p>
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
  margin: 16px 0 0;
}

.system-plugin-list {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0 48px;
  margin-top: 32px;
}

.system-plugin-item {
  display: block;
  padding: 24px 0;
  border-bottom: 1px solid var(--wox-home-border);
  text-decoration: none;
}

.system-plugin-item h3 {
  margin: 0;
  color: var(--vp-c-text-1);
  font-size: 18px;
  font-weight: 600;
  line-height: 1.4;
}

.system-plugin-item p {
  margin: 8px 0 0;
  font-size: 15px;
  line-height: 1.6;
}

.system-plugin-item:hover h3 {
  text-decoration: underline;
  text-underline-offset: 4px;
}

@media (max-width: 960px) {
  .system-plugin-list {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0 28px;
  }
}

@media (max-width: 540px) {
  .system-plugin-list {
    grid-template-columns: 1fr;
    margin-top: 16px;
  }
}
</style>
