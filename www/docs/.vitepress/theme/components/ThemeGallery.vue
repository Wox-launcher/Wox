<script setup>
import { computed, ref, onMounted } from "vue";

const themes = ref([]);
const searchQuery = ref("");

onMounted(async () => {
  try {
    const res = await fetch("https://raw.githubusercontent.com/Wox-launcher/Wox/master/store-theme.json");
    themes.value = await res.json();
  } catch (e) {
    console.error(e);
  }
});

const filteredThemes = computed(() => {
  return themes.value.filter((t) => t.ThemeName.toLowerCase().includes(searchQuery.value.toLowerCase()) || t.Description.toLowerCase().includes(searchQuery.value.toLowerCase()));
});
</script>

<template>
  <div class="gallery-container">
    <div class="search-bar">
      <input v-model="searchQuery" type="text" placeholder="Search themes..." class="search-input" />
    </div>

    <div class="grid">
      <div v-for="theme in filteredThemes" :key="theme.ThemeId" class="card">
        <div class="preview" :style="{ backgroundColor: theme.AppBackgroundColor || '#282a36' }">
          <div class="preview-content">
            <div
              class="preview-query"
              :style="{
                backgroundColor: theme.QueryBoxBackgroundColor,
                color: theme.QueryBoxFontColor,
                borderRadius: (theme.QueryBoxBorderRadius || 4) + 'px',
              }"
            >
              > wox
            </div>
            <div
              class="preview-result"
              :style="{
                color: theme.ResultItemTitleColor,
              }"
            >
              Result Item
            </div>
          </div>
        </div>
        <div class="card-body">
          <div class="header">
            <h3 class="name">{{ theme.ThemeName }}</h3>
            <span class="version">v{{ theme.Version }}</span>
          </div>
          <p class="author">by {{ theme.ThemeAuthor }}</p>
          <p class="description">{{ theme.Description }}</p>
        </div>
      </div>
    </div>
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
  padding: 12px 16px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 8px;
  background-color: var(--vp-c-bg-alt);
  color: var(--vp-c-text-1);
  font-size: 16px;
  transition: border-color 0.2s;
}

.search-input:focus {
  border-color: var(--vp-c-brand);
  outline: none;
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 20px;
}

.card {
  background-color: var(--vp-c-bg-soft);
  border: 1px solid var(--vp-c-divider);
  border-radius: 12px;
  overflow: hidden;
  transition: transform 0.2s, box-shadow 0.2s;
  display: flex;
  flex-direction: column;
}

.card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.1);
  border-color: var(--vp-c-brand);
}

.preview {
  height: 120px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.preview-content {
  width: 100%;
  max-width: 200px;
}

.preview-query {
  padding: 8px 12px;
  font-family: monospace;
  font-size: 12px;
  margin-bottom: 8px;
}

.preview-result {
  padding: 4px 12px;
  font-size: 12px;
  opacity: 0.8;
}

.card-body {
  padding: 16px;
  flex: 1;
  display: flex;
  flex-direction: column;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.name {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--vp-c-text-1);
}

.version {
  font-size: 11px;
  color: var(--vp-c-text-3);
  background-color: var(--vp-c-bg-mute);
  padding: 2px 6px;
  border-radius: 4px;
}

.author {
  font-size: 12px;
  color: var(--vp-c-text-2);
  margin: 0 0 8px 0;
}

.description {
  font-size: 13px;
  color: var(--vp-c-text-2);
  margin: 0;
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
</style>
