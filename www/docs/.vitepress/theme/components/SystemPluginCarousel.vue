<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useData, withBase } from "vitepress";

type SystemPluginSlide = {
  key: string;
  image: string;
  href: string;
  title: {
    en: string;
    zh: string;
  };
  description: {
    en: string;
    zh: string;
  };
  alt: {
    en: string;
    zh: string;
  };
};

const slides: SystemPluginSlide[] = [
  {
    key: "app",
    image: "/images/system-plugin-app.png",
    href: "/guide/plugins/system/application.html",
    title: { en: "Application", zh: "应用" },
    description: { en: "Launch apps, open folders, and activate running windows from one result list.", zh: "启动应用、打开所在目录，并从结果列表直接激活已运行窗口。" },
    alt: { en: "Wox Application plugin showing app results", zh: "Wox 应用插件展示应用搜索结果" },
  },
  {
    key: "file",
    image: "/images/system-plugin-filesearch.png",
    href: "/guide/plugins/system/file.html",
    title: { en: "File", zh: "文件" },
    description: { en: "Search indexed files and folders with focused roots and fast result actions.", zh: "搜索已索引的文件和文件夹，并用操作面板继续处理结果。" },
    alt: { en: "Wox File plugin showing file search results", zh: "Wox 文件插件展示文件搜索结果" },
  },
  {
    key: "clipboard",
    image: "/images/system-plugin-clipboard.png",
    href: "/guide/plugins/system/clipboard.html",
    title: { en: "Clipboard", zh: "剪贴板" },
    description: { en: "Bring back recent text and image clipboard items without leaving the keyboard.", zh: "不用离开键盘就能找回最近的文本和图片剪贴板记录。" },
    alt: { en: "Wox Clipboard plugin showing clipboard history", zh: "Wox 剪贴板插件展示剪贴板历史" },
  },
  {
    key: "calculator",
    image: "/images/system-plugin-calculator.png",
    href: "/guide/plugins/system/calculator.html",
    title: { en: "Calculator", zh: "计算器" },
    description: { en: "Evaluate expressions directly in the launcher and copy formatted results.", zh: "直接在启动器里计算表达式，并复制原始值或格式化结果。" },
    alt: { en: "Wox Calculator plugin showing calculation results", zh: "Wox 计算器插件展示计算结果" },
  },
  {
    key: "converter",
    image: "/images/system-plugin-converter.png",
    href: "/guide/plugins/system/converter.html",
    title: { en: "Converter", zh: "转换器" },
    description: { en: "Convert units, currencies, crypto, number bases, dates, and time zones.", zh: "转换单位、货币、加密货币、进制、日期和时区。" },
    alt: { en: "Wox Converter plugin showing conversion results", zh: "Wox 转换器插件展示转换结果" },
  },
  {
    key: "bookmark",
    image: "/images/system-plugin-bookmark.png",
    href: "/guide/plugins/system/browser-bookmark.html",
    title: { en: "Browser Bookmark", zh: "浏览器书签" },
    description: { en: "Open bookmarks from supported browser profiles with icons and MRU ordering.", zh: "从受支持的浏览器 profile 中搜索书签，带图标和常用排序。" },
    alt: { en: "Wox Browser Bookmark plugin showing bookmark results", zh: "Wox 浏览器书签插件展示书签结果" },
  },
  {
    key: "websearch",
    image: "/images/system-plugin-websearch.png",
    href: "/guide/plugins/system/websearch.html",
    title: { en: "WebSearch", zh: "网页搜索" },
    description: { en: "Send fallback searches or explicit engine keywords to your configured search URLs.", zh: "用 fallback 搜索或搜索引擎关键字打开已配置的搜索 URL。" },
    alt: { en: "Wox WebSearch plugin showing search engine results", zh: "Wox 网页搜索插件展示搜索结果" },
  },
  {
    key: "emoji",
    image: "/images/system-plugin-emoji.png",
    href: "/guide/plugins/system/emoji.html",
    title: { en: "Emoji", zh: "Emoji" },
    description: { en: "Search emoji in a grid layout, with optional AI matching for natural descriptions.", zh: "用网格结果搜索 Emoji，也可以开启 AI 匹配自然语言描述。" },
    alt: { en: "Wox Emoji plugin showing emoji grid results", zh: "Wox Emoji 插件展示网格结果" },
  },
  {
    key: "chat",
    image: "/images/system-plugin-ai-chat.png",
    href: "/guide/plugins/system/chat.html",
    title: { en: "AI Chat", zh: "AI 对话" },
    description: { en: "Talk to configured models and agents with tools from inside Wox.", zh: "在 Wox 内与已配置的模型和 Agent 对话，并使用工具。" },
    alt: { en: "Wox AI Chat plugin showing a conversation", zh: "Wox AI 对话插件展示会话" },
  },
];

const { lang } = useData();
const activeIndex = ref(0);
const isPaused = ref(false);
let timer: ReturnType<typeof setInterval> | undefined;

const isZh = computed(() => (lang.value || "").toLowerCase().startsWith("zh"));

const text = computed(() => {
  if (isZh.value) {
    return {
      label: "系统插件",
      heading: "内置插件覆盖从启动到 AI 的日常工作",
      body: "Wox 自带的系统插件不只是示例。它们负责应用、文件、书签、网页搜索、剪贴板、计算、转换、Emoji 和 AI 对话等高频入口。",
      previous: "上一张",
      next: "下一张",
      openGuide: "查看指南",
    };
  }

  return {
    label: "System plugins",
    heading: "Built-in plugins cover everyday work from launch to AI",
    body: "Wox ships with system plugins for apps, files, bookmarks, web search, clipboard history, calculation, conversion, emoji, and AI chat.",
    previous: "Previous",
    next: "Next",
    openGuide: "Open guide",
  };
});

const activeSlide = computed(() => slides[activeIndex.value]);

const localizedHref = computed(() => {
  const href = activeSlide.value.href;
  return withBase(isZh.value ? `/zh${href}` : href);
});

function localize<T extends { en: string; zh: string }>(value: T) {
  return isZh.value ? value.zh : value.en;
}

function setActive(index: number) {
  activeIndex.value = (index + slides.length) % slides.length;
}

function go(delta: number) {
  setActive(activeIndex.value + delta);
}

function startAutoRotate() {
  // The carousel is informational, not a task-critical control. Rotate slowly
  // and pause on interaction so users can inspect screenshots without fighting
  // automatic movement.
  timer = setInterval(() => {
    if (!isPaused.value) {
      go(1);
    }
  }, 5500);
}

onMounted(startAutoRotate);

onBeforeUnmount(() => {
  if (timer) {
    clearInterval(timer);
  }
});
</script>

<template>
  <section class="system-plugin-carousel" @mouseenter="isPaused = true" @mouseleave="isPaused = false" @focusin="isPaused = true" @focusout="isPaused = false">
    <div class="system-plugin-copy">
      <p class="wox-home-label">{{ text.label }}</p>
      <h2>{{ text.heading }}</h2>
      <p>{{ text.body }}</p>
    </div>

    <div class="system-plugin-stage">
      <div class="system-plugin-info">
        <span class="system-plugin-count">{{ activeIndex + 1 }}/{{ slides.length }}</span>
        <h3>{{ localize(activeSlide.title) }}</h3>
        <p>{{ localize(activeSlide.description) }}</p>
        <a class="system-plugin-link" :href="localizedHref">{{ text.openGuide }}</a>
      </div>

      <figure class="system-plugin-shot">
        <img :src="withBase(activeSlide.image)" :alt="localize(activeSlide.alt)" />
      </figure>

      <div class="system-plugin-controls" aria-label="System plugin screenshots">
        <button type="button" class="system-plugin-arrow" :aria-label="text.previous" @click="go(-1)">‹</button>
        <div class="system-plugin-dots">
          <button
            v-for="(slide, index) in slides"
            :key="slide.key"
            type="button"
            :class="{ active: index === activeIndex }"
            :aria-label="localize(slide.title)"
            :aria-current="index === activeIndex ? 'true' : undefined"
            @click="setActive(index)"
          >
            <span>{{ localize(slide.title) }}</span>
          </button>
        </div>
        <button type="button" class="system-plugin-arrow" :aria-label="text.next" @click="go(1)">›</button>
      </div>
    </div>
  </section>
</template>

<style scoped>
.system-plugin-carousel {
  display: grid;
  grid-template-columns: minmax(0, 0.72fr) minmax(560px, 1.28fr);
  gap: clamp(28px, 5vw, 64px);
  align-items: center;
  margin-top: clamp(76px, 10vw, 128px);
}

.system-plugin-copy h2 {
  margin: 0;
  font-size: clamp(32px, 4.6vw, 58px);
  line-height: 1.05;
  letter-spacing: 0;
}

.system-plugin-copy p:not(.wox-home-label) {
  color: var(--wox-home-text-soft);
  font-size: 18px;
  line-height: 1.7;
}

.system-plugin-stage {
  display: grid;
  grid-template-rows: auto auto 50px;
  min-width: 0;
  padding: 10px;
  border: 1px solid var(--wox-home-border);
  border-radius: 8px;
  background: var(--wox-home-panel-soft);
  box-shadow: 0 24px 70px rgba(0, 0, 0, 0.22);
}

.system-plugin-info {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 10px 16px;
  align-items: center;
  min-height: 98px;
  padding: 10px 10px 14px;
}

.system-plugin-count {
  color: var(--wox-home-accent-strong);
  font-size: 13px;
  font-weight: 800;
  line-height: 1;
}

.dark .system-plugin-count {
  color: var(--wox-home-accent);
}

.system-plugin-info h3 {
  margin: 0;
  color: var(--vp-c-text-1);
  font-size: 24px;
  line-height: 1.2;
  letter-spacing: 0;
}

.system-plugin-info p {
  grid-column: 2 / 3;
  margin: 0;
  color: var(--wox-home-text-soft);
  font-size: 15px;
  line-height: 1.55;
}

.system-plugin-link {
  grid-column: 3;
  grid-row: 1 / span 2;
  align-self: center;
  padding: 10px 14px;
  border: 1px solid var(--wox-home-border);
  border-radius: 8px;
  color: var(--vp-c-text-1);
  font-size: 14px;
  font-weight: 800;
  line-height: 1;
  text-decoration: none;
}

.system-plugin-link:hover {
  border-color: color-mix(in srgb, var(--wox-home-accent) 62%, var(--wox-home-border));
  color: var(--vp-c-text-1);
  text-decoration: none;
}

.system-plugin-shot {
  /* Keep every slide in the same viewport so screenshots with different source
  dimensions do not resize the carousel while users browse the system plugins. */
  display: flex;
  align-items: center;
  justify-content: center;
  aspect-ratio: 16 / 9;
  margin: 0;
  overflow: hidden;
  border-radius: 6px;
  background: color-mix(in srgb, var(--wox-home-panel) 86%, var(--vp-c-bg));
}

.system-plugin-shot img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
  object-position: center;
}

.system-plugin-controls {
  display: grid;
  grid-template-columns: 40px minmax(0, 1fr) 40px;
  gap: 10px;
  align-items: center;
  padding: 12px 4px 2px;
}

.system-plugin-arrow,
.system-plugin-dots button {
  border: 1px solid var(--wox-home-border);
  background: var(--wox-home-panel);
  color: var(--vp-c-text-1);
  cursor: pointer;
}

.system-plugin-arrow {
  width: 40px;
  height: 36px;
  border-radius: 8px;
  font-size: 24px;
  line-height: 1;
}

.system-plugin-arrow:hover,
.system-plugin-dots button:hover,
.system-plugin-dots button.active {
  border-color: color-mix(in srgb, var(--wox-home-accent) 70%, var(--wox-home-border));
  background: color-mix(in srgb, var(--wox-home-accent) 14%, var(--wox-home-panel));
}

.system-plugin-dots {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 2px;
  scrollbar-width: none;
}

.system-plugin-dots::-webkit-scrollbar {
  display: none;
}

.system-plugin-dots button {
  flex: 0 0 auto;
  min-width: 112px;
  padding: 9px 12px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 800;
  line-height: 1;
}

.system-plugin-dots button span {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 1180px) {
  .system-plugin-carousel {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .system-plugin-copy p:not(.wox-home-label) {
    font-size: 16px;
  }

  .system-plugin-info {
    grid-template-columns: 1fr;
    min-height: 128px;
  }

  .system-plugin-count,
  .system-plugin-info p,
  .system-plugin-link {
    grid-column: auto;
    grid-row: auto;
  }

  .system-plugin-link {
    justify-self: start;
  }

  .system-plugin-controls {
    grid-template-columns: 36px minmax(0, 1fr) 36px;
  }

  .system-plugin-arrow {
    width: 36px;
  }

  .system-plugin-dots button {
    min-width: 92px;
  }
}
</style>
