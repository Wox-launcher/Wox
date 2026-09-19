<script setup lang="ts">
import { computed } from "vue";
import { useData } from "vitepress";

const props = withDefaults(
  defineProps<{
    query?: string;
    href?: string;
    show?: "query" | "href";
  }>(),
  { show: "href" },
);

const { lang } = useData();

const buttonLabel = computed(() => {
  return (lang.value || "").toLowerCase().startsWith("zh") ? "在 Wox 中执行" : "Run in Wox";
});

const resolvedHref = computed(() => {
  if (props.href) {
    return props.href;
  }
  return `wox://query?q=${encodeURIComponent(props.query || "")}`;
});

const displayText = computed(() => {
  if (props.show === "query" && props.query) {
    return props.query;
  }
  return resolvedHref.value;
});
</script>

<template>
  <div class="query-row">
    <code class="query">{{ displayText }}</code>
    <a :href="resolvedHref" class="run-btn">{{ buttonLabel }}</a>
  </div>
</template>

<style scoped>
.query-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin: 20px 0;
  padding: 14px 16px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 14px;
  background: var(--vp-c-bg-soft);
}

.query {
  flex: 1 1 220px;
  margin: 0;
  padding: 0;
  background: transparent;
  color: var(--vp-c-text-1);
  font-size: 15px;
}

.run-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 34px;
  padding: 0 14px;
  border-radius: 999px;
  background: var(--vp-c-brand-1);
  color: var(--vp-c-bg);
  text-decoration: none;
  font-size: 13px;
  font-weight: 600;
  white-space: nowrap;
}

.run-btn:hover {
  background: var(--vp-c-brand-2);
  color: var(--vp-c-bg);
}
</style>
