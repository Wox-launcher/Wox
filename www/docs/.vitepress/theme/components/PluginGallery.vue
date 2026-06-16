<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useData, withBase } from "vitepress";
import { fetchStorePlugins, localizePlugin, type LocalizedStorePluginManifest } from "./pluginStore";

const plugins = ref<LocalizedStorePluginManifest[]>([]);
const searchQuery = ref("");
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
    };
  }

  return {
    searchPlaceholder: "Search plugins...",
    by: "by",
    install: "Install",
    source: "Website",
    empty: "No plugins match your search.",
  };
});

onMounted(async () => {
  try {
    const storePlugins = await fetchStorePlugins();
    plugins.value = storePlugins.map((plugin) => localizePlugin(plugin, lang.value));
  } catch (error) {
    console.error(error);
  }
});

const filteredPlugins = computed(() => {
  const query = searchQuery.value.trim().toLowerCase();
  if (!query) return plugins.value;

  return plugins.value.filter((plugin) => {
    return [plugin.LocalizedName, plugin.LocalizedDescription, plugin.Author, plugin.Runtime].filter(Boolean).some((value) => value!.toLowerCase().includes(query));
  });
});

function pluginDetailHref(pluginId: string) {
  const prefix = (lang.value || "").toLowerCase().startsWith("zh") ? "/zh/store/plugin.html" : "/store/plugin.html";
  return withBase(`${prefix}?id=${encodeURIComponent(pluginId)}`);
}

function installHref(pluginName: string) {
  return `wox://query?q=${encodeURIComponent(`wpm install ${pluginName}`)}`;
}
</script>

<template>
  <div class="gallery-container">
    <div class="search-bar">
      <input v-model="searchQuery" type="text" :placeholder="uiText.searchPlaceholder" class="search-input" />
    </div>

    <div v-if="filteredPlugins.length" class="grid">
      <article
        v-for="plugin in filteredPlugins"
        :key="plugin.Id"
        class="card"
        tabindex="0"
        @keydown.enter="() => (window.location.href = pluginDetailHref(plugin.Id))"
      >
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
          <span class="version">v{{ plugin.Version }}</span>

          <div class="actions">
            <a v-if="plugin.Website" :href="plugin.Website" class="secondary-btn" target="_blank" rel="noreferrer" @click.stop>
              {{ uiText.source }}
            </a>
            <a :href="installHref(plugin.LocalizedName)" class="primary-btn" @click.stop>{{ uiText.install }}</a>
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
  margin-bottom: 24px;
}

.search-input {
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
  color: white;
}

.primary-btn:hover {
  background: var(--vp-c-brand-2);
  color: white;
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
