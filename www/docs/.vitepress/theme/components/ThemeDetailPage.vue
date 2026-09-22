<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useData, withBase } from "vitepress";
import ThemeSwatch from "./ThemeSwatch.vue";
import { englishThemeName, fetchStoreThemes, formatStoreDate, hasThemeSwatch, localizeTheme, type LocalizedStoreThemeManifest } from "./themeStore";

const { lang } = useData();
const themes = ref<LocalizedStoreThemeManifest[]>([]);
const isLoading = ref(true);
const activeScreenshot = ref(0);
const currentThemeId = ref("");
const lastThemeStorageKey = "wox-store:last-theme-id";
const themeDetailBodyClass = "theme-detail-page";

const uiText = computed(() => {
  const normalizedLang = (lang.value || "").toLowerCase();

  if (normalizedLang.startsWith("zh")) {
    return {
      backToStore: "返回主题商店",
      themeStore: "主题商店",
      install: "安装主题",
      imageBadge: "贴图",
      imageMemory: "贴图 · 会增加内存占用",
      source: "Website",
      share: "分享",
      screenshots: "截图预览",
      noScreenshots: "这个主题还没有上传截图。",
      metadata: "元信息",
      author: "作者",
      version: "版本",
      minWoxVersion: "最低 Wox 版本",
      themeId: "主题 ID",
      created: "创建时间",
      updated: "更新时间",
      more: "更多主题",
      notFoundTitle: "没有找到这个主题",
      notFoundDescription: "当前链接里的主题 ID 不存在，或者该主题已经从商店移除。",
      loading: "正在加载主题详情...",
    };
  }

  return {
    backToStore: "Back to theme store",
    themeStore: "Theme Store",
    install: "Install Theme",
    imageBadge: "Image",
    imageMemory: "Image · uses more memory",
    source: "Website",
    share: "Share",
    screenshots: "Screenshots",
    noScreenshots: "This theme does not provide screenshots yet.",
    metadata: "Metadata",
    author: "Author",
    version: "Version",
    minWoxVersion: "Min Wox Version",
    themeId: "Theme ID",
    created: "Created",
    updated: "Updated",
    more: "More themes",
    notFoundTitle: "Theme not found",
    notFoundDescription: "The theme ID in this link does not exist or is no longer published in the store.",
    loading: "Loading theme details...",
  };
});

const theme = computed(() => {
  return themes.value.find((item) => item.Id === currentThemeId.value) || null;
});

const screenshotUrls = computed(() => {
  return theme.value?.ScreenshotUrls?.filter(Boolean) || [];
});

const relatedThemes = computed(() => {
  if (!theme.value) return [];

  return themes.value
    .filter((item) => item.Id !== theme.value!.Id)
    .sort((left, right) => Number(right.Author === theme.value!.Author) - Number(left.Author === theme.value!.Author))
    .slice(0, 3);
});

const metadataRows = computed(() => {
  if (!theme.value) return [];

  return [
    { label: uiText.value.author, value: theme.value.Author },
    { label: uiText.value.version, value: theme.value.Version ? `v${theme.value.Version}` : "" },
    { label: uiText.value.minWoxVersion, value: theme.value.MinWoxVersion || "" },
    { label: uiText.value.themeId, value: theme.value.Id },
    { label: uiText.value.created, value: formatStoreDate(theme.value.DateCreated, lang.value) },
    { label: uiText.value.updated, value: formatStoreDate(theme.value.DateUpdated, lang.value) },
  ].filter((item) => item.value);
});

const themeStoreHref = computed(() => {
  const prefix = (lang.value || "").toLowerCase().startsWith("zh") ? "/zh/store/themes.html" : "/store/themes.html";
  return withBase(prefix);
});

const shareText = computed(() => {
  if (!theme.value || typeof window === "undefined") return "";

  const themeUrl = window.location.href;
  const description = theme.value.LocalizedDescription?.trim();
  const preview = screenshotUrls.value[activeScreenshot.value] || screenshotUrls.value[0] || "";
  const previewLine = preview ? `\n${preview}` : "";
  const normalizedLang = (lang.value || "").toLowerCase();

  if (normalizedLang.startsWith("zh")) {
    const summary = description ? `\n${description}` : "";
    return `我发现了一个很好看的 Wox 主题：${theme.value.LocalizedName}${summary}\n\n#Wox #WoxLauncher\n${themeUrl}${previewLine}`;
  }

  const summary = description ? `\n${description}` : "";
  return `I found a great Wox theme: ${theme.value.LocalizedName}${summary}\n\n#Wox #WoxLauncher\n${themeUrl}${previewLine}`;
});

const shareHref = computed(() => {
  if (!shareText.value) return "";

  const shareIntentUrl = new URL("https://x.com/intent/post");
  shareIntentUrl.searchParams.set("text", shareText.value);
  return shareIntentUrl.toString();
});

function themeDetailHref(themeId: string) {
  const prefix = (lang.value || "").toLowerCase().startsWith("zh") ? "/zh/store/theme.html" : "/store/theme.html";
  return withBase(`${prefix}?id=${encodeURIComponent(themeId)}`);
}

function installHref(theme: LocalizedStoreThemeManifest) {
  return `wox://query?q=${encodeURIComponent(`theme ${englishThemeName(theme)}`)}`;
}

function syncCurrentThemeIdFromUrl(replaceHistory = false) {
  if (typeof window === "undefined") return;

  const queryThemeId = new URLSearchParams(window.location.search).get("id") || "";
  if (queryThemeId) {
    currentThemeId.value = queryThemeId;
    window.sessionStorage.setItem(lastThemeStorageKey, queryThemeId);
    return;
  }

  const lastThemeId = window.sessionStorage.getItem(lastThemeStorageKey) || "";
  currentThemeId.value = lastThemeId;

  if (!lastThemeId) return;

  const currentUrl = new URL(window.location.href);
  currentUrl.searchParams.set("id", lastThemeId);
  if (replaceHistory) {
    window.history.replaceState({}, "", currentUrl.toString());
  }
}

function openThemeDetail(themeId: string) {
  if (typeof window === "undefined" || !themeId) return;
  if (currentThemeId.value === themeId) return;

  currentThemeId.value = themeId;
  activeScreenshot.value = 0;
  window.sessionStorage.setItem(lastThemeStorageKey, themeId);

  const currentUrl = new URL(window.location.href);
  currentUrl.searchParams.set("id", themeId);
  window.history.pushState({}, "", currentUrl.toString());
  window.scrollTo({ top: 0, behavior: "smooth" });
}

async function loadThemes() {
  isLoading.value = true;

  try {
    const storeThemes = await fetchStoreThemes();
    themes.value = storeThemes.map((item) => localizeTheme(item, lang.value));
    syncCurrentThemeIdFromUrl(true);
    activeScreenshot.value = 0;
  } catch (error) {
    console.error(error);
  } finally {
    isLoading.value = false;
  }
}

function handlePopState() {
  syncCurrentThemeIdFromUrl();
  activeScreenshot.value = 0;
}

onMounted(() => {
  if (typeof document !== "undefined") {
    document.body.classList.add(themeDetailBodyClass);
  }

  if (typeof window !== "undefined") {
    window.addEventListener("popstate", handlePopState);
  }

  loadThemes();
});

onUnmounted(() => {
  if (typeof document !== "undefined") {
    document.body.classList.remove(themeDetailBodyClass);
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

  <div v-else-if="theme" class="theme-page">
    <a :href="themeStoreHref" class="back-link">{{ uiText.backToStore }}</a>

    <section class="hero-card">
      <div class="hero-copy">
        <div class="eyebrow">{{ uiText.themeStore }}</div>

        <div class="hero-head">
          <ThemeSwatch v-if="hasThemeSwatch(theme.IconColors)" class="hero-icon" :colors="theme.IconColors" :size="88" />
          <div v-else class="hero-icon hero-icon-placeholder">◐</div>

          <div class="hero-title-wrap">
            <h1>{{ theme.LocalizedName }}</h1>
            <p class="hero-description">{{ theme.LocalizedDescription }}</p>
            <div class="hero-meta">
              <span>{{ uiText.author }} · {{ theme.Author }}</span>
              <span v-if="theme.DateUpdated">{{ uiText.updated }} · {{ formatStoreDate(theme.DateUpdated, lang) }}</span>
              <span v-if="theme.ImageTheme" class="image-badge">{{ uiText.imageMemory }}</span>
            </div>
          </div>
        </div>

        <div class="hero-actions">
          <a :href="installHref(theme)" class="primary-action">
            <svg viewBox="0 0 24 24" aria-hidden="true" class="action-icon">
              <path
                d="M12 2.5a1 1 0 0 1 1 1v8.1l2.6-2.6a1 1 0 1 1 1.4 1.4l-4.3 4.3a1 1 0 0 1-1.4 0L7 10.4a1 1 0 0 1 1.4-1.4l2.6 2.6V3.5a1 1 0 0 1 1-1ZM5 15.5a1 1 0 0 1 1 1v2h12v-2a1 1 0 1 1 2 0v2.5a1.5 1.5 0 0 1-1.5 1.5h-13A1.5 1.5 0 0 1 4 19v-2.5a1 1 0 0 1 1-1Z"
                fill="currentColor"
              />
            </svg>
            <span>{{ uiText.install }}</span>
          </a>
          <!-- Keep Website as a single top-level action; the removed sidebar duplicate made metadata feel like navigation instead of factual details. -->
          <a v-if="theme.Website" :href="theme.Website" target="_blank" rel="noreferrer" class="secondary-action">
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
        <div class="hero-stats">
          <div class="stat-card">
            <span class="stat-label">{{ uiText.version }}</span>
            <strong>v{{ theme.Version }}</strong>
          </div>
          <div class="stat-card" v-if="theme.MinWoxVersion">
            <span class="stat-label">{{ uiText.minWoxVersion }}</span>
            <strong>{{ theme.MinWoxVersion }}</strong>
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
            <img :src="screenshotUrls[activeScreenshot]" class="hero-shot" :alt="`${theme.LocalizedName} screenshot ${activeScreenshot + 1}`" />

            <div v-if="screenshotUrls.length > 1" class="thumbnail-row">
              <button
                v-for="(url, index) in screenshotUrls"
                :key="url"
                type="button"
                class="thumbnail-btn"
                :class="{ active: index === activeScreenshot }"
                @click="activeScreenshot = index"
              >
                <img :src="url" :alt="`${theme.LocalizedName} thumbnail ${index + 1}`" />
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

    <section v-if="relatedThemes.length" class="related-section">
      <div class="section-head">
        <h2>{{ uiText.more }}</h2>
      </div>

      <div class="related-grid">
        <a v-for="item in relatedThemes" :key="item.Id" :href="themeDetailHref(item.Id)" class="related-card" @click.prevent="openThemeDetail(item.Id)">
          <div class="related-top">
            <ThemeSwatch v-if="hasThemeSwatch(item.IconColors)" class="related-icon" :colors="item.IconColors" :size="48" />
            <div v-else class="related-icon related-placeholder">◐</div>

            <div class="related-copy">
              <div class="name-row">
                <h3>{{ item.LocalizedName }}</h3>
                <span v-if="item.ImageTheme" class="image-badge">{{ uiText.imageBadge }}</span>
              </div>
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
      <a :href="themeStoreHref" class="secondary-action">{{ uiText.backToStore }}</a>
    </div>
  </div>
</template>

<style scoped>
.theme-page {
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

.name-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.image-badge {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  min-height: 26px;
  padding: 0 10px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--vp-c-warning-1) 16%, transparent);
  color: var(--vp-c-warning-1);
  font-size: 13px;
  font-weight: 650;
  line-height: 1;
}

.hero-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
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
  color: var(--vp-c-bg);
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
  color: var(--vp-c-bg);
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
