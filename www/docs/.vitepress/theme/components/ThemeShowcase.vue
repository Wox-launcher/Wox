<script setup lang="ts">
import { computed } from "vue";
import { useData, withBase } from "vitepress";

const { lang } = useData();
const isZh = computed(() => (lang.value || "").toLowerCase().startsWith("zh"));
</script>

<template>
  <section class="wox-section theme-showcase">
    <figure class="wox-feature-shot wox-feature-shot-theme">
      <img
        :src="withBase('/images/theme-light.png')"
        :alt="isZh ? 'Wox 浅色主题，展示项目搜索结果' : 'Wox light theme showing project search results'"
        width="1904"
        height="1658"
        loading="lazy"
      />
    </figure>
    <div>
      <p class="wox-home-label">{{ isZh ? "主题" : "Themes" }}</p>
      <h2>{{ isZh ? "选一个喜欢的外观。" : "Make it feel at home." }}</h2>
      <p>
        {{
          isZh
            ? "跟随系统明暗模式，或从社区主题中选择颜色、背景和窗口样式。"
            : "Follow your system appearance, or choose a community theme for a different palette and window style."
        }}
      </p>
      <a class="wox-button" :href="withBase(isZh ? '/zh/store/themes.html' : '/store/themes.html')">{{ isZh ? "浏览主题商店" : "Browse themes" }}</a>
    </div>
  </section>
</template>

<style scoped>
.theme-showcase {
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(0, 0.85fr);
  align-items: center;
  gap: 64px;
  padding-top: 48px;
  border-top: 1px solid var(--wox-home-border);
}

@media (max-width: 720px) {
  .theme-showcase {
    grid-template-columns: 1fr;
    gap: 28px;
    padding-top: 32px;
  }

  .theme-showcase > div {
    order: -1;
  }
}
</style>
