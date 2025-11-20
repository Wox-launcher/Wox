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
              > {{ theme.ThemeName.toLowerCase().substring(0, 10) }}
            </div>
            <div class="preview-results">
              <div
                class="preview-result active"
                :style="{
                  backgroundColor: theme.ResultItemActiveBackgroundColor,
                  borderRadius: (theme.ResultItemBorderRadius || 0) + 'px',
                }"
              >
                <div class="preview-icon"></div>
                <div class="preview-text" :style="{ color: theme.ResultItemActiveTitleColor }">
                  {{ theme.ThemeName }}
                </div>
              </div>
              <div
                class="preview-result"
                :style="{
                  borderRadius: (theme.ResultItemBorderRadius || 0) + 'px',
                }"
              >
                <div class="preview-icon inactive"></div>
                <div class="preview-text" :style="{ color: theme.ResultItemTitleColor }">Another Theme</div>
              </div>
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
          <div class="color-palette">
            <div class="swatches">
              <div class="color-swatch" :style="{ backgroundColor: theme.AppBackgroundColor }" title="Background"></div>
              <div class="color-swatch" :style="{ backgroundColor: theme.ResultItemActiveBackgroundColor }" title="Accent"></div>
              <div class="color-swatch" :style="{ backgroundColor: theme.ResultItemTitleColor }" title="Text"></div>
              <div class="color-swatch" :style="{ backgroundColor: theme.QueryBoxBackgroundColor }" title="Query Box"></div>
            </div>
            <a :href="`wox://query?q=theme ${theme.ThemeName}`" class="install-btn" @click.stop>Install</a>
          </div>
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
  height: 140px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
}

.preview-content {
  width: 100%;
  max-width: 240px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.preview-query {
  padding: 6px 10px;
  font-family: monospace;
  font-size: 11px;
  display: flex;
  align-items: center;
}

.preview-results {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.preview-result {
  padding: 6px 10px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.preview-icon {
  width: 12px;
  height: 12px;
  border-radius: 3px;
  background-color: currentColor;
  opacity: 0.8;
}

.preview-icon.inactive {
  opacity: 0.4;
}

.preview-text {
  font-size: 11px;
  font-weight: 500;
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
  align-items: flex-start;
  margin-bottom: 8px;
}

.actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.install-btn {
  font-size: 12px;
  font-weight: 500;
  color: var(--vp-c-brand);
  text-decoration: none;
  padding: 2px 8px;
  border-radius: 12px;
  background-color: var(--vp-c-brand-dimm);
  transition: background-color 0.2s;
}

.install-btn:hover {
  background-color: var(--vp-c-brand-soft);
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
  font-size: 14px;
  color: var(--vp-c-text-2);
  margin: 0 0 16px 0;
  line-height: 1.5;
  flex: 1;
}

.color-palette {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  margin-top: auto;
  padding-top: 12px;
  border-top: 1px solid var(--vp-c-divider);
}

.swatches {
  display: flex;
  gap: 8px;
}

.color-swatch {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  border: 1px solid rgba(128, 128, 128, 0.2);
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}
</style>
