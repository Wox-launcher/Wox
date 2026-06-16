import storePluginData from "../../../../../store-plugin.json";

export type StorePluginManifest = {
  Id: string;
  Name: string;
  Author: string;
  Version: string;
  MinWoxVersion?: string;
  Runtime?: string;
  Description: string;
  IconUrl?: string;
  IconEmoji?: string;
  Website?: string;
  DownloadUrl?: string;
  ScreenshotUrls?: string[];
  SupportedOS?: string[];
  DateCreated?: string;
  DateUpdated?: string;
  I18n?: Record<string, Record<string, string>>;
};

export type LocalizedStorePluginManifest = StorePluginManifest & {
  LocalizedName: string;
  LocalizedDescription: string;
};

export async function fetchStorePlugins() {
  return storePluginData as StorePluginManifest[];
}

export function getCurrentLangCode(lang: string) {
  const normalizedLang = (lang || "").toLowerCase();

  if (normalizedLang.startsWith("zh")) return "zh_CN";
  if (normalizedLang.startsWith("pt")) return "pt_BR";
  if (normalizedLang.startsWith("ru")) return "ru_RU";

  return "en_US";
}

export function translatePluginValue(plugin: StorePluginManifest, value: string | undefined, lang: string) {
  const raw = String(value || "");
  if (!raw.startsWith("i18n:")) return raw;

  const key = raw.slice(5);
  const i18n = plugin.I18n || {};
  const langCode = getCurrentLangCode(lang);

  return i18n[langCode]?.[key] || i18n.en_US?.[key] || raw;
}

export function localizePlugin(plugin: StorePluginManifest, lang: string): LocalizedStorePluginManifest {
  return {
    ...plugin,
    LocalizedName: translatePluginValue(plugin, plugin.Name, lang),
    LocalizedDescription: translatePluginValue(plugin, plugin.Description, lang),
  };
}

export function normalizeOsLabel(os: string) {
  if (os === "Darwin") return "macOS";
  return os;
}

export function formatStoreDate(dateText: string | undefined, lang: string) {
  if (!dateText) return "";

  const normalized = dateText.replace(" ", "T");
  const parsed = new Date(normalized);
  if (Number.isNaN(parsed.getTime())) {
    return dateText.split(" ")[0] || dateText;
  }

  return new Intl.DateTimeFormat((lang || "en-US").toLowerCase(), {
    year: "numeric",
    month: "short",
    day: "numeric",
  }).format(parsed);
}
