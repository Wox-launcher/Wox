<script setup lang="ts">
import { computed } from "vue";
import { useData } from "vitepress";

const THREAD_URL =
  "https://www.reddit.com/r/windowsapps/search/?q=%22Wox%20Launcher%22%20author%3ARamyIssa&restrict_sr=1";

const quotes = [
  {
    text: "After using a lot of them I settled on using Flow Launcher, but now I stumbled upon Wox Launcher and it's actually so great that I'm considering a switch.",
    author: "u/RamyIssa",
    href: THREAD_URL,
  },
  {
    text: "From my testing it couldn't be better. It's so polished with great themes and so many customization capabilities.",
    author: "u/RamyIssa",
    href: THREAD_URL,
  },
];

const { lang } = useData();
const isZh = computed(() => (lang.value || "").toLowerCase().startsWith("zh"));
</script>

<template>
  <section class="wox-section wox-quotes">
    <div class="wox-section-heading">
      <h2>{{ isZh ? "大家怎么说" : "What people are saying" }}</h2>
    </div>
    <div class="wox-quotes-grid">
      <article v-for="quote in quotes" :key="quote.text" class="wox-quote-card">
        <blockquote>
          <p>{{ quote.text }}</p>
        </blockquote>
        <p class="wox-quote-meta">
          <a :href="quote.href" rel="noopener noreferrer">{{ quote.author }}</a>
          · Reddit · r/windowsapps
        </p>
      </article>
    </div>
    <p class="wox-quotes-more">
      <a :href="THREAD_URL" rel="noopener noreferrer">{{
        isZh ? "查看 Reddit 原帖" : "See the Reddit thread"
      }}</a>
    </p>
  </section>
</template>
