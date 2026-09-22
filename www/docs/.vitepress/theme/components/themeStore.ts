import storeThemeData from "../../../../../store-theme.json";

export type StoreThemeIconColors = {
  Background?: string;
  Query?: string;
  Selected?: string;
  Outline?: string;
  OutlineWidth?: number;
};

export type StoreThemeManifest = {
  Id: string;
  Name: string;
  Author: string;
  Version: string;
  MinWoxVersion?: string;
  Description: string;
  ImageTheme?: boolean;
  IconColors?: StoreThemeIconColors;
  Website?: string;
  DownloadUrl?: string;
  ScreenshotUrls?: string[];
  DateCreated?: string;
  DateUpdated?: string;
  I18n?: Record<string, Record<string, string>>;
};

export function hasThemeSwatch(colors: StoreThemeIconColors | undefined) {
  return Boolean(colors?.Background && colors.Query && colors.Selected);
}

export type LocalizedStoreThemeManifest = StoreThemeManifest & {
  LocalizedName: string;
  LocalizedDescription: string;
};

export async function fetchStoreThemes() {
  return storeThemeData as StoreThemeManifest[];
}

export function getCurrentLangCode(lang: string) {
  const normalizedLang = (lang || "").toLowerCase();

  if (normalizedLang.startsWith("zh")) return "zh_CN";
  if (normalizedLang.startsWith("pt")) return "pt_BR";
  if (normalizedLang.startsWith("ru")) return "ru_RU";
  if (normalizedLang.startsWith("ko")) return "ko_KR";
  if (normalizedLang.startsWith("ja")) return "ja_JP";

  return "en_US";
}

export function translateThemeValue(theme: StoreThemeManifest, value: string | undefined, lang: string) {
  const raw = String(value || "");
  if (!raw.startsWith("i18n:")) return raw;

  const key = raw.slice(5);
  const i18n = theme.I18n || {};
  const langCode = getCurrentLangCode(lang);

  return i18n[langCode]?.[key] || i18n.en_US?.[key] || raw;
}

export function localizeTheme(theme: StoreThemeManifest, lang: string): LocalizedStoreThemeManifest {
  return {
    ...theme,
    LocalizedName: translateThemeValue(theme, theme.Name, lang),
    LocalizedDescription: translateThemeValue(theme, theme.Description, lang),
  };
}

export function englishThemeName(theme: StoreThemeManifest): string {
  return translateThemeValue(theme, theme.Name, "en-US");
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
