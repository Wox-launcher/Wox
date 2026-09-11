<script setup lang="ts">
import { computed } from "vue";
import { useData, withBase } from "vitepress";

const latest = __WOX_LATEST_RELEASE__;
const { lang } = useData();
const isZh = computed(() => (lang.value || "").toLowerCase().startsWith("zh"));
const changelogHref = computed(() => withBase(latest.changelogUrl));
</script>

<template>
  <p class="wox-release-stamp">
    <template v-if="isZh">
      更新于 {{ latest.dateMonthZh }} · 最新 {{ latest.tag }} ·
      <a :href="latest.githubReleaseUrl">GitHub Release</a>
      ·
      <a :href="changelogHref">更新日志</a>
    </template>
    <template v-else>
      Updated {{ latest.monthNameEn }} {{ latest.year }} · Latest {{ latest.tag }} ·
      <a :href="latest.githubReleaseUrl">GitHub Release</a>
      ·
      <a :href="changelogHref">Changelog</a>
    </template>
  </p>
</template>
