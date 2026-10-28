<script setup lang="ts">
import { computed } from "vue";
import { hasThemeSwatch, type StoreThemeIconColors } from "./themeStore";

const props = defineProps<{
  colors?: StoreThemeIconColors;
  size?: number;
}>();

const visible = computed(() => hasThemeSwatch(props.colors));
const outlineWidth = computed(() => props.colors?.Outline && props.colors.OutlineWidth
  ? Math.min(2.5, Math.max(1.5, props.colors.OutlineWidth * 0.75))
  : 0);
</script>

<template>
  <svg v-if="visible" class="theme-swatch" :width="size || 56" :height="size || 56" viewBox="0 0 32 32" aria-hidden="true">
    <rect width="32" height="32" rx="8" :fill="colors?.Background" />
    <rect x="4" y="4" width="24" height="8" rx="4" :fill="colors?.Query" />
    <rect x="4" y="19" width="24" height="6" rx="2" :fill="colors?.Selected" />
    <rect v-if="colors?.Separator" x="4" y="13" width="24" height="0.75" :fill="colors.Separator" />
    <rect v-if="colors?.Indicator" x="4" y="19" width="1" height="6" :fill="colors.Indicator" />
    <rect v-if="outlineWidth" :x="outlineWidth / 2" :y="outlineWidth / 2" :width="32 - outlineWidth" :height="32 - outlineWidth" :rx="8 - outlineWidth / 2" fill="none" :stroke="colors?.Outline" :stroke-width="outlineWidth" />
  </svg>
</template>

<style scoped>
.theme-swatch {
  display: block;
  flex-shrink: 0;
}
</style>
