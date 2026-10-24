<script setup lang="ts">
import { computed } from "vue";
import { useData } from "vitepress";

const { lang } = useData();
const isZh = computed(() => lang.value.toLowerCase().startsWith("zh"));

const THREAD_URL =
  "https://www.reddit.com/r/windowsapps/comments/1woafkh/wox_the_best_alternative_ive_found_and_even/";
const PREVIOUS_THREAD_URL =
  "https://www.reddit.com/r/windowsapps/comments/1wawwhi/best_app_launcher_for_windows/";

const quotes = [
  {
    text: "Opensource, free, Surprisingly lighter than raycast while delivering the same and more.",
    textZh: "开源、免费，没想到比 Raycast 更轻量，却能提供同样的功能，甚至更多。",
    author: "u/NOVARYS",
    href: THREAD_URL,
  },
  {
    text: "After using a lot of them I settled on using Flow Launcher, but now I stumbled upon Wox Launcher and it's actually so great that I'm considering a switch.",
    textZh: "试过不少启动器后，我最终选了 Flow Launcher。不过最近偶然发现了 Wox Launcher，觉得很好用，已经在考虑换过来了。",
    author: "u/RamyIssa",
    href: PREVIOUS_THREAD_URL,
  },
];
</script>

<template>
  <section class="wox-section wox-quotes">
    <div class="wox-section-heading">
      <h2>{{ isZh ? "大家怎么说" : "What people are saying" }}</h2>
    </div>
    <div class="wox-quotes-grid">
      <article v-for="quote in quotes" :key="quote.author" class="wox-quote-card">
        <blockquote>
          <p>{{ isZh ? quote.textZh : quote.text }}</p>
        </blockquote>
        <p class="wox-quote-meta">
          <a :href="quote.href" rel="noopener noreferrer">
            {{ quote.author }} · Reddit · r/windowsapps
          </a>
          <span v-if="isZh"> · 译文</span>
        </p>
      </article>
    </div>
  </section>
</template>
