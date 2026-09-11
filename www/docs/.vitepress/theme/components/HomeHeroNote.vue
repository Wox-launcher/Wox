<script setup lang="ts">
import { computed } from "vue";
import { useData, withBase } from "vitepress";

const latest = __WOX_LATEST_RELEASE__;
const { lang } = useData();
const isZh = computed(() => (lang.value || "").toLowerCase().startsWith("zh"));

const summary = computed(() =>
  isZh.value
    ? `最新 ${latest.tag} · ${latest.dateMonthZh} · Windows · macOS · Linux`
    : `Latest ${latest.tag} · ${latest.monthNameEn} ${latest.year} · Windows · macOS · Linux`,
);

const compareHref = computed(() => withBase(isZh.value ? "/zh/compare/" : "/compare/"));
const compareLabel = computed(() =>
  isZh.value ? "对比 Flow、Raycast 和 PowerToys" : "Wox vs Flow, Raycast, and PowerToys",
);
</script>

<template>
  <p class="wox-hero-note">
    {{ summary }}
    <br />
    <a :href="compareHref">{{ compareLabel }}</a>
  </p>
</template>
