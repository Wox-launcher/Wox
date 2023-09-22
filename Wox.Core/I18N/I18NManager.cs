using System.Text.Json;
using Wox.Core.Plugin;
using Wox.Core.Utils;

namespace Wox.Core.I18n;

// <PluginId, <LanguageCode, <TranslationKey, TranslationValue>>>
using PluginTranslationMap = Dictionary<string, Dictionary<string, Dictionary<string, string>>>;

public static class I18NManager
{
    private static readonly string FakeWoxPluginId = "WOX";
    private static readonly PluginTranslationMap I18NDict = new();

    /// <summary>
    ///     Load all translations
    ///     Because we need to load plugin translations, so this method must load after plugin loaded
    /// </summary>
    public static async Task Load()
    {
        // load wox translations
        await LoadLanguages(FakeWoxPluginId, DataLocation.I18nDirectory);

        // load plugin translations
        foreach (var pluginInstance in PluginManager.GetAllPlugins()) await LoadLanguages(pluginInstance.Metadata.Id, pluginInstance.PluginDirectory);
    }

    private static async Task LoadLanguages(string pluginId, string pluginDirectory)
    {
        var pluginTranslations = new Dictionary<string, Dictionary<string, string>>();
        I18NDict.Add(pluginId, pluginTranslations);
        foreach (var i18NFile in Directory.GetFiles(pluginDirectory))
            try
            {
                var json = await File.ReadAllTextAsync(i18NFile);
                var translations = JsonSerializer.Deserialize<Dictionary<string, string>>(json);
                if (translations == null)
                {
                    Logger.Error($"Fail to deserialize i18n file {i18NFile}");
                    continue;
                }

                pluginTranslations.Add(Path.GetFileNameWithoutExtension(i18NFile), translations);
            }
            catch (Exception e)
            {
                Logger.Error($"Fail to load i18n file {i18NFile}", e);
            }
    }

    public static string GetWoxTranslation(string key)
    {
        return GetWoxTranslation(key, Languages.English.Code);
    }

    public static string GetPluginTranslation(string pluginId, string key)
    {
        return GetPluginTranslation(pluginId, key, Languages.English.Code);
    }

    private static string GetWoxTranslation(string key, string languageCode)
    {
        return GetPluginTranslation(FakeWoxPluginId, key, languageCode);
    }

    private static string GetPluginTranslation(string pluginId, string key, string languageCode)
    {
        I18NDict.TryGetValue(pluginId, out var pluginTranslations);
        if (pluginTranslations != null)
        {
            pluginTranslations.TryGetValue(languageCode, out var languageResults);
            if (languageResults != null)
            {
                languageResults.TryGetValue(key, out var languageResult);
                if (languageResult != null) return languageResult;
            }

            // try to get english translations
            pluginTranslations.TryGetValue(Languages.English.Code, out var englishTranslations);
            if (englishTranslations != null)
            {
                englishTranslations.TryGetValue(key, out var englishTranslationResult);
                if (englishTranslationResult != null) return englishTranslationResult;
            }
        }

        return $"[NO TRANSLATION FOR: {key}]";
    }
}