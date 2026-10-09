using System.IO;
using System.Net.Http;
using System.Runtime.CompilerServices;
using System.Text.Json;
using System.Windows;
using System.Windows.Media;
using System.Xml.Linq;
using Flow.Launcher.Plugin;
using Flow.Launcher.Plugin.SharedModels;

namespace Wox.Flow.DotNetHost;

// BridgeApi is the IPublicAPI implementation plugins call.
// Calls that change the launcher are written as JSON events. Settings and cache
// stay in dot directories beside the plugin because this process does not share
// the other application's data folder.
sealed class BridgeApi : IPublicAPI
{
    static readonly HttpClient Http = new();
    static readonly JsonSerializerOptions JsonOptions = new()
    {
        PropertyNameCaseInsensitive = true,
        WriteIndented = true
    };

    readonly PluginMetadata metadata;
    readonly Action<object> emit;
    readonly Dictionary<Type, object> settings = new();
    readonly Dictionary<string, object> caches = new();
    readonly Dictionary<string, string> translations = new(StringComparer.Ordinal);
    readonly object visibilityLock = new();
    // mainWindowVisible stays true until Wox has hidden the launcher for this epoch.
    // Window Manager waits on IsMainWindowVisible, then minimizes GetForegroundWindow.
    int mainWindowVisible = 1;
    int visibilityEpoch;

    public IAsyncPlugin Plugin { get; set; }

    public event VisibilityChangedEventHandler VisibilityChanged;

    // Theme changes are not raised from this process.
#pragma warning disable CS0067
    public event ActualApplicationThemeChangedEventHandler ActualApplicationThemeChanged;
#pragma warning restore CS0067

    public BridgeApi(PluginMetadata metadata, Action<object> emit)
    {
        this.metadata = metadata;
        this.emit = emit;
    }

    public void ChangeQuery(string query, bool requery = false)
    {
        emit(new { @event = "changeQuery", query = query ?? "" });
    }

    public void RestartApp()
    {
    }

    public void ShellRun(string cmd, string filename = "cmd.exe")
    {
        emit(new { @event = "shell", command = cmd ?? "", program = filename ?? "cmd.exe" });
    }

    public void CopyToClipboard(string text, bool directCopy = false, bool showDefaultNotification = true)
    {
        emit(new { @event = "copy", text = text ?? "" });
        if (showDefaultNotification)
        {
            ShowMsg("Copied", text ?? "");
        }
    }

    public void SaveAppAllSettings() => SavePluginSettings();

    public void SavePluginSettings()
    {
        foreach (var entry in settings)
        {
            WriteJson(SettingsPath(entry.Key), entry.Value);
        }
    }

    public Task ReloadAllPluginData() => Task.CompletedTask;

    public void CheckForNewUpdate()
    {
    }

    public void ShowMsgError(string title, string subTitle = "") => ShowMsg(title, subTitle);

    public void ShowMsgErrorWithButton(string title, string buttonText, Action buttonAction, string subTitle = "")
    {
        ShowMsg(title, subTitle);
    }

    public void ShowMainWindow()
    {
        MarkLauncherVisible();
        emit(new { @event = "show" });
    }

    public void FocusQueryTextBox() => ShowMainWindow();

    public void HideMainWindow() => emit(new { @event = "hide", epoch = VisibilityEpoch });

    public bool IsMainWindowVisible() => Volatile.Read(ref mainWindowVisible) != 0;

    // MarkLauncherVisible starts a new generation because a query or action means the launcher is open.
    // A hide report that still carries the previous generation must not flip this one back to hidden.
    public int MarkLauncherVisible()
    {
        int epoch;
        bool changed;
        lock (visibilityLock)
        {
            visibilityEpoch++;
            epoch = visibilityEpoch;
            changed = ExchangeVisible(true);
        }
        RaiseVisibility(changed, true);
        return epoch;
    }

    public int VisibilityEpoch => Volatile.Read(ref visibilityEpoch);

    // MarkLauncherHidden accepts the generation the plugin attached to this hide.
    public void MarkLauncherHidden(int epoch)
    {
        bool changed;
        lock (visibilityLock)
        {
            if (epoch != visibilityEpoch)
            {
                return;
            }
            changed = ExchangeVisible(false);
        }
        RaiseVisibility(changed, false);
    }

    bool ExchangeVisible(bool visible)
    {
        var next = visible ? 1 : 0;
        return Interlocked.Exchange(ref mainWindowVisible, next) != next;
    }

    void RaiseVisibility(bool changed, bool visible)
    {
        if (!changed)
        {
            return;
        }
        try
        {
            VisibilityChanged?.Invoke(this, new VisibilityChangedEventArgs { IsVisible = visible });
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine("visibility: " + ex.Message);
        }
    }

    public void ShowMsg(string title, string subTitle = "", string iconPath = "")
    {
        emit(new { @event = "notify", title = title ?? "", subtitle = subTitle ?? "" });
    }

    public void ShowMsg(string title, string subTitle, string iconPath, bool useMainWindowAsOwner = true) => ShowMsg(title, subTitle, iconPath);

    public void ShowMsgWithButton(string title, string buttonText, Action buttonAction, string subTitle = "", string iconPath = "") => ShowMsg(title, subTitle, iconPath);

    public void ShowMsgWithButton(string title, string buttonText, Action buttonAction, string subTitle, string iconPath, bool useMainWindowAsOwner = true) => ShowMsg(title, subTitle, iconPath);

    public void OpenSettingDialog()
    {
    }

    // GetTranslation reads the plugin's Languages/*.xaml dictionary.
    // English is the base. The Wox language (zh_CN, pt_BR) replaces it when that file exists.
    // A missing key stays the key, which is what plugins already show as a fallback.
    public string GetTranslation(string key)
    {
        if (string.IsNullOrEmpty(key))
        {
            return "";
        }
        if (translations.TryGetValue(key, out var value) && value.Length > 0)
        {
            return value;
        }
        return key;
    }

    // LoadTranslations fills the dictionary before the plugin initializes.
    // uiLanguage uses Wox codes such as en_US and zh_CN.
    // ApplyTranslations copies loaded plugin strings into the WPF resource scope.
    // Settings panels bind their labels with DynamicResource.
    public void ApplyTranslations(ResourceDictionary dictionary)
    {
        if (dictionary == null)
        {
            return;
        }
        foreach (var pair in translations)
        {
            dictionary[pair.Key] = pair.Value;
        }
    }

    public void LoadTranslations(string pluginDirectory, string uiLanguage)
    {
        var dir = Path.Combine(pluginDirectory ?? "", "Languages");
        if (!Directory.Exists(dir))
        {
            return;
        }
        MergeLanguage(dir, "en");
        var code = (uiLanguage ?? "").Trim().Replace('_', '-').ToLowerInvariant();
        if (code.Length == 0 || code == "en" || code == "en-us")
        {
            return;
        }
        var dash = code.IndexOf('-');
        if (dash > 0)
        {
            MergeLanguage(dir, code.Substring(0, dash));
        }
        MergeLanguage(dir, code);
    }

    void MergeLanguage(string dir, string name)
    {
        string match = null;
        foreach (var path in Directory.EnumerateFiles(dir, "*.xaml"))
        {
            if (string.Equals(Path.GetFileNameWithoutExtension(path), name, StringComparison.OrdinalIgnoreCase))
            {
                match = path;
                break;
            }
        }
        if (match == null)
        {
            return;
        }
        XDocument document;
        try
        {
            document = XDocument.Load(match);
        }
        catch (Exception)
        {
            return;
        }
        foreach (var element in document.Descendants())
        {
            if (element.Name.LocalName != "String")
            {
                continue;
            }
            var key = "";
            foreach (var attribute in element.Attributes())
            {
                if (attribute.Name.LocalName == "Key")
                {
                    key = attribute.Value.Trim();
                    break;
                }
            }
            if (key.Length == 0)
            {
                continue;
            }
            translations[key] = element.Value.Trim();
        }
    }

    public List<PluginPair> GetAllPlugins() => Pairs();

    public List<PluginPair> GetAllInitializedPlugins(bool includeFailed) => Pairs();

    public void RegisterGlobalKeyboardCallback(Func<int, int, SpecialKeyState, bool> callback)
    {
    }

    public void RemoveGlobalKeyboardCallback(Func<int, int, SpecialKeyState, bool> callback)
    {
    }

    // FuzzySearch is a contains match. Plugins use it to drop non-matching rows
    // and to highlight the matched span. An empty query matches everything.
    public MatchResult FuzzySearch(string query, string stringToCompare)
    {
        query ??= "";
        stringToCompare ??= "";
        if (query.Length == 0)
        {
            return new MatchResult(true, SearchPrecisionScore.None, new List<int>(), 100);
        }
        var index = stringToCompare.IndexOf(query, StringComparison.OrdinalIgnoreCase);
        if (index < 0)
        {
            return new MatchResult(false, SearchPrecisionScore.None);
        }
        var positions = new List<int>(query.Length);
        for (var i = 0; i < query.Length; i++)
        {
            positions.Add(index + i);
        }
        var score = string.Equals(query, stringToCompare, StringComparison.OrdinalIgnoreCase) ? 100 : index == 0 ? 80 : 50;
        return new MatchResult(true, SearchPrecisionScore.None, positions, score);
    }

    public async Task<string> HttpGetStringAsync(string url, CancellationToken token = default)
    {
        return await Http.GetStringAsync(url, token);
    }

    public async Task<Stream> HttpGetStreamAsync(string url, CancellationToken token = default)
    {
        return await Http.GetStreamAsync(url, token);
    }

    public async Task HttpDownloadAsync(string url, string filePath, Action<double> reportProgress = null, CancellationToken token = default)
    {
        Directory.CreateDirectory(Path.GetDirectoryName(Path.GetFullPath(filePath)) ?? ".");
        await using var stream = await Http.GetStreamAsync(url, token);
        await using var file = File.Create(filePath);
        await stream.CopyToAsync(file, token);
        reportProgress?.Invoke(100);
    }

    public void AddActionKeyword(string pluginId, string newActionKeyword)
    {
    }

    public void RemoveActionKeyword(string pluginId, string oldActionKeyword)
    {
    }

    public bool ActionKeywordAssigned(string actionKeyword) => false;

    public void LogDebug(string className, string message, [CallerMemberName] string methodName = "") => Log(className, message);

    public void LogInfo(string className, string message, [CallerMemberName] string methodName = "") => Log(className, message);

    public void LogWarn(string className, string message, [CallerMemberName] string methodName = "") => Log(className, message);

    public void LogError(string className, string message, [CallerMemberName] string methodName = "") => Log(className, message);

    public void LogException(string className, string message, Exception e, [CallerMemberName] string methodName = "")
    {
        Log(className, message + " " + e);
    }

    public T LoadSettingJsonStorage<T>() where T : new()
    {
        if (settings.TryGetValue(typeof(T), out var cached))
        {
            return (T)cached;
        }
        var path = SettingsPath(typeof(T));
        T value = default;
        if (File.Exists(path))
        {
            try
            {
                value = JsonSerializer.Deserialize<T>(File.ReadAllText(path), JsonOptions);
            }
            catch (Exception ex)
            {
                Console.Error.WriteLine("settings " + path + ": " + ex.Message);
            }
        }
        value ??= new T();
        settings[typeof(T)] = value;
        return value;
    }

    public void SaveSettingJsonStorage<T>() where T : new()
    {
        if (!settings.TryGetValue(typeof(T), out var value))
        {
            value = new T();
            settings[typeof(T)] = value;
        }
        WriteJson(SettingsPath(typeof(T)), value);
    }

    public void OpenDirectory(string directoryPath, string fileNameOrFilePath = null)
    {
        emit(new { @event = "openDirectory", directory = directoryPath ?? "", file = fileNameOrFilePath ?? "" });
    }

    public void OpenWebUrl(Uri url, bool? inPrivate = null) => OpenUrl(url?.ToString());

    public void OpenWebUrl(string url, bool? inPrivate = null) => OpenUrl(url);

    public void OpenUrl(Uri url, bool? inPrivate = null) => OpenUrl(url?.ToString());

    public void OpenUrl(string url, bool? inPrivate = null)
    {
        emit(new { @event = "open", path = url ?? "" });
    }

    public void OpenAppUri(Uri appUri) => OpenUrl(appUri?.ToString());

    public void OpenAppUri(string appUri) => OpenUrl(appUri);

    public void ToggleGameMode()
    {
    }

    public void SetGameMode(bool value)
    {
    }

    public bool IsGameModeOn() => false;

    public void ReQuery(bool reselect = true) => emit(new { @event = "refresh" });

    public void BackToQueryResults()
    {
    }

    public System.Windows.MessageBoxResult ShowMsgBox(string messageBoxText, string caption = "", System.Windows.MessageBoxButton button = System.Windows.MessageBoxButton.OK, System.Windows.MessageBoxImage icon = System.Windows.MessageBoxImage.None, System.Windows.MessageBoxResult defaultResult = System.Windows.MessageBoxResult.OK)
    {
        ShowMsg(string.IsNullOrEmpty(caption) ? messageBoxText : caption, string.IsNullOrEmpty(caption) ? "" : messageBoxText ?? "");
        return defaultResult;
    }

    public async Task ShowProgressBoxAsync(string caption, Func<Action<double>, Task> reportProgressAsync, Action cancelProgress = null)
    {
        if (reportProgressAsync != null)
        {
            await reportProgressAsync(_ => { });
        }
    }

    public void StartLoadingBar()
    {
    }

    public void StopLoadingBar()
    {
    }

    public List<ThemeData> GetAvailableThemes() => new();

    public ThemeData GetCurrentTheme() => new("Wox", "Wox", true, false);

    public bool SetCurrentTheme(ThemeData theme) => false;

    public void SavePluginCaches()
    {
        foreach (var entry in caches)
        {
            WriteJson(CachePath(metadata.PluginCacheDirectoryPath, entry.Key), entry.Value);
        }
    }

    public Task<T> LoadCacheBinaryStorageAsync<T>(string cacheName, string cacheDirectory, T defaultData) where T : new()
    {
        if (caches.TryGetValue(cacheName ?? "", out var cached))
        {
            return Task.FromResult((T)cached);
        }
        var path = CachePath(cacheDirectory, cacheName);
        T value = default;
        if (File.Exists(path))
        {
            try
            {
                value = JsonSerializer.Deserialize<T>(File.ReadAllText(path), JsonOptions);
            }
            catch (Exception ex)
            {
                Console.Error.WriteLine("cache " + path + ": " + ex.Message);
            }
        }
        value ??= defaultData ?? new T();
        caches[cacheName ?? ""] = value;
        return Task.FromResult(value);
    }

    public Task SaveCacheBinaryStorageAsync<T>(string cacheName, string cacheDirectory) where T : new()
    {
        if (caches.TryGetValue(cacheName ?? "", out var value))
        {
            WriteJson(CachePath(cacheDirectory, cacheName), value);
        }
        return Task.CompletedTask;
    }

    public ValueTask<ImageSource> LoadImageAsync(string path, bool loadFullImage = false, bool cacheImage = true)
    {
        return ValueTask.FromResult<ImageSource>(null);
    }

    public Task<bool> UpdatePluginManifestAsync(bool usePrimaryUrlOnly = false, CancellationToken token = default) => Task.FromResult(false);

    public IReadOnlyList<UserPlugin> GetPluginManifest() => Array.Empty<UserPlugin>();

    public bool PluginModified(string id) => false;

    public Task<bool> UpdatePluginAsync(PluginMetadata pluginMetadata, UserPlugin plugin, string zipFilePath) => Task.FromResult(false);

    public bool InstallPlugin(UserPlugin plugin, string zipFilePath) => false;

    public Task<bool> UninstallPluginAsync(PluginMetadata pluginMetadata, bool removePluginSettings = false) => Task.FromResult(false);

    public long StopwatchLogDebug(string className, string message, Action action, [CallerMemberName] string methodName = "") => Time(action);

    public async Task<long> StopwatchLogDebugAsync(string className, string message, Func<Task> action, [CallerMemberName] string methodName = "")
    {
        var started = Environment.TickCount64;
        if (action != null)
        {
            await action();
        }
        return Environment.TickCount64 - started;
    }

    public long StopwatchLogInfo(string className, string message, Action action, [CallerMemberName] string methodName = "") => Time(action);

    public Task<long> StopwatchLogInfoAsync(string className, string message, Func<Task> action, [CallerMemberName] string methodName = "") => StopwatchLogDebugAsync(className, message, action, methodName);

    public bool IsApplicationDarkTheme() => true;

    public string GetDataDirectory() => metadata.PluginSettingsDirectoryPath ?? metadata.PluginDirectory ?? "";

    public string GetLogDirectory() => metadata.PluginCacheDirectoryPath ?? metadata.PluginDirectory ?? "";

    List<PluginPair> Pairs()
    {
        var pair = new PluginPair();
        Set(pair, "Plugin", Plugin);
        Set(pair, "Metadata", metadata);
        return new List<PluginPair> { pair };
    }

    static void Set(object target, string propertyName, object value)
    {
        var property = target.GetType().GetProperty(propertyName, System.Reflection.BindingFlags.Instance | System.Reflection.BindingFlags.Public | System.Reflection.BindingFlags.NonPublic);
        property?.SetValue(target, value);
    }

    string SettingsPath(Type type) => Path.Combine(metadata.PluginSettingsDirectoryPath ?? metadata.PluginDirectory ?? ".", type.Name + ".json");

    static string CachePath(string directory, string name)
    {
        directory = string.IsNullOrWhiteSpace(directory) ? "." : directory;
        Directory.CreateDirectory(directory);
        var file = string.IsNullOrWhiteSpace(name) ? "cache" : name;
        foreach (var invalid in Path.GetInvalidFileNameChars())
        {
            file = file.Replace(invalid, '_');
        }
        return Path.Combine(directory, file + ".json");
    }

    static void WriteJson(string path, object value)
    {
        Directory.CreateDirectory(Path.GetDirectoryName(Path.GetFullPath(path)) ?? ".");
        File.WriteAllText(path, JsonSerializer.Serialize(value, JsonOptions));
    }

    static void Log(string className, string message)
    {
        Console.Error.WriteLine((className ?? "") + " " + (message ?? ""));
    }

    static long Time(Action action)
    {
        var started = Environment.TickCount64;
        action?.Invoke();
        return Environment.TickCount64 - started;
    }
}
