<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useData, withBase } from "vitepress";

type ThemeSlide = {
  key: string;
  image: string;
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

const slides: ThemeSlide[] = [
  {
    key: "auto",
    image: "/images/theme-auto.png",
    title: { en: "Auto Theme", zh: "自动主题" },
    description: { en: "Follow the system appearance and keep Wox aligned with the desktop.", zh: "跟随系统外观，让 Wox 和桌面明暗状态保持一致。" },
    alt: { en: "Wox automatic theme setting screenshot", zh: "Wox 自动主题设置截图" },
  },
  {
    key: "dark",
    image: "/images/theme-dark.png",
    title: { en: "Dark", zh: "深色" },
    description: { en: "Use a restrained dark appearance for focused keyboard work.", zh: "使用克制的深色外观，适合长时间键盘操作。" },
    alt: { en: "Wox dark theme screenshot", zh: "Wox 深色主题截图" },
  },
  {
    key: "light",
    image: "/images/theme-light.png",
    title: { en: "Light", zh: "浅色" },
    description: { en: "Keep the launcher bright and readable in daytime environments.", zh: "在日间环境里保持明亮、清晰的启动器外观。" },
    alt: { en: "Wox light theme screenshot", zh: "Wox 浅色主题截图" },
  },
  {
    key: "tokyonight",
    image: "/images/theme-tokyonight.png",
    title: { en: "Tokyo Night", zh: "Tokyo Night" },
    description: { en: "Switch to a high-contrast editor-style palette.", zh: "切换到对比明确的编辑器风格配色。" },
    alt: { en: "Wox Tokyo Night theme screenshot", zh: "Wox Tokyo Night 主题截图" },
  },
  {
    key: "everforest",
    image: "/images/theme-everforest.png",
    title: { en: "Everforest", zh: "Everforest" },
    description: { en: "Use a softer green palette for a calmer launcher surface.", zh: "使用更柔和的绿色调，让启动器界面更平静。" },
    alt: { en: "Wox Everforest theme screenshot", zh: "Wox Everforest 主题截图" },
  },
  {
    key: "tundra",
    image: "/images/theme-tundra.png",
    title: { en: "Tundra", zh: "Tundra" },
    description: { en: "Try a cool muted palette when you want less visual weight.", zh: "使用冷静、低饱和的配色，降低视觉负担。" },
    alt: { en: "Wox Tundra theme screenshot", zh: "Wox Tundra 主题截图" },
  },
];

const { lang } = useData();
const activeIndex = ref(0);
const isPaused = ref(false);
let timer: ReturnType<typeof setInterval> | undefined;

const isZh = computed(() => (lang.value || "").toLowerCase().startsWith("zh"));
const activeSlide = computed(() => slides[activeIndex.value]);
const themeStoreHref = computed(() => withBase(isZh.value ? "/zh/store/themes.html" : "/store/themes.html"));

const text = computed(() => {
  if (isZh.value) {
    return {
      label: "主题",
      heading: "从明暗模式到社区主题，都可以换成你的风格",
      body: "Wox 的主题不只改变颜色，也会影响输入框、结果列表、激活项和整体氛围。首页直接展示几种真实外观，帮助用户快速理解主题能力。",
      previous: "上一张主题",
      next: "下一张主题",
      openStore: "浏览主题商店",
    };
  }

  return {
    label: "Themes",
    heading: "Switch from light and dark modes to community-made styles",
    body: "Wox themes shape the query box, result list, active item, and the overall launcher mood. These screenshots show real appearances instead of abstract color swatches.",
    previous: "Previous theme",
    next: "Next theme",
    openStore: "Browse themes",
  };
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
  // Theme screenshots have different source proportions, so the carousel keeps
  // a fixed viewport and rotates slowly enough for users to compare styles.
  timer = setInterval(() => {
    if (!isPaused.value) {
      go(1);
    }
  }, 6000);
}

onMounted(startAutoRotate);

onBeforeUnmount(() => {
  if (timer) {
    clearInterval(timer);
  }
});
</script>

<template>
  <section class="wox-section theme-showcase" @mouseenter="isPaused = true" @mouseleave="isPaused = false" @focusin="isPaused = true" @focusout="isPaused = false">
    <div class="theme-showcase-stage">
      <div class="theme-showcase-info">
        <span>{{ activeIndex + 1 }}/{{ slides.length }}</span>
        <div>
          <h3>{{ localize(activeSlide.title) }}</h3>
          <p>{{ localize(activeSlide.description) }}</p>
        </div>
      </div>

      <figure class="theme-showcase-shot">
        <img :src="withBase(activeSlide.image)" :alt="localize(activeSlide.alt)" />
      </figure>

      <div class="theme-showcase-controls" aria-label="Theme screenshots">
        <button type="button" class="theme-showcase-arrow" :aria-label="text.previous" @click="go(-1)">‹</button>
        <div class="theme-showcase-dots">
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
        <button type="button" class="theme-showcase-arrow" :aria-label="text.next" @click="go(1)">›</button>
      </div>
    </div>

    <div class="theme-showcase-copy">
      <p class="wox-home-label">{{ text.label }}</p>
      <h2>{{ text.heading }}</h2>
      <p>{{ text.body }}</p>
      <a class="wox-button" :href="themeStoreHref">{{ text.openStore }}</a>
    </div>
  </section>
</template>

<style scoped>
.theme-showcase {
  display: grid;
  /* Put the theme screenshots on the opposite side of the system plugin
  carousel while keeping the same wide visual column rhythm. */
  grid-template-columns: minmax(560px, 1.28fr) minmax(0, 0.72fr);
  gap: clamp(28px, 5vw, 64px);
  align-items: center;
}

.theme-showcase-copy h2 {
  margin: 0;
  font-size: clamp(32px, 4.6vw, 58px);
  line-height: 1.05;
  letter-spacing: 0;
}

.theme-showcase-copy p:not(.wox-home-label) {
  color: var(--wox-home-text-soft);
  font-size: 18px;
  line-height: 1.7;
}

.theme-showcase-copy .wox-button {
  margin-top: 18px;
}

.theme-showcase-stage {
  display: grid;
  grid-template-rows: auto auto 50px;
  min-width: 0;
  padding: 10px;
  border: 1px solid var(--wox-home-border);
  border-radius: 8px;
  background: var(--wox-home-panel-soft);
  box-shadow: 0 24px 70px rgba(0, 0, 0, 0.22);
}

.theme-showcase-info {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 14px;
  align-items: center;
  min-height: 96px;
  padding: 10px 10px 14px;
}

.theme-showcase-info span {
  color: var(--wox-home-accent-strong);
  font-size: 13px;
  font-weight: 800;
  line-height: 1;
}

.dark .theme-showcase-info span {
  color: var(--wox-home-accent);
}

.theme-showcase-info h3 {
  margin: 0;
  color: var(--vp-c-text-1);
  font-size: 24px;
  line-height: 1.2;
  letter-spacing: 0;
}

.theme-showcase-info p {
  margin: 8px 0 0;
  color: var(--wox-home-text-soft);
  font-size: 15px;
  line-height: 1.55;
}

.theme-showcase-shot {
  /* The fixed frame prevents tall and wide theme screenshots from shifting the
  homepage while still keeping the full launcher UI visible. */
  display: flex;
  align-items: center;
  justify-content: center;
  aspect-ratio: 4 / 3;
  margin: 0;
  overflow: hidden;
  border-radius: 6px;
  background: color-mix(in srgb, var(--wox-home-panel) 86%, var(--vp-c-bg));
}

.theme-showcase-shot img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
  object-position: center;
}

.theme-showcase-controls {
  display: grid;
  grid-template-columns: 40px minmax(0, 1fr) 40px;
  gap: 10px;
  align-items: center;
  padding: 12px 4px 2px;
}

.theme-showcase-arrow,
.theme-showcase-dots button {
  border: 1px solid var(--wox-home-border);
  background: var(--wox-home-panel);
  color: var(--vp-c-text-1);
  cursor: pointer;
}

.theme-showcase-arrow {
  width: 40px;
  height: 36px;
  border-radius: 8px;
  font-size: 24px;
  line-height: 1;
}

.theme-showcase-arrow:hover,
.theme-showcase-dots button:hover,
.theme-showcase-dots button.active {
  border-color: color-mix(in srgb, var(--wox-home-accent) 70%, var(--wox-home-border));
  background: color-mix(in srgb, var(--wox-home-accent) 14%, var(--wox-home-panel));
}

.theme-showcase-dots {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 2px;
  scrollbar-width: none;
}

.theme-showcase-dots::-webkit-scrollbar {
  display: none;
}

.theme-showcase-dots button {
  flex: 0 0 auto;
  min-width: 112px;
  padding: 9px 12px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 800;
  line-height: 1;
}

.theme-showcase-dots button span {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 1180px) {
  .theme-showcase {
    grid-template-columns: 1fr;
  }

  .theme-showcase-copy {
    order: -1;
  }
}

@media (max-width: 720px) {
  .theme-showcase-copy p:not(.wox-home-label) {
    font-size: 16px;
  }

  .theme-showcase-info {
    min-height: 120px;
  }

  .theme-showcase-controls {
    grid-template-columns: 36px minmax(0, 1fr) 36px;
  }

  .theme-showcase-arrow {
    width: 36px;
  }

  .theme-showcase-dots button {
    min-width: 92px;
  }
}
</style>
