<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useData, withBase } from "vitepress";
import { sortStorePlugins } from "./pluginSort.mjs";
import { englishPluginName, useStorePlugins, localizePlugin, pluginDetailPath, type LocalizedStorePluginManifest } from "./pluginStore";


const searchQuery = ref("");
const sortOrder = ref("downloads");
const sortMenu = ref<HTMLDetailsElement | null>(null);
const sortTrigger = ref<HTMLElement | null>(null);
const { lang } = useData();

const uiText = computed(() => {
  const normalizedLang = (lang.value || "").toLowerCase();

  if (normalizedLang.startsWith("zh")) {
    return {
      searchPlaceholder: "搜索插件...",
      by: "作者",
      install: "安装",
      source: "官网",
      empty: "没有找到匹配的插件。",
      sort: "排序方式",
      nameSort: "名称",
      updatedSort: "更新时间",
      starsSort: "Star 数量",
      stars: "GitHub star 数量",
      downloadsSort: "下载量",
      downloads: "下载量",
    };
  }

  return {
    searchPlaceholder: "Search plugins...",
    by: "by",
    install: "Install",
    source: "Website",
    empty: "No plugins match your search.",
    sort: "Sort by",
    nameSort: "Name",
    updatedSort: "Recently updated",
    starsSort: "Stars",
    stars: "GitHub stars",
    downloadsSort: "Downloads",
    downloads: "Downloads",
  };
});

const sortOptions = computed(() => [
  { value: "name", label: uiText.value.nameSort },
  { value: "stars", label: uiText.value.starsSort },
  { value: "downloads", label: uiText.value.downloadsSort },
  { value: "updated", label: uiText.value.updatedSort },
]);
const selectedSortLabel = computed(() => sortOptions.value.find((option) => option.value === sortOrder.value)?.label || uiText.value.downloadsSort);

function closeSortMenu(restoreFocus = false) {
  if (sortMenu.value) sortMenu.value.open = false;
  if (restoreFocus) sortTrigger.value?.focus();
}

function onSortOptionClick(event: MouseEvent) {
  if (event.detail > 0) closeSortMenu(true);
}

function dismissSortMenu(event: PointerEvent) {
  if (event.target instanceof Node && !sortMenu.value?.contains(event.target)) closeSortMenu();
}

function onSortFocusOut(event: FocusEvent) {
  if (event.relatedTarget instanceof Node && !sortMenu.value?.contains(event.relatedTarget)) closeSortMenu();
}

onMounted(() => document.addEventListener("pointerdown", dismissSortMenu));
onUnmounted(() => document.removeEventListener("pointerdown", dismissSortMenu));

const storePlugins = useStorePlugins();
const plugins = computed(() => storePlugins.value.map((plugin) => localizePlugin(plugin, lang.value)));

const filteredPlugins = computed(() => {
  const query = searchQuery.value.trim().toLowerCase();
  const matching = plugins.value.filter((plugin) => {
    return !query || [plugin.LocalizedName, plugin.LocalizedDescription, plugin.Author, plugin.Runtime].filter(Boolean).some((value) => value!.toLowerCase().includes(query));
  });
  return sortStorePlugins(matching, sortOrder.value, lang.value);
});

function pluginDetailHref(pluginId: string) {
  return withBase(pluginDetailPath(pluginId, lang.value));
}

function installHref(plugin: LocalizedStorePluginManifest) {
  return `wox://query?q=${encodeURIComponent(`wpm install ${englishPluginName(plugin)}`)}`;
}
</script>

<template>
  <div class="gallery-container">
    <div class="search-bar">
      <input v-model="searchQuery" type="search" :placeholder="uiText.searchPlaceholder" :aria-label="uiText.searchPlaceholder" class="search-input" />
      <details ref="sortMenu" class="sort-menu" @keydown.esc.stop.prevent="closeSortMenu(true)" @focusout="onSortFocusOut">
        <summary ref="sortTrigger" class="sort-trigger">
          <svg class="sort-icon" viewBox="0 0 20 20" fill="none" aria-hidden="true"><path d="M3 5h14M3 10h9M3 15h5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" /></svg>
          <span class="sr-only">{{ uiText.sort }}: </span>
          <span class="sort-value">{{ selectedSortLabel }}</span>
          <svg class="sort-chevron" viewBox="0 0 16 16" fill="none" aria-hidden="true"><path d="m4 6 4 4 4-4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" /></svg>
        </summary>
        <fieldset class="sort-options">
          <legend class="sr-only">{{ uiText.sort }}</legend>
          <span class="sort-heading" aria-hidden="true">{{ uiText.sort }}</span>
          <label v-for="option in sortOptions" :key="option.value" class="sort-option" :class="{ selected: sortOrder === option.value }">
            <input v-model="sortOrder" type="radio" name="plugin-sort" :value="option.value" :aria-label="option.label" @click="onSortOptionClick" @keyup.space="closeSortMenu(true)" @keydown.enter.prevent="closeSortMenu(true)" />
            <span>{{ option.label }}</span>
            <svg v-if="sortOrder === option.value" class="sort-check" viewBox="0 0 16 16" fill="none" aria-hidden="true"><path d="m3.5 8 3 3 6-6" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" /></svg>
          </label>
        </fieldset>
      </details>
    </div>

    <div v-if="filteredPlugins.length" class="grid">
      <article v-for="plugin in filteredPlugins" :key="plugin.Id" class="card" tabindex="0" @keydown.enter="() => (window.location.href = pluginDetailHref(plugin.Id))">
        <a :href="pluginDetailHref(plugin.Id)" class="card-link" :aria-label="plugin.LocalizedName"></a>

        <div class="card-header">
          <img v-if="plugin.IconUrl" :src="plugin.IconUrl" class="icon" alt="plugin icon" />
          <div v-else class="icon-placeholder">{{ plugin.IconEmoji || "🧩" }}</div>

          <div class="title-area">
            <h3 class="name">{{ plugin.LocalizedName }}</h3>
            <span class="author">{{ uiText.by }} {{ plugin.Author }}</span>
          </div>
        </div>

        <p class="description">{{ plugin.LocalizedDescription }}</p>

        <div class="footer">
          <div class="card-meta">
            <span class="version">v{{ plugin.Version }}</span>
            <span v-if="plugin.StarCount !== undefined" class="stars" :aria-label="`${uiText.stars}: ${plugin.StarCount}`"><span aria-hidden="true">★</span> {{ plugin.StarCount.toLocaleString("en-US") }}</span>
            <span v-if="plugin.DownloadCount !== undefined" class="downloads" :aria-label="`${uiText.downloads}: ${plugin.DownloadCount}`" :title="uiText.downloads"><span aria-hidden="true">↓</span> {{ plugin.DownloadCount.toLocaleString("en-US") }}</span>
          </div>

          <div class="actions">
            <a v-if="plugin.Website" :href="plugin.Website" class="secondary-btn" target="_blank" rel="noreferrer" @click.stop>
              {{ uiText.source }}
            </a>
            <a :href="installHref(plugin)" class="primary-btn" @click.stop>{{ uiText.install }}</a>
          </div>
        </div>
      </article>
    </div>

    <div v-else class="empty-state">{{ uiText.empty }}</div>
  </div>
</template>

<style scoped>
.gallery-container {
  margin-top: 20px;
}

.search-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 24px;
}

.sort-menu {
  position: relative;
  z-index: 4;
  flex: 0 0 224px;
  min-width: 0;
}

.sort-trigger {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 100%;
  margin: 0;
  padding: 14px 16px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 14px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.04), transparent), var(--vp-c-bg-alt);
  color: var(--vp-c-text-1);
  cursor: pointer;
  font-size: 14px;
  font-weight: 600;
  list-style: none;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.sort-trigger::-webkit-details-marker {
  display: none;
}

.sort-trigger:hover,
.sort-menu[open] .sort-trigger {
  border-color: color-mix(in srgb, var(--vp-c-brand-1) 45%, var(--vp-c-divider));
}

.sort-trigger:focus-visible {
  outline: 2px solid var(--vp-c-brand-1);
  outline-offset: 2px;
}

.sort-icon,
.sort-chevron,
.sort-check {
  flex: none;
  width: 16px;
  height: 16px;
}

.sort-icon,
.sort-chevron {
  color: var(--vp-c-text-2);
}

.sort-value {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sort-menu[open] .sort-chevron {
  transform: rotate(180deg);
}

.sort-options {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  width: max(100%, 210px);
  margin: 0;
  padding: 8px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 16px;
  background: var(--vp-c-bg-soft);
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.2);
}

.sort-heading {
  display: block;
  padding: 7px 10px 6px;
  color: var(--vp-c-text-3);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.sort-option {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 42px;
  padding: 0 10px;
  border-radius: 10px;
  color: var(--vp-c-text-2);
  cursor: pointer;
  font-size: 14px;
  font-weight: 500;
}

.sort-option:hover {
  background: var(--vp-c-bg-mute);
  color: var(--vp-c-text-1);
}

.sort-option.selected {
  background: color-mix(in srgb, var(--vp-c-brand-1) 12%, var(--vp-c-bg-mute));
  color: var(--vp-c-text-1);
}

.sort-option:focus-within {
  outline: 2px solid var(--vp-c-brand-1);
  outline-offset: -2px;
}

.sort-option input,
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  overflow: hidden;
  clip-path: inset(50%);
  white-space: nowrap;
}

.sort-check {
  color: var(--vp-c-brand-1);
}

@media (max-width: 640px) {
  .search-bar {
    flex-wrap: wrap;
  }

  .sort-menu {
    flex: 1 0 100%;
  }

  .sort-trigger {
    min-height: 48px;
  }

  .sort-options {
    left: 0;
    right: auto;
    width: 100%;
  }
}

.search-input {
  flex: 1;
  min-width: 0;
  width: 100%;
  padding: 14px 16px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 14px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.04), transparent), var(--vp-c-bg-alt);
  color: var(--vp-c-text-1);
  font-size: 16px;
  transition:
    border-color 0.2s,
    box-shadow 0.2s;
}

.search-input:focus {
  border-color: var(--vp-c-brand-1);
  outline: none;
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--vp-c-brand-1) 18%, transparent);
}

.grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 20px;
}

@media (max-width: 1024px) {
  .grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .grid {
    grid-template-columns: 1fr;
  }
}

.card {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 280px;
  padding: 22px;
  border: 1px solid color-mix(in srgb, var(--vp-c-divider) 84%, white 16%);
  border-radius: 22px;
  background:
    radial-gradient(circle at top right, rgba(100, 108, 255, 0.12), transparent 34%), linear-gradient(180deg, rgba(255, 255, 255, 0.04), transparent 32%), var(--vp-c-bg-soft);
  cursor: pointer;
  transition:
    transform 0.2s,
    border-color 0.2s,
    box-shadow 0.2s;
}

.card:hover {
  transform: translateY(-3px);
  border-color: color-mix(in srgb, var(--vp-c-brand-1) 45%, var(--vp-c-divider));
  box-shadow: 0 18px 34px rgba(15, 23, 42, 0.1);
}

.card-link {
  position: absolute;
  inset: 0;
  z-index: 2;
  border-radius: inherit;
}

.card:focus-visible {
  outline: 2px solid var(--vp-c-brand-1);
  outline-offset: 3px;
}

.actions,
.actions a {
  position: relative;
  z-index: 3;
}

.card-header {
  display: flex;
  align-items: center;
  margin-bottom: 14px;
}

.icon,
.icon-placeholder {
  width: 56px;
  height: 56px;
  border-radius: 16px;
  margin-right: 14px;
  flex-shrink: 0;
}

.icon {
  object-fit: cover;
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.14);
}

.icon-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in srgb, var(--vp-c-brand-1) 22%, var(--vp-c-bg-mute));
  font-size: 28px;
}

.title-area {
  flex: 1;
  min-width: 0;
}

.name {
  margin: 0;
  color: var(--vp-c-text-1);
  font-size: 19px;
  font-weight: 700;
  line-height: 1.2;
}

.author {
  display: inline-block;
  margin-top: 6px;
  color: var(--vp-c-text-2);
  font-size: 13px;
}

.description {
  flex: 1;
  margin: 0 0 16px;
  color: var(--vp-c-text-2);
  font-size: 14px;
  line-height: 1.6;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  overflow: hidden;
}

.card-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.stars,
.downloads {
  color: var(--vp-c-text-2);
  white-space: nowrap;
  font-size: 12px;
}

.stars span {
  color: #bb8417;
}

.version {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  padding: 0 10px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--vp-c-brand-1) 10%, var(--vp-c-bg-mute));
  color: var(--vp-c-text-2);
  font-size: 12px;
}

.footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: auto;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
}

.primary-btn,
.secondary-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 34px;
  padding: 0 14px;
  border-radius: 999px;
  text-decoration: none;
  font-size: 13px;
  font-weight: 600;
  transition:
    transform 0.2s,
    background-color 0.2s,
    border-color 0.2s;
}

.primary-btn {
  background: var(--vp-c-brand-1);
  color: var(--vp-c-bg);
}

.primary-btn:hover {
  background: var(--vp-c-brand-2);
  color: var(--vp-c-bg);
}

.secondary-btn {
  border: 1px solid var(--vp-c-divider);
  background: var(--vp-c-bg);
  color: var(--vp-c-text-1);
}

.secondary-btn:hover,
.primary-btn:hover {
  transform: translateY(-1px);
}

.secondary-btn:hover {
  color: var(--vp-c-text-1);
}

.empty-state {
  padding: 48px 24px;
  border: 1px dashed var(--vp-c-divider);
  border-radius: 20px;
  color: var(--vp-c-text-2);
  text-align: center;
}
</style>
