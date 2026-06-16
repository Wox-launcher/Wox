<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useData, withBase } from "vitepress";
import { fetchStorePlugins, formatStoreDate, localizePlugin, normalizeOsLabel, type LocalizedStorePluginManifest } from "./pluginStore";

const { lang } = useData();
const plugins = ref<LocalizedStorePluginManifest[]>([]);
const isLoading = ref(true);
const activeScreenshot = ref(0);
const currentPluginId = ref("");
const lastPluginStorageKey = "wox-store:last-plugin-id";
const pluginDetailBodyClass = "plugin-detail-page";

const uiText = computed(() => {
  const normalizedLang = (lang.value || "").toLowerCase();

  if (normalizedLang.startsWith("zh")) {
    return {
      backToStore: "返回插件商店",
      pluginStore: "插件商店",
      install: "安装插件",
      source: "Website",
      share: "分享",
      screenshots: "截图预览",
      noScreenshots: "这个插件还没有上传截图。",
      metadata: "元信息",
      compatibility: "兼容平台",
      author: "作者",
      version: "版本",
      runtime: "运行时",
      minWoxVersion: "最低 Wox 版本",
      pluginId: "插件 ID",
      created: "创建时间",
      updated: "更新时间",
      more: "更多插件",
      notFoundTitle: "没有找到这个插件",
      notFoundDescription: "当前链接里的插件 ID 不存在，或者该插件已经从商店移除。",
      loading: "正在加载插件详情...",
    };
  }

  return {
    backToStore: "Back to plugin store",
    pluginStore: "Plugin Store",
    install: "Install Plugin",
    source: "Website",
    share: "Share",
    screenshots: "Screenshots",
    noScreenshots: "This plugin does not provide screenshots yet.",
    metadata: "Metadata",
    compatibility: "Compatibility",
    author: "Author",
    version: "Version",
    runtime: "Runtime",
    minWoxVersion: "Min Wox Version",
    pluginId: "Plugin ID",
    created: "Created",
    updated: "Updated",
    more: "More plugins",
    notFoundTitle: "Plugin not found",
    notFoundDescription: "The plugin ID in this link does not exist or is no longer published in the store.",
    loading: "Loading plugin details...",
  };
});

const plugin = computed(() => {
  return plugins.value.find((item) => item.Id === currentPluginId.value) || null;
});

const screenshotUrls = computed(() => {
  return plugin.value?.ScreenshotUrls?.filter(Boolean) || [];
});

const platformItems = computed(() => {
  return (plugin.value?.SupportedOS || []).map((os) => {
    const normalizedOs = normalizeOsLabel(os);
    const iconKey = normalizedOs.toLowerCase();

    return {
      key: iconKey,
      label: normalizedOs,
    };
  });
});

const relatedPlugins = computed(() => {
  if (!plugin.value) return [];

  return plugins.value
    .filter((item) => item.Id !== plugin.value!.Id)
    .sort((left, right) => {
      const leftScore = Number(left.Author === plugin.value!.Author) * 2 + Number(left.Runtime === plugin.value!.Runtime);
      const rightScore = Number(right.Author === plugin.value!.Author) * 2 + Number(right.Runtime === plugin.value!.Runtime);
      return rightScore - leftScore;
    })
    .slice(0, 3);
});

const metadataRows = computed(() => {
  if (!plugin.value) return [];

  return [
    { label: uiText.value.author, value: plugin.value.Author },
    { label: uiText.value.version, value: plugin.value.Version ? `v${plugin.value.Version}` : "" },
    { label: uiText.value.runtime, value: plugin.value.Runtime?.toUpperCase() || "" },
    { label: uiText.value.minWoxVersion, value: plugin.value.MinWoxVersion || "" },
    { label: uiText.value.pluginId, value: plugin.value.Id },
    { label: uiText.value.created, value: formatStoreDate(plugin.value.DateCreated, lang.value) },
    { label: uiText.value.updated, value: formatStoreDate(plugin.value.DateUpdated, lang.value) },
  ].filter((item) => item.value);
});

const pluginStoreHref = computed(() => {
  const prefix = (lang.value || "").toLowerCase().startsWith("zh") ? "/zh/store/plugins.html" : "/store/plugins.html";
  return withBase(prefix);
});

const shareText = computed(() => {
  if (!plugin.value || typeof window === "undefined") return "";

  const pluginUrl = window.location.href;
  const description = plugin.value.LocalizedDescription?.trim();
  const normalizedLang = (lang.value || "").toLowerCase();

  if (normalizedLang.startsWith("zh")) {
    const summary = description ? `\n${description}` : "";
    return `我发现了一个很好用的 Wox 插件：${plugin.value.LocalizedName}${summary}\n\n#Wox #WoxLauncher #WoxLauncherPlugin\n${pluginUrl}`;
  }

  const summary = description ? `\n${description}` : "";
  return `I found a great Wox plugin: ${plugin.value.LocalizedName}${summary}\n\n#Wox #WoxLauncher #WoxLauncherPlugin\n${pluginUrl}`;
});

const shareHref = computed(() => {
  if (!shareText.value) return "";

  const shareIntentUrl = new URL("https://x.com/intent/post");
  shareIntentUrl.searchParams.set("text", shareText.value);
  return shareIntentUrl.toString();
});

function pluginDetailHref(pluginId: string) {
  const prefix = (lang.value || "").toLowerCase().startsWith("zh") ? "/zh/store/plugin.html" : "/store/plugin.html";
  return withBase(`${prefix}?id=${encodeURIComponent(pluginId)}`);
}

function installHref(pluginName: string) {
  return `wox://query?q=${encodeURIComponent(`wpm install ${pluginName}`)}`;
}

function platformIconPath(osKey: string) {
  if (osKey === "windows") {
    return "M3 4.2 11 3v8.5H3zm9.5-1.3L21 1.6v9.1h-8.5zM3 12.9h8v8.6L3 20.3zm9.5 0H21v9.5l-8.5-1.2z";
  }

  if (osKey === "macos") {
    return "M16.1 11.8c0-2.2 1.8-3.3 1.9-3.4-1-1.6-2.7-1.8-3.2-1.8-1.3-.1-2.5.8-3.2.8-.7 0-1.7-.8-2.7-.8-1.5 0-2.9.9-3.7 2.2-1.6 2.8-.4 6.9 1.2 9.1.8 1.1 1.7 2.3 3 2.2 1.2 0 1.6-.7 3-.7 1.4 0 1.8.7 3 .7 1.3 0 2.1-1.1 2.9-2.2.9-1.3 1.3-2.7 1.3-2.8-.1 0-3.5-1.4-3.5-5.3Zm-2.2-6.7c.6-.8 1-1.8.9-2.8-.9 0-2 .6-2.6 1.4-.6.7-1.1 1.8-.9 2.8 1 0 2-.5 2.6-1.4Z";
  }

  return "M12 2.2c2.2 0 4.1 1.5 4.6 3.6 1.6.6 2.7 2.1 2.7 3.9 0 1.2-.5 2.4-1.4 3.2v4.4c0 .5-.3 1-.8 1.2l-1.6.8-.8 2c-.2.5-.7.8-1.2.8h-3c-.5 0-1-.3-1.2-.8l-.8-2-1.6-.8c-.5-.2-.8-.7-.8-1.2V13c-.9-.8-1.4-2-1.4-3.2 0-1.8 1.1-3.3 2.7-3.9.5-2.1 2.4-3.7 4.6-3.7Zm-2.4 18.1.4 1h4l.4-1Zm-.4-9.2c-.7 0-1.2.6-1.2 1.2s.5 1.2 1.2 1.2 1.2-.5 1.2-1.2-.6-1.2-1.2-1.2Zm5.6 0c-.7 0-1.2.6-1.2 1.2s.5 1.2 1.2 1.2 1.2-.5 1.2-1.2-.5-1.2-1.2-1.2Z";
}

function syncCurrentPluginIdFromUrl(replaceHistory = false) {
  if (typeof window === "undefined") return;

  const queryPluginId = new URLSearchParams(window.location.search).get("id") || "";
  if (queryPluginId) {
    currentPluginId.value = queryPluginId;
    window.sessionStorage.setItem(lastPluginStorageKey, queryPluginId);
    return;
  }

  const lastPluginId = window.sessionStorage.getItem(lastPluginStorageKey) || "";
  currentPluginId.value = lastPluginId;

  if (!lastPluginId) return;

  const currentUrl = new URL(window.location.href);
  currentUrl.searchParams.set("id", lastPluginId);
  if (replaceHistory) {
    window.history.replaceState({}, "", currentUrl.toString());
  }
}

function openPluginDetail(pluginId: string) {
  if (typeof window === "undefined" || !pluginId) return;
  if (currentPluginId.value === pluginId) return;

  currentPluginId.value = pluginId;
  activeScreenshot.value = 0;
  window.sessionStorage.setItem(lastPluginStorageKey, pluginId);

  const currentUrl = new URL(window.location.href);
  currentUrl.searchParams.set("id", pluginId);
  window.history.pushState({}, "", currentUrl.toString());
  window.scrollTo({ top: 0, behavior: "smooth" });
}

async function loadPlugins() {
  isLoading.value = true;

  try {
    const storePlugins = await fetchStorePlugins();
    plugins.value = storePlugins.map((item) => localizePlugin(item, lang.value));
    syncCurrentPluginIdFromUrl(true);
    activeScreenshot.value = 0;
  } catch (error) {
    console.error(error);
  } finally {
    isLoading.value = false;
  }
}

function handlePopState() {
  syncCurrentPluginIdFromUrl();
  activeScreenshot.value = 0;
}

onMounted(() => {
  if (typeof document !== "undefined") {
    document.body.classList.add(pluginDetailBodyClass);
  }

  if (typeof window !== "undefined") {
    window.addEventListener("popstate", handlePopState);
  }

  loadPlugins();
});

onUnmounted(() => {
  if (typeof document !== "undefined") {
    document.body.classList.remove(pluginDetailBodyClass);
  }

  if (typeof window !== "undefined") {
    window.removeEventListener("popstate", handlePopState);
  }
});
</script>

<template>
  <div v-if="isLoading" class="status-shell">
    <div class="status-card">{{ uiText.loading }}</div>
  </div>

  <div v-else-if="plugin" class="plugin-page">
    <a :href="pluginStoreHref" class="back-link">{{ uiText.backToStore }}</a>

    <section class="hero-card">
      <div class="hero-copy">
        <div class="eyebrow">{{ uiText.pluginStore }}</div>

        <div class="hero-head">
          <img v-if="plugin.IconUrl" :src="plugin.IconUrl" class="hero-icon" alt="plugin icon" />
          <div v-else class="hero-icon hero-icon-placeholder">{{ plugin.IconEmoji || "🧩" }}</div>

          <div class="hero-title-wrap">
            <h1>{{ plugin.LocalizedName }}</h1>
            <p class="hero-description">{{ plugin.LocalizedDescription }}</p>
            <div class="hero-meta">
              <span>{{ uiText.author }} · {{ plugin.Author }}</span>
              <span v-if="plugin.Runtime">{{ uiText.runtime }} · {{ plugin.Runtime.toUpperCase() }}</span>
              <span v-if="plugin.DateUpdated">{{ uiText.updated }} · {{ formatStoreDate(plugin.DateUpdated, lang) }}</span>
            </div>
          </div>
        </div>

        <div class="hero-actions">
          <a :href="installHref(plugin.LocalizedName)" class="primary-action">
            <svg viewBox="0 0 24 24" aria-hidden="true" class="action-icon">
              <path
                d="M12 2.5a1 1 0 0 1 1 1v8.1l2.6-2.6a1 1 0 1 1 1.4 1.4l-4.3 4.3a1 1 0 0 1-1.4 0L7 10.4a1 1 0 0 1 1.4-1.4l2.6 2.6V3.5a1 1 0 0 1 1-1ZM5 15.5a1 1 0 0 1 1 1v2h12v-2a1 1 0 1 1 2 0v2.5a1.5 1.5 0 0 1-1.5 1.5h-13A1.5 1.5 0 0 1 4 19v-2.5a1 1 0 0 1 1-1Z"
                fill="currentColor"
              />
            </svg>
            <span>{{ uiText.install }}</span>
          </a>
          <!-- Keep Website as a single top-level action; the removed sidebar duplicate made metadata feel like navigation instead of factual details. -->
          <a v-if="plugin.Website" :href="plugin.Website" target="_blank" rel="noreferrer" class="secondary-action">
            <svg viewBox="0 0 24 24" aria-hidden="true" class="action-icon">
              <path
                d="M14 3h6a1 1 0 0 1 1 1v6a1 1 0 1 1-2 0V6.4l-8.8 8.8a1 1 0 0 1-1.4-1.4L17.6 5H14a1 1 0 1 1 0-2ZM6 5a2 2 0 0 0-2 2v11a2 2 0 0 0 2 2h11a2 2 0 0 0 2-2v-4a1 1 0 1 0-2 0v4H6V7h4a1 1 0 1 0 0-2H6Z"
                fill="currentColor"
              />
            </svg>
            <span>{{ uiText.source }}</span>
          </a>
          <a v-if="shareHref" :href="shareHref" target="_blank" rel="noopener noreferrer" class="secondary-action share-action">
            <svg viewBox="0 0 24 24" aria-hidden="true" class="action-icon">
              <path d="M18.9 2H22l-6.8 7.8L23.2 22h-6.3l-4.9-7.4L5.6 22H2.5l7.3-8.3L1.8 2h6.4l4.4 6.8L18.9 2Zm-1.1 18h1.8L7.2 3.9H5.3Z" fill="currentColor" />
            </svg>
            <span>{{ uiText.share }}</span>
          </a>
        </div>
      </div>

      <div class="hero-panel">
        <div class="panel-label">{{ uiText.compatibility }}</div>
        <div class="platform-list">
          <span v-for="item in platformItems" :key="item.key" class="platform-chip">
            <svg viewBox="0 0 24 24" aria-hidden="true" class="platform-icon">
              <path :d="platformIconPath(item.key)" fill="currentColor" />
            </svg>
            <span>{{ item.label }}</span>
          </span>
        </div>
        <div class="hero-stats">
          <div class="stat-card">
            <span class="stat-label">{{ uiText.version }}</span>
            <strong>v{{ plugin.Version }}</strong>
          </div>
          <div class="stat-card" v-if="plugin.MinWoxVersion">
            <span class="stat-label">{{ uiText.minWoxVersion }}</span>
            <strong>{{ plugin.MinWoxVersion }}</strong>
          </div>
          <div class="stat-card" v-if="plugin.Runtime">
            <span class="stat-label">{{ uiText.runtime }}</span>
            <strong>{{ plugin.Runtime.toUpperCase() }}</strong>
          </div>
        </div>
      </div>
    </section>

    <section class="content-grid">
      <div class="main-column">
        <div class="content-card">
          <div class="section-head">
            <h2>{{ uiText.screenshots }}</h2>
          </div>

          <div v-if="screenshotUrls.length" class="screenshot-shell">
            <img :src="screenshotUrls[activeScreenshot]" class="hero-shot" :alt="`${plugin.LocalizedName} screenshot ${activeScreenshot + 1}`" />

            <div v-if="screenshotUrls.length > 1" class="thumbnail-row">
              <button
                v-for="(url, index) in screenshotUrls"
                :key="url"
                type="button"
                class="thumbnail-btn"
                :class="{ active: index === activeScreenshot }"
                @click="activeScreenshot = index"
              >
                <img :src="url" :alt="`${plugin.LocalizedName} thumbnail ${index + 1}`" />
              </button>
            </div>
          </div>

          <div v-else class="empty-preview">{{ uiText.noScreenshots }}</div>
        </div>
      </div>

      <aside class="sidebar-column">
        <div class="content-card compact">
          <div class="section-head">
            <h2>{{ uiText.metadata }}</h2>
          </div>

          <dl class="metadata-list">
            <div v-for="item in metadataRows" :key="item.label" class="metadata-row">
              <dt>{{ item.label }}</dt>
              <dd>{{ item.value }}</dd>
            </div>
          </dl>
        </div>
      </aside>
    </section>

    <section v-if="relatedPlugins.length" class="related-section">
      <div class="section-head">
        <h2>{{ uiText.more }}</h2>
      </div>

      <div class="related-grid">
        <a v-for="item in relatedPlugins" :key="item.Id" :href="pluginDetailHref(item.Id)" class="related-card" @click.prevent="openPluginDetail(item.Id)">
          <div class="related-top">
            <img v-if="item.IconUrl" :src="item.IconUrl" class="related-icon" alt="plugin icon" />
            <div v-else class="related-icon related-placeholder">{{ item.IconEmoji || "🧩" }}</div>

            <div class="related-copy">
              <h3>{{ item.LocalizedName }}</h3>
              <span>{{ item.Author }}</span>
            </div>
          </div>

          <p>{{ item.LocalizedDescription }}</p>
        </a>
      </div>
    </section>
  </div>

  <div v-else class="status-shell">
    <div class="status-card">
      <h1>{{ uiText.notFoundTitle }}</h1>
      <p>{{ uiText.notFoundDescription }}</p>
      <a :href="pluginStoreHref" class="secondary-action">{{ uiText.backToStore }}</a>
    </div>
  </div>
</template>

<style scoped>
.plugin-page {
  --plugin-detail-card-padding: 30px;
  --plugin-detail-gap: 24px;
  --plugin-detail-columns: minmax(0, 1.45fr) minmax(300px, 0.9fr);
  margin: 12px 0 36px;
}

.back-link {
  display: inline-flex;
  align-items: center;
  margin-bottom: 18px;
  color: var(--vp-c-text-2);
  text-decoration: none;
  font-size: 14px;
}

.back-link:hover {
  color: var(--vp-c-brand-1);
}

.hero-card {
  display: grid;
  grid-template-columns: var(--plugin-detail-columns);
  gap: var(--plugin-detail-gap);
  padding: var(--plugin-detail-card-padding);
  border: 1px solid color-mix(in srgb, var(--vp-c-divider) 82%, white 18%);
  border-radius: 30px;
  background:
    radial-gradient(circle at top left, rgba(100, 108, 255, 0.18), transparent 34%), radial-gradient(circle at bottom right, rgba(56, 189, 248, 0.12), transparent 28%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.06), transparent 30%), var(--vp-c-bg-soft);
  box-shadow: 0 28px 50px rgba(15, 23, 42, 0.08);
}

.eyebrow {
  display: inline-flex;
  align-items: center;
  min-height: 30px;
  margin-bottom: 18px;
  padding: 0 12px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--vp-c-brand-1) 14%, var(--vp-c-bg-mute));
  color: var(--vp-c-brand-1);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.hero-head {
  display: flex;
  align-items: flex-start;
  gap: 18px;
}

.hero-icon {
  width: 88px;
  height: 88px;
  border-radius: 24px;
  object-fit: cover;
  flex-shrink: 0;
  box-shadow: 0 20px 36px rgba(15, 23, 42, 0.16);
}

.hero-icon-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in srgb, var(--vp-c-brand-1) 20%, var(--vp-c-bg-mute));
  font-size: 42px;
}

.hero-title-wrap {
  min-width: 0;
}

.hero-title-wrap h1 {
  margin: 0;
  font-size: clamp(34px, 5vw, 48px);
  line-height: 1.02;
  letter-spacing: -0.03em;
}

.hero-description {
  margin: 16px 0 0;
  color: var(--vp-c-text-2);
  font-size: 17px;
  line-height: 1.75;
  max-width: 720px;
}

.hero-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px 16px;
  margin-top: 16px;
  color: var(--vp-c-text-2);
  font-size: 13px;
}

.hero-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 24px;
}

.primary-action,
.secondary-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 42px;
  padding: 0 18px;
  border-radius: 999px;
  border: 1px solid transparent;
  text-decoration: none;
  font-size: 14px;
  font-weight: 600;
  transition:
    transform 0.2s,
    border-color 0.2s,
    background-color 0.2s;
}

.primary-action {
  background: var(--vp-c-brand-1);
  color: white;
}

.secondary-action {
  background: var(--vp-c-bg);
  color: var(--vp-c-text-1);
  border-color: var(--vp-c-divider);
}

.primary-action:hover,
.secondary-action:hover {
  transform: translateY(-1px);
}

.primary-action:hover {
  color: white;
}

.secondary-action:hover {
  color: var(--vp-c-text-1);
}

.action-icon {
  width: 15px;
  height: 15px;
  flex-shrink: 0;
}

.hero-panel {
  padding: 22px;
  border-radius: 24px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.05), transparent 36%), color-mix(in srgb, var(--vp-c-bg) 90%, white 10%);
  border: 1px solid color-mix(in srgb, var(--vp-c-divider) 84%, white 16%);
}

.panel-label,
.section-head h2 {
  margin: 0;
  color: var(--vp-c-text-1);
  font-size: 18px;
  font-weight: 700;
}

.platform-list {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 18px;
}

.platform-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 34px;
  padding: 0 12px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--vp-c-brand-1) 12%, var(--vp-c-bg-mute));
  color: var(--vp-c-text-1);
  font-size: 13px;
  font-weight: 600;
}

.platform-icon {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
}

.hero-stats {
  display: grid;
  gap: 12px;
  margin-top: 22px;
}

.stat-card {
  padding: 16px;
  border-radius: 18px;
  background: color-mix(in srgb, var(--vp-c-bg-soft) 70%, transparent);
  border: 1px solid color-mix(in srgb, var(--vp-c-divider) 88%, white 12%);
}

.stat-label {
  display: block;
  margin-bottom: 6px;
  color: var(--vp-c-text-2);
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.content-grid {
  display: grid;
  /* Match the hero card's inner grid so the Compatibility panel and Metadata card keep the same right-rail width on desktop. */
  grid-template-columns: var(--plugin-detail-columns);
  gap: var(--plugin-detail-gap);
  margin-top: 24px;
  box-sizing: border-box;
}

.main-column,
.sidebar-column {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.content-card {
  padding: 24px;
  border: 1px solid color-mix(in srgb, var(--vp-c-divider) 82%, white 18%);
  border-radius: 26px;
  background: var(--vp-c-bg-soft);
}

.content-card.compact {
  padding: 22px;
}

.section-head {
  margin-bottom: 18px;
}

.section-head h2 {
  border-top: 0;
  padding-top: 0;
  margin: 0;
  font-size: 20px;
  letter-spacing: normal;
  line-height: 1.2;
}

.screenshot-shell {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.hero-shot {
  width: 100%;
  border-radius: 20px;
  border: 1px solid color-mix(in srgb, var(--vp-c-divider) 82%, white 18%);
  background: var(--vp-c-bg-alt);
}

.thumbnail-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(92px, 1fr));
  gap: 12px;
}

.thumbnail-btn {
  padding: 0;
  border: 1px solid var(--vp-c-divider);
  border-radius: 16px;
  overflow: hidden;
  background: transparent;
  cursor: pointer;
  transition:
    transform 0.2s,
    border-color 0.2s;
}

.thumbnail-btn img {
  display: block;
  width: 100%;
  aspect-ratio: 16 / 10;
  object-fit: cover;
}

.thumbnail-btn.active {
  border-color: var(--vp-c-brand-1);
  transform: translateY(-1px);
}

.empty-preview {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 260px;
  border: 1px dashed var(--vp-c-divider);
  border-radius: 20px;
  color: var(--vp-c-text-2);
  text-align: center;
}

.metadata-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
  margin: 0;
}

.metadata-row {
  display: grid;
  grid-template-columns: minmax(0, 120px) minmax(0, 1fr);
  gap: 14px;
  margin: 0;
}

.metadata-row dt {
  color: var(--vp-c-text-2);
  font-size: 13px;
}

.metadata-row dd {
  margin: 0;
  color: var(--vp-c-text-1);
  font-size: 13px;
  word-break: break-word;
  text-align: right;
}

.related-section {
  margin-top: 28px;
}

.related-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 18px;
}

.related-card {
  display: block;
  padding: 20px;
  border: 1px solid color-mix(in srgb, var(--vp-c-divider) 82%, white 18%);
  border-radius: 22px;
  background: radial-gradient(circle at top right, rgba(100, 108, 255, 0.1), transparent 30%), var(--vp-c-bg-soft);
  cursor: pointer;
  transition:
    transform 0.2s,
    border-color 0.2s;
  text-decoration: none;
}

.related-card:hover {
  transform: translateY(-2px);
  border-color: color-mix(in srgb, var(--vp-c-brand-1) 45%, var(--vp-c-divider));
}

.related-top {
  display: flex;
  gap: 12px;
  margin-bottom: 14px;
}

.related-icon {
  width: 48px;
  height: 48px;
  border-radius: 14px;
  object-fit: cover;
  flex-shrink: 0;
}

.related-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in srgb, var(--vp-c-brand-1) 16%, var(--vp-c-bg-mute));
  font-size: 24px;
}

.related-copy h3 {
  margin: 0;
  font-size: 16px;
}

.related-copy span {
  display: inline-block;
  margin-top: 4px;
  color: var(--vp-c-text-2);
  font-size: 12px;
}

.related-card p {
  margin: 0;
  color: var(--vp-c-text-2);
  font-size: 14px;
  line-height: 1.6;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  overflow: hidden;
}

.status-shell {
  display: flex;
  justify-content: center;
  padding: 40px 0;
}

.status-card {
  width: min(680px, 100%);
  padding: 28px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 24px;
  background: var(--vp-c-bg-soft);
  text-align: center;
}

.status-card h1 {
  margin: 0 0 12px;
}

.status-card p {
  margin: 0 0 18px;
  color: var(--vp-c-text-2);
}

@media (max-width: 1180px) {
  .hero-card,
  .content-grid {
    grid-template-columns: 1fr;
  }

  .content-grid {
    padding-inline: 0;
  }

  .related-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .hero-card,
  .content-card,
  .content-card.compact,
  .status-card {
    padding: 20px;
    border-radius: 22px;
  }

  .hero-head {
    flex-direction: column;
  }

  .hero-icon {
    width: 74px;
    height: 74px;
    border-radius: 20px;
  }

  .metadata-row {
    grid-template-columns: 1fr;
    gap: 4px;
  }

  .metadata-row dd {
    text-align: left;
  }
}
</style>
