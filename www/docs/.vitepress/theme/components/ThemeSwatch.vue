<script setup lang="ts">
import { computed } from "vue";
import { hasThemeSwatch, type StoreThemeIconColors } from "./themeStore";

const props = defineProps<{
  colors?: StoreThemeIconColors;
  size?: number;
}>();

const visible = computed(() => hasThemeSwatch(props.colors));
const frameStyle = computed(() => {
  const colors = props.colors;
  const width = colors?.Outline && colors.OutlineWidth ? Math.min(3, Math.max(1, colors.OutlineWidth)) : 0;
  return {
    width: `${props.size || 56}px`,
    height: `${props.size || 56}px`,
    background: colors?.Background || "transparent",
    border: width > 0 ? `${width}px solid ${colors?.Outline}` : "0",
  };
});
const queryColor = computed(() => {
  const query = props.colors?.Query || "transparent";
  return query.toLowerCase() === "transparent" ? "transparent" : query;
});
</script>

<template>
  <div v-if="visible" class="theme-swatch" :style="frameStyle">
    <span class="bar query" :style="{ background: queryColor }"></span>
    <span class="bar selected" :style="{ background: colors?.Selected || 'transparent' }"></span>
  </div>
</template>

<style scoped>
.theme-swatch {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 4px;
  box-sizing: border-box;
  padding: 7px;
  border-radius: 16px;
  flex-shrink: 0;
}

.bar {
  display: block;
  width: 100%;
  border-radius: 4px;
}

.query {
  height: 10px;
}

.selected {
  height: 6px;
  border-radius: 3px;
}
</style>
