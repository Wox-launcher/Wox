<script setup lang="ts">
const THREAD_URL =
  "https://www.reddit.com/r/windowsapps/comments/1woafkh/wox_the_best_alternative_ive_found_and_even/";
const PREVIOUS_THREAD_URL =
  "https://www.reddit.com/r/windowsapps/comments/1wawwhi/best_app_launcher_for_windows/";

const quotes = [
  {
    text: "Opensource, free, Surprisingly lighter than raycast while delivering the same and more.",
    author: "u/NOVARYS",
    href: THREAD_URL,
  },
  {
    text: "After using a lot of them I settled on using Flow Launcher, but now I stumbled upon Wox Launcher and it's actually so great that I'm considering a switch.",
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
      <article v-for="quote in quotes" :key="quote.text" class="wox-quote-card">
        <blockquote>
          <p>{{ quote.text }}</p>
        </blockquote>
        <p class="wox-quote-meta">
          <a :href="quote.href" rel="noopener noreferrer">
            {{ quote.author }} · Reddit · r/windowsapps
          </a>
        </p>
      </article>
    </div>
  </section>
</template>
