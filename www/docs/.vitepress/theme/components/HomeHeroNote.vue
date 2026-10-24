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

const storyHref = computed(() => withBase(isZh.value ? "/zh/blog/rediscover-wox-v2" : "/blog/rediscover-wox-v2"));
const storyLabel = computed(() =>
  isZh.value ? "博客：重新认识 Wox v2" : "Blog: Rediscover Wox v2",
);
</script>

<template>
  <p class="wox-hero-note">
    {{ summary }}
    <br />
    <a :href="storyHref">{{ storyLabel }}</a>
  </p>
</template>
