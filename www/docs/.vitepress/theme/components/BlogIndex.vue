<script setup lang="ts">
import { computed } from "vue";
import { useData, withBase } from "vitepress";
import { data as posts } from "./blog.data.mts";

const { lang, isDark } = useData();
const isChinese = computed(() => lang.value.toLowerCase().startsWith("zh"));
const localizedPosts = computed(() => posts.filter((post) => post.lang === (isChinese.value ? "zh" : "en")));
const categories = computed(() =>
  isChinese.value
    ? { workspace: "窗口与应用", preview: "文件与预览", automation: "快捷键与自动化", product: "Wox 动态" }
    : { workspace: "Windows & apps", preview: "Files & previews", automation: "Shortcuts & automation", product: "Inside Wox" },
);

// Publication dates are calendar dates, so keep SSR and client time zones identical.
function formatDate(date: string) {
  return new Intl.DateTimeFormat(isChinese.value ? "zh-CN" : "en-US", {
    year: "numeric",
    month: "long",
    day: "numeric",
    timeZone: "UTC",
  }).format(new Date(`${date}T00:00:00Z`));
}
</script>

<template>
  <div class="wox-blog">
    <div class="blog-grid">
      <article v-for="(post, index) in localizedPosts" :key="post.url" class="blog-card">
        <a :href="withBase(post.url)" class="blog-card-link" :aria-label="post.title">
          <div class="blog-cover-frame">
            <img
              class="blog-cover"
              :src="withBase(isDark ? post.coverDark : post.cover)"
              :alt="post.coverAlt"
              width="1600"
              height="900"
              :loading="index < 3 ? 'eager' : 'lazy'"
              decoding="async"
            />
            <img v-if="post.cover.endsWith('/wox-launcher-loves-omarchy.webp')" class="blog-wox-logo" :src="withBase('/logo.png')" alt="" />
          </div>
          <div class="blog-card-body">
            <span class="blog-category">{{ categories[post.category] }}</span>
            <h2>{{ post.title }}</h2>
            <p class="blog-description">{{ post.description }}</p>
            <time :datetime="post.date">{{ formatDate(post.date) }}</time>
          </div>
        </a>
      </article>
    </div>
  </div>
</template>

<style scoped>
.wox-blog {
  margin-top: 32px;
}

.blog-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 24px;
}

.blog-card {
  overflow: hidden;
  border: 1px solid var(--wox-home-border);
  border-radius: 16px;
  background: var(--vp-c-bg);
  transition:
    border-color 160ms ease,
    box-shadow 160ms ease;
}

.blog-card:hover,
.blog-card:focus-within {
  border-color: var(--vp-c-text-3);
  box-shadow: 0 6px 24px rgb(0 0 0 / 7%);
}

.blog-card-link {
  display: flex;
  flex-direction: column;
  height: 100%;
  color: var(--vp-c-text-1);
  text-decoration: none;
}

.blog-card-link:focus-visible {
  outline: 3px solid var(--vp-c-brand-1);
  outline-offset: -4px;
  border-radius: 15px;
}

.blog-cover-frame {
  position: relative;
}

.blog-cover {
  display: block;
  width: 100%;
  height: auto;
  aspect-ratio: 16 / 9;
  object-fit: cover;
  background: #f3f4f5;
  border-bottom: 1px solid var(--wox-home-border);
}

.blog-wox-logo {
  position: absolute;
  left: 22.2%;
  top: 33.4%;
  width: 17.3%;
  height: auto;
}

.blog-card-body {
  display: flex;
  flex: 1;
  flex-direction: column;
  align-items: flex-start;
  padding: 24px;
}

.blog-category {
  color: var(--vp-c-text-2);
  font-size: 12px;
  font-weight: 500;
}

.blog-card h2 {
  margin: 10px 0 12px;
  padding: 0;
  border: 0;
  font-size: 21px;
  font-weight: 600;
  line-height: 1.45;
  letter-spacing: -0.02em;
  overflow-wrap: anywhere;
  text-wrap: balance;
}

.blog-description {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  margin: 0 0 24px;
  color: var(--vp-c-text-2);
  font-size: 15px;
  line-height: 1.7;
}

.blog-card time {
  margin-top: auto;
  color: var(--vp-c-text-2);
  font-size: 12px;
}

@media (max-width: 959px) {
  .blog-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 639px) {
  .blog-grid {
    grid-template-columns: minmax(0, 1fr);
    gap: 20px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .blog-card {
    transition: none;
  }
}
</style>
