<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";

type ReleaseLink = {
  tag: string;
  year: number;
  href: string;
};

const props = defineProps<{ releases: ReleaseLink[]; chinese: boolean }>();
const root = ref<HTMLElement>();
const feed = ref<HTMLElement>();
const activeTag = ref(props.releases[0]?.tag ?? "");
let frame = 0;
let cards: HTMLElement[] = [];

const groups = computed(() => {
  const grouped: { year: number; releases: ReleaseLink[] }[] = [];
  for (const release of props.releases) {
    const current = grouped[grouped.length - 1];
    if (!current || current.year !== release.year) {
      grouped.push({ year: release.year, releases: [release] });
    } else {
      current.releases.push(release);
    }
  }
  return grouped;
});

// Track the card at the top of the independently scrolling release feed.
function updateActiveVersion() {
  frame = 0;
  if (cards.length === 0 || !feed.value) {
    return;
  }
  let current = 0;
  const top = feed.value.getBoundingClientRect().top;
  for (let index = 0; index < cards.length; index++) {
    if (cards[index].getBoundingClientRect().top > top + 32) break;
    current = index;
  }
  if (feed.value.scrollTop + feed.value.clientHeight >= feed.value.scrollHeight - 1) {
    current = cards.length - 1;
  }
  const tag = cards[current].dataset.release;
  if (!tag) {
    return;
  }
  activeTag.value = tag;
  const list = root.value?.querySelector<HTMLElement>(".release-links");
  const link = list ? [...list.querySelectorAll<HTMLAnchorElement>("a[data-tag]")].find((item) => item.dataset.tag === tag) : undefined;
  // Year groups sit between the link and the scrolling list, so scroll that list directly.
  if (link && list && (link.offsetTop < list.scrollTop || link.offsetTop + link.offsetHeight > list.scrollTop + list.clientHeight)) {
    list.scrollTop = link.offsetTop - list.clientHeight / 2 + link.offsetHeight / 2;
  }
}

function scheduleUpdate() {
  if (!frame) frame = requestAnimationFrame(updateActiveVersion);
}

// Restore deep links inside the feed, including browser back and forward navigation.
function scrollToRelease() {
  const card = cards.find((item) => `#${item.id}` === window.location.hash);
  if (card && feed.value) {
    feed.value.scrollTop += card.getBoundingClientRect().top - feed.value.getBoundingClientRect().top;
  }
  scheduleUpdate();
}

// Hash navigation preserves deep links and browser back/forward.
function selectRelease(event: Event) {
  const select = event.target as HTMLSelectElement;
  const release = props.releases.find((item) => item.tag === select.value);
  if (!release) {
    return;
  }
  const id = release.href.slice(1);
  window.location.hash = id;
  document.getElementById(id)?.focus({ preventScroll: true });
}

onMounted(() => {
  cards = Array.from(root.value?.querySelectorAll<HTMLElement>(".release-card") || []);
  scrollToRelease();
  window.addEventListener("resize", scheduleUpdate);
  window.addEventListener("hashchange", scrollToRelease);
});

onBeforeUnmount(() => {
  cancelAnimationFrame(frame);
  window.removeEventListener("resize", scheduleUpdate);
  window.removeEventListener("hashchange", scrollToRelease);
});
</script>

<template>
  <div ref="root" class="wox-changelog">
    <nav class="release-navigation" :aria-label="chinese ? '版本列表' : 'Release list'">
      <p class="release-nav-title">{{ chinese ? "版本列表" : "Release list" }}</p>
      <!-- Bypass VitePress routing so native hash links respect scroll margins and keyboard focus. -->
      <div class="release-links vp-raw">
        <div v-for="group in groups" :key="group.year" class="release-year-group">
          <p class="release-year">{{ group.year }}</p>
          <a
            v-for="release in group.releases"
            :key="release.tag"
            :href="release.href"
            :data-tag="release.tag"
            :aria-current="activeTag === release.tag ? 'location' : undefined"
          >{{ release.tag }}</a>
        </div>
      </div>
      <label class="release-picker">
        <span>{{ chinese ? "选择版本" : "Choose a version" }}</span>
        <select :value="activeTag" @change="selectRelease">
          <optgroup v-for="group in groups" :key="group.year" :label="String(group.year)">
            <option v-for="release in group.releases" :key="release.tag" :value="release.tag">Wox {{ release.tag }}</option>
          </optgroup>
        </select>
      </label>
      <noscript>
        <template v-for="group in groups" :key="group.year">
          <p>{{ group.year }}</p>
          <a v-for="release in group.releases" :key="release.tag" :href="release.href">{{ release.tag }}</a>
        </template>
      </noscript>
    </nav>
    <div ref="feed" class="release-feed" role="region" :aria-label="chinese ? '更新日志' : 'Release notes'" tabindex="0" @scroll.passive="scheduleUpdate">
      <slot />
    </div>
  </div>
</template>

<style>
/* Fill the viewport only on changelog indexes; the cards own the page's scrolling. */
.Layout:has(.wox-changelog) {
  height: 100dvh;
  min-height: 0;
  overflow: hidden;
}

.Layout:has(.wox-changelog) .VPFooter,
.Layout:has(.wox-changelog) .VPDocFooter {
  display: none;
}

.Layout:has(.wox-changelog) .VPContent {
  flex-shrink: 1;
  min-height: 0;
}

.Layout:has(.wox-changelog) .VPDoc,
.Layout:has(.wox-changelog) .VPDoc .container,
.Layout:has(.wox-changelog) .VPDoc .content,
.Layout:has(.wox-changelog) .VPDoc .content-container,
.Layout:has(.wox-changelog) .VPDoc .main,
.Layout:has(.wox-changelog) .vp-doc,
.Layout:has(.wox-changelog) .vp-doc > div {
  height: 100%;
  min-height: 0;
}

.Layout:has(.wox-changelog) .VPDoc,
.Layout:has(.wox-changelog) .VPDoc .content {
  padding-bottom: 0;
}

.Layout:has(.wox-changelog) .vp-doc > div {
  display: flex;
  flex-direction: column;
}
</style>

<style scoped>
.wox-changelog {
  display: grid;
  grid-template-columns: 196px minmax(0, 1fr);
  grid-template-rows: minmax(0, 1fr);
  gap: 40px;
  flex: 1 1 auto;
  min-height: 0;
  margin-top: 32px;
}

.release-navigation {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}

.release-nav-title {
  flex: 0 0 auto;
  margin: 0 0 12px;
  font-size: 14px;
  font-weight: 600;
  color: var(--vp-c-text-2);
}

.release-links {
  position: relative;
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.release-links,
.release-feed {
  scrollbar-width: thin;
  scrollbar-color: transparent transparent;
}

.release-links:hover,
.release-links:focus-within,
.release-feed:hover,
.release-feed:focus-within {
  scrollbar-color: rgba(128, 128, 128, 0.72) transparent;
}

.release-links::-webkit-scrollbar,
.release-feed::-webkit-scrollbar {
  width: 8px;
}

.release-links::-webkit-scrollbar-track,
.release-feed::-webkit-scrollbar-track {
  background: transparent;
}

.release-links::-webkit-scrollbar-thumb,
.release-feed::-webkit-scrollbar-thumb {
  border: 2px solid transparent;
  border-radius: 8px;
  background: transparent;
  background-clip: padding-box;
}

.release-links:hover::-webkit-scrollbar-thumb,
.release-links:focus-within::-webkit-scrollbar-thumb,
.release-feed:hover::-webkit-scrollbar-thumb,
.release-feed:focus-within::-webkit-scrollbar-thumb {
  background: rgba(128, 128, 128, 0.72);
  background-clip: padding-box;
}

.release-year-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.release-year-group + .release-year-group {
  margin-top: 12px;
}

.release-year {
  position: sticky;
  top: 0;
  z-index: 1;
  margin: 0;
  padding: 0 14px 4px;
  background: var(--vp-c-bg);
  color: var(--vp-c-text-3);
  font-size: 12px;
  font-weight: 600;
  line-height: 16px;
}

.release-links a {
  position: relative;
  display: block;
  padding: 8px 14px;
  border-radius: 6px;
  color: var(--vp-c-text-2);
  font-size: 14px;
  font-weight: 500;
  line-height: 24px;
  text-decoration: none;
  overflow-wrap: anywhere;
}

.release-links a:hover {
  color: var(--vp-c-text-1);
  background: var(--vp-c-bg-soft);
}

.release-links a[aria-current="location"] {
  color: var(--vp-c-text-1);
  background: var(--vp-c-bg-soft);
  font-weight: 600;
}

.release-links a[aria-current="location"]::before {
  position: absolute;
  top: 10px;
  bottom: 10px;
  left: 0;
  width: 3px;
  border-radius: 2px;
  background: var(--vp-c-brand-1);
  content: "";
}

.release-links a:focus-visible,
.release-picker select:focus-visible,
.release-feed:focus-visible,
:deep(.release-header a:focus-visible) {
  outline: 2px solid var(--vp-c-brand-1);
  outline-offset: 3px;
}

.release-links a:focus-visible,
.release-feed:focus-visible {
  outline-offset: -2px;
}

.release-picker {
  display: none;
}

.release-feed {
  display: grid;
  gap: 32px;
  min-width: 0;
  min-height: 0;
  padding: 0 8px 24px 0;
  overflow-y: auto;
  overscroll-behavior: contain;
}

:deep(.release-card) {
  min-width: 0;
  padding: 28px 32px;
  border: 1px solid var(--wox-home-border);
  border-radius: 12px;
  scroll-margin-top: 0;
  overflow-wrap: anywhere;
}

:deep(.release-card:focus-visible) {
  outline: 2px solid var(--vp-c-brand-1);
  outline-offset: 4px;
}

:deep(.release-header) {
  padding-bottom: 20px;
  margin-bottom: 24px;
  border-bottom: 1px solid var(--vp-c-divider);
}

:deep(.release-title-row) {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
}

:deep(.release-title-row h2) {
  margin: 0;
  padding: 0;
  border: 0;
  font-size: 26px;
  line-height: 36px;
  letter-spacing: -0.02em;
}

:deep(.release-title-row h2 a) {
  color: var(--vp-c-text-1);
  text-decoration: none;
  font-weight: 600;
}

:deep(.release-title-row h2 a:hover) {
  text-decoration: underline;
  text-underline-offset: 4px;
}

:deep(.release-badge) {
  padding: 1px 10px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 20px;
  background: var(--vp-c-bg-soft);
  font-size: 12px;
  line-height: 22px;
}

:deep(.release-meta) {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 20px;
  margin-top: 10px;
  color: var(--vp-c-text-2);
  font-size: 13px;
}

:deep(.release-meta a) {
  color: var(--vp-c-text-2);
  font-weight: 400;
  text-decoration: none;
}

:deep(.release-meta a:hover) {
  text-decoration: underline;
}

:deep(.release-card > :last-child) {
  margin-bottom: 0;
}

:deep(.release-card img),
:deep(.release-card video) {
  display: block;
  width: auto;
  max-width: 100%;
  height: auto;
  max-height: 560px;
  margin: 20px 0;
  border-radius: 8px;
}

@media (max-width: 767px) {
  .wox-changelog {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: auto minmax(0, 1fr);
    gap: 20px;
    margin-top: 24px;
  }

  .release-navigation {
    z-index: 10;
    padding: 12px 0;
    background: var(--vp-c-bg);
    border-bottom: 1px solid var(--vp-c-divider);
  }

  .release-nav-title,
  .release-links {
    display: none;
  }

  .release-picker {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    font-size: 14px;
    font-weight: 500;
  }

  .release-picker select {
    min-width: 180px;
    max-width: 70%;
    padding: 8px 12px;
    border: 1px solid var(--wox-home-border);
    border-radius: 6px;
    background: var(--vp-c-bg-soft);
    color: var(--vp-c-text-1);
    font: inherit;
    cursor: pointer;
  }

  .release-feed {
    gap: 24px;
  }

  :deep(.release-card) {
    padding: 20px;
  }

  :deep(.release-title-row h2) {
    font-size: 23px;
    line-height: 32px;
  }

  noscript a {
    display: inline-block;
    margin: 4px 12px 4px 0;
  }
}
</style>
