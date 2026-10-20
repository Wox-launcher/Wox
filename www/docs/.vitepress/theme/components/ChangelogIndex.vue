<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";

const props = defineProps<{ versions: string[]; chinese: boolean }>();
const root = ref<HTMLElement>();
const activeVersion = ref(props.versions[0]);
let frame = 0;
let cards: HTMLElement[] = [];

const releaseId = (version: string) => `release-v${version.replaceAll(".", "-")}`;

// Read the rendered scroll margin so the mobile picker and desktop nav share the same threshold.
function updateActiveVersion() {
  frame = 0;
  let current = 0;
  for (let index = 0; index < cards.length; index++) {
    const top = cards[index].getBoundingClientRect().top;
    const offset = parseFloat(getComputedStyle(cards[index]).scrollMarginTop);
    if (top > offset + 32) break;
    current = index;
  }
  activeVersion.value = props.versions[current];
  const link = root.value?.querySelector<HTMLElement>(`.release-links a[href="#${releaseId(activeVersion.value)}"]`);
  const list = link?.parentElement;
  // Scroll only the version list, never the document while the reader is scrolling.
  if (link && list && (link.offsetTop < list.scrollTop || link.offsetTop + link.offsetHeight > list.scrollTop + list.clientHeight)) {
    list.scrollTop = link.offsetTop - list.clientHeight / 2 + link.offsetHeight / 2;
  }
}

function scheduleUpdate() {
  if (!frame) frame = requestAnimationFrame(updateActiveVersion);
}

// Hash navigation preserves deep links and browser back/forward, including mobile selections.
function selectVersion(event: Event) {
  const version = (event.target as HTMLSelectElement).value;
  window.location.hash = releaseId(version);
  document.getElementById(releaseId(version))?.focus({ preventScroll: true });
}

onMounted(() => {
  cards = Array.from(root.value?.querySelectorAll<HTMLElement>(".release-card") || []);
  updateActiveVersion();
  window.addEventListener("scroll", scheduleUpdate, { passive: true });
  window.addEventListener("resize", scheduleUpdate);
  window.addEventListener("hashchange", scheduleUpdate);
});

onBeforeUnmount(() => {
  cancelAnimationFrame(frame);
  window.removeEventListener("scroll", scheduleUpdate);
  window.removeEventListener("resize", scheduleUpdate);
  window.removeEventListener("hashchange", scheduleUpdate);
});
</script>

<template>
  <div ref="root" class="wox-changelog">
    <nav class="release-navigation" :aria-label="chinese ? '版本列表' : 'Release list'">
      <p class="release-nav-title">{{ chinese ? "版本列表" : "Release list" }}</p>
      <!-- Bypass VitePress routing so native hash links respect scroll margins and keyboard focus. -->
      <div class="release-links vp-raw">
        <a
          v-for="version in versions"
          :key="version"
          :href="`#${releaseId(version)}`"
          :aria-current="activeVersion === version ? 'location' : undefined"
        >v{{ version }}</a>
      </div>
      <label class="release-picker">
        <span>{{ chinese ? "选择版本" : "Choose a version" }}</span>
        <select :value="activeVersion" @change="selectVersion">
          <option v-for="version in versions" :key="version" :value="version">Wox v{{ version }}</option>
        </select>
      </label>
      <noscript>
        <a v-for="version in versions" :key="version" :href="`#${releaseId(version)}`">v{{ version }}</a>
      </noscript>
    </nav>
    <div class="release-feed">
      <slot />
    </div>
  </div>
</template>

<style scoped>
.wox-changelog {
  display: grid;
  grid-template-columns: 184px minmax(0, 1fr);
  gap: 40px;
  align-items: start;
  margin-top: 32px;
}

.release-navigation {
  position: sticky;
  top: calc(var(--vp-nav-height) + 24px);
  min-width: 0;
}

.release-nav-title {
  margin: 0 0 12px;
  font-size: 14px;
  font-weight: 600;
  color: var(--vp-c-text-2);
}

.release-links {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: calc(100dvh - var(--vp-nav-height) - 108px);
  overflow-y: auto;
  overscroll-behavior: contain;
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
:deep(.release-header a:focus-visible) {
  outline: 2px solid var(--vp-c-brand-1);
  outline-offset: 3px;
}

.release-links a:focus-visible {
  outline-offset: -2px;
}

.release-picker {
  display: none;
}

.release-feed {
  display: grid;
  gap: 32px;
  min-width: 0;
}

:deep(.release-card) {
  min-width: 0;
  padding: 28px 32px;
  border: 1px solid var(--wox-home-border);
  border-radius: 12px;
  scroll-margin-top: calc(var(--vp-nav-height) + 24px);
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

/* Below desktop, VitePress scrolls the main nav away and keeps a 48px local bar. */
@media (max-width: 959px) {
  .release-navigation {
    top: 48px;
  }

  :deep(.release-card) {
    scroll-margin-top: 72px;
  }
}

@media (max-width: 767px) {
  .wox-changelog {
    grid-template-columns: minmax(0, 1fr);
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
    min-width: 148px;
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
    scroll-margin-top: 136px;
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
