<script setup>
import { computed, ref, onMounted } from "vue";

const plugins = ref([]);
const searchQuery = ref("");

onMounted(async () => {
  try {
    const res = await fetch("https://raw.githubusercontent.com/Wox-launcher/Wox/master/store-plugin.json");
    plugins.value = await res.json();
  } catch (e) {
    console.error(e);
  }
});

const filteredPlugins = computed(() => {
  return plugins.value.filter((p) => p.Name.toLowerCase().includes(searchQuery.value.toLowerCase()) || p.Description.toLowerCase().includes(searchQuery.value.toLowerCase()));
});

const openLink = (url) => {
  if (url) window.open(url, "_blank");
};
</script>

<template>
  <div class="gallery-container">
    <div class="search-bar">
      <input v-model="searchQuery" type="text" placeholder="Search plugins..." class="search-input" />
    </div>

    <div class="grid">
      <div v-for="plugin in filteredPlugins" :key="plugin.Id" class="card" @click="openLink(plugin.Website)">
        <div class="card-header">
          <img v-if="plugin.IconUrl" :src="plugin.IconUrl" class="icon" alt="icon" />
          <div v-else class="icon-placeholder">{{ plugin.IconEmoji || "🧩" }}</div>
          <div class="title-area">
            <h3 class="name">{{ plugin.Name }}</h3>
            <span class="author">by {{ plugin.Author }}</span>
          </div>
        </div>
        <p class="description">{{ plugin.Description }}</p>
        <div class="footer">
          <span class="version">v{{ plugin.Version }}</span>
          <a :href="`wox://query?q=wpm install ${plugin.DownloadUrl}`" class="download-btn" @click.stop>Install</a>
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
  padding: 20px;
  transition: transform 0.2s, box-shadow 0.2s;
  cursor: pointer;
  display: flex;
  flex-direction: column;
}

.card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.1);
  border-color: var(--vp-c-brand);
}

.card-header {
  display: flex;
  align-items: center;
  margin-bottom: 12px;
}

.icon {
  width: 48px;
  height: 48px;
  border-radius: 10px;
  margin-right: 12px;
  object-fit: cover;
}

.icon-placeholder {
  width: 48px;
  height: 48px;
  border-radius: 10px;
  margin-right: 12px;
  background-color: var(--vp-c-bg-mute);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
}

.title-area {
  flex: 1;
  overflow: hidden;
}

.name {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--vp-c-text-1);
}

.author {
  font-size: 12px;
  color: var(--vp-c-text-2);
}

.description {
  font-size: 14px;
  color: var(--vp-c-text-2);
  margin: 0 0 16px 0;
  line-height: 1.5;
  flex: 1;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: auto;
}

.version {
  font-size: 12px;
  color: var(--vp-c-text-3);
  background-color: var(--vp-c-bg-mute);
  padding: 2px 6px;
  border-radius: 4px;
}

.download-btn {
  font-size: 13px;
  font-weight: 500;
  color: var(--vp-c-brand);
  text-decoration: none;
  padding: 4px 12px;
  border-radius: 16px;
  background-color: var(--vp-c-brand-dimm);
  transition: background-color 0.2s;
}

.download-btn:hover {
  background-color: var(--vp-c-brand-soft);
}
</style>
