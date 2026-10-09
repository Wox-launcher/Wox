using System.IO;
using System.Reflection;
using System.Runtime.Loader;
using System.Windows.Documents;
using Microsoft.Win32;
using System.Text;
using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;
using Flow.Launcher.Plugin;

namespace Wox.Flow.DotNetHost;

// Program loads one C# or F# plugin and speaks one JSON object per line.
// The plugin assembly is compiled against Flow.Launcher.Plugin, so this process
// supplies that assembly and a small API bridge back to Wox.
public static class Program
{
    [STAThread]
    public static int Main()
    {
        Console.InputEncoding = Encoding.UTF8;
        Console.OutputEncoding = new UTF8Encoding(false);
        try
        {
            if (System.Windows.Application.Current == null)
            {
                _ = new System.Windows.Application
                {
                    ShutdownMode = System.Windows.ShutdownMode.OnExplicitShutdown
                };
            }
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine("wpf: " + ex.Message);
        }

        var stdout = new StreamWriter(Console.OpenStandardOutput(), new UTF8Encoding(false)) { AutoFlush = true };
        var host = new PluginProcess(stdout);
        using var stdin = new StreamReader(Console.OpenStandardInput(), Encoding.UTF8);
        string line;
        while ((line = stdin.ReadLine()) != null)
        {
            if (string.IsNullOrWhiteSpace(line))
            {
                continue;
            }
            host.Handle(line);
        }
        return 0;
    }
}

sealed class PluginProcess
{
    readonly StreamWriter stdout;
    readonly object writeLock = new();
    // Query and ResultsUpdated both register row actions. The event can arrive on
    // another thread while a query is still building its rows.
    readonly object actionLock = new();
    readonly Dictionary<string, Result> actions = new();
    BridgeApi api;
    IAsyncPlugin plugin;
    ISettingProvider settingProvider;
    IContextMenu contextMenus;
    string pluginName;
    int actionSerial;

    public PluginProcess(StreamWriter stdout)
    {
        this.stdout = stdout;
    }

    public void Handle(string line)
    {
        JsonDocument document;
        try
        {
            document = JsonDocument.Parse(line);
        }
        catch (Exception ex)
        {
            Write(new { ok = false, error = "invalid request: " + ex.Message });
            return;
        }
        using (document)
        {
            var root = document.RootElement;
            var id = Text(root, "id");
            var method = Text(root, "method");
            try
            {
                switch (method)
                {
                    case "init":
                        Init(id, root);
                        break;
                    case "query":
                        Query(id, root);
                        break;
                    case "action":
                        Action(id, Text(root, "action"));
                        break;
                    case "visibility":
                        ApplyVisibility(root);
                        break;
                    case "showSettings":
                        ShowSettings(id);
                        break;
                    default:
                        Write(new { id, ok = false, error = "unknown method " + method });
                        break;
                }
            }
            catch (Exception ex)
            {
                Console.Error.WriteLine(ex);
                Write(new { id, ok = false, error = ex.Message });
            }
        }
    }

    void Init(string id, JsonElement root)
    {
        var directory = Path.GetFullPath(Required(root, "directory"));
        var entry = Required(root, "entry");
        var dllPath = Path.GetFullPath(Path.IsPathRooted(entry) ? entry : Path.Combine(directory, entry));
        if (!File.Exists(dllPath))
        {
            throw new FileNotFoundException("plugin assembly was not found", dllPath);
        }

        Directory.SetCurrentDirectory(directory);
        // ModernWpf asks the default context for the 18362 projection. The plugin
        // ships a newer copy beside its own assembly; share that one copy.
        EnablePluginProjectionResolve(directory);
        var metadata = new PluginMetadata
        {
            ID = Text(root, "pluginId"),
            Name = Text(root, "name"),
            Author = Text(root, "author"),
            Version = Text(root, "version"),
            Description = Text(root, "description"),
            Website = Text(root, "website"),
            Language = Text(root, "language"),
            ExecuteFileName = entry,
            IcoPath = Text(root, "icoPath"),
            ActionKeyword = Keyword(Text(root, "keyword")),
            ActionKeywords = new List<string> { Keyword(Text(root, "keyword")) }
        };
        var settingsDir = Path.Combine(directory, ".flow-settings");
        var cacheDir = Path.Combine(directory, ".flow-cache");
        Directory.CreateDirectory(settingsDir);
        Directory.CreateDirectory(cacheDir);
        SetInternal(metadata, "PluginDirectory", directory);
        SetInternal(metadata, "PluginSettingsDirectoryPath", settingsDir);
        SetInternal(metadata, "PluginCacheDirectoryPath", cacheDir);

        var loadContext = new PluginLoadContext(dllPath);
        var assembly = loadContext.LoadFromAssemblyPath(dllPath);
        var type = FindPluginType(assembly);
        var instance = Activator.CreateInstance(type) ?? throw new InvalidOperationException("plugin type could not be constructed");
        if (instance is IAsyncPlugin asyncPlugin)
        {
            plugin = asyncPlugin;
        }
        else if (instance is IPlugin syncPlugin)
        {
            plugin = new SyncPluginAdapter(syncPlugin);
        }
        else
        {
            throw new InvalidOperationException("plugin type does not implement IPlugin or IAsyncPlugin");
        }
        contextMenus = instance as IContextMenu;
        api = new BridgeApi(metadata, Emit);
        api.Plugin = plugin;
        api.LoadTranslations(directory, Text(root, "uiLanguage"));
        var context = new PluginInitContext(metadata, api);
        plugin.InitAsync(context).GetAwaiter().GetResult();
        if (instance is IResultUpdated updated)
        {
            // ReQuery keeps the refresh event. ResultsUpdated carries the list to show.
            updated.ResultsUpdated += OnResultsUpdated;
        }
        settingProvider = instance as ISettingProvider;
        pluginName = metadata.Name;
        Write(new { id, ok = true, hasSettingPanel = settingProvider != null });
    }

    static string projectionDirectory;
    static int projectionResolveRegistered;

    // EnablePluginProjectionResolve lets the default context load WinRT projections
    // from the plugin directory. ModernWpf references Microsoft.Windows.SDK.NET 10.0.18362.10,
    // and plugins ship a newer build of that same assembly.
    static void EnablePluginProjectionResolve(string directory)
    {
        projectionDirectory = directory;
        if (Interlocked.Exchange(ref projectionResolveRegistered, 1) != 0)
        {
            return;
        }
        AssemblyLoadContext.Default.Resolving += ResolveProjectionAssembly;
    }

    static Assembly ResolveProjectionAssembly(AssemblyLoadContext context, AssemblyName name)
    {
        if (name.Name is not ("Microsoft.Windows.SDK.NET" or "WinRT.Runtime"))
        {
            return null;
        }
        foreach (var loaded in context.Assemblies)
        {
            if (string.Equals(loaded.GetName().Name, name.Name, StringComparison.OrdinalIgnoreCase))
            {
                return loaded;
            }
        }
        if (string.IsNullOrEmpty(projectionDirectory))
        {
            return null;
        }
        var path = Path.Combine(projectionDirectory, name.Name + ".dll");
        if (!File.Exists(path))
        {
            return null;
        }
        return context.LoadFromAssemblyPath(path);
    }

    static bool flowSettingsResourcesReady;
    static bool settingsUseDarkTheme;
    static SolidColorBrush settingsWindowBrush = Brushes.White;
    static SolidColorBrush settingsTextBrush = Brushes.Black;

    // SystemAppsUseDarkTheme reads the Windows app theme. ModernWpf follows it only
    // after ApplicationTheme is set; otherwise the window, labels, and popups each pick a different palette.
    static bool SystemAppsUseDarkTheme()
    {
        try
        {
            using var key = Registry.CurrentUser.OpenSubKey(@"Software\Microsoft\Windows\CurrentVersion\Themes\Personalize");
            return key?.GetValue("AppsUseLightTheme") is int light && light == 0;
        }
        catch (Exception)
        {
            return false;
        }
    }

    // EnsureFlowSettingsResources supplies the Flow settings-page resources that plugin
    // panels reference. Those panels are built to live inside Flow, so StaticResource
    // keys and ModernWpf controls are missing until this host provides them.
    static void EnsureFlowSettingsResources()
    {
        if (flowSettingsResourcesReady)
        {
            return;
        }
        var app = Application.Current ?? new Application { ShutdownMode = ShutdownMode.OnExplicitShutdown };
        settingsUseDarkTheme = SystemAppsUseDarkTheme();
        var theme = settingsUseDarkTheme ? ModernWpf.ApplicationTheme.Dark : ModernWpf.ApplicationTheme.Light;
        settingsWindowBrush = new SolidColorBrush(settingsUseDarkTheme ? Color.FromRgb(0x20, 0x20, 0x20) : Colors.White);
        settingsTextBrush = new SolidColorBrush(settingsUseDarkTheme ? Color.FromRgb(0xF3, 0xF3, 0xF3) : Color.FromRgb(0x1A, 0x1A, 0x1A));
        settingsWindowBrush.Freeze();
        settingsTextBrush.Freeze();
        try
        {
            ModernWpf.ThemeManager.Current.ApplicationTheme = theme;
            app.Resources.MergedDictionaries.Add(new ModernWpf.ThemeResources { RequestedTheme = theme });
            app.Resources.MergedDictionaries.Add(new ModernWpf.Controls.XamlControlsResources());
        }
        catch (Exception)
        {
            // A plain WPF panel does not need the ModernWpf theme. Panels that do
            // will fail again in CreateSettingPanel, with the load error intact.
        }
        var border = new SolidColorBrush(settingsUseDarkTheme ? Color.FromRgb(0x5A, 0x5A, 0x5A) : Color.FromRgb(0xD6, 0xD6, 0xD6));
        var secondary = new SolidColorBrush(settingsUseDarkTheme ? Color.FromRgb(0xC8, 0xC8, 0xC8) : Color.FromRgb(0x5C, 0x5C, 0x5C));
        border.Freeze();
        secondary.Freeze();
        var resources = new ResourceDictionary
        {
            ["SettingPanelMargin"] = new Thickness(70, 13.5, 18, 13.5),
            ["SettingPanelItemLeftMargin"] = new Thickness(9, 0, 0, 0),
            ["SettingPanelItemRightMargin"] = new Thickness(0, 0, 9, 0),
            ["SettingPanelItemTopBottomMargin"] = new Thickness(0, 4.5, 0, 4.5),
            ["SettingPanelItemLeftTopBottomMargin"] = new Thickness(9, 4.5, 0, 4.5),
            ["SettingPanelItemRightTopBottomMargin"] = new Thickness(0, 4.5, 9, 4.5),
            ["SettingPanelTextBoxMinWidth"] = 180d,
            ["SettingPanelPathTextBoxWidth"] = 240d,
            ["SettingPanelAreaTextBoxMinHeight"] = 150d,
            ["Color03B"] = border,
            ["Color04B"] = secondary
        };
        var separator = new Style(typeof(Separator));
        separator.Setters.Add(new Setter(FrameworkElement.MarginProperty, new Thickness(-70, 13.5, -18, 13.5)));
        separator.Setters.Add(new Setter(FrameworkElement.HeightProperty, 1d));
        separator.Setters.Add(new Setter(FrameworkElement.VerticalAlignmentProperty, VerticalAlignment.Top));
        separator.Setters.Add(new Setter(Control.BackgroundProperty, border));
        resources["SettingPanelSeparatorStyle"] = separator;
        var description = new Style(typeof(TextBlock));
        description.Setters.Add(new Setter(TextBlock.FontSizeProperty, 12d));
        description.Setters.Add(new Setter(FrameworkElement.MarginProperty, new Thickness(0, 2, 0, 0)));
        description.Setters.Add(new Setter(TextBlock.ForegroundProperty, secondary));
        resources["SettingPanelTextBlockDescriptionStyle"] = description;
        // Plugin labels are TextBlocks. They do not inherit Control.Foreground, so a dark
        // window otherwise leaves them black and invisible.
        var labels = new Style(typeof(TextBlock));
        if (app.TryFindResource(typeof(TextBlock)) is Style themeLabels)
        {
            labels.BasedOn = themeLabels;
        }
        labels.Setters.Add(new Setter(TextBlock.ForegroundProperty, settingsTextBrush));
        resources.Add(typeof(TextBlock), labels);
        app.Resources.MergedDictionaries.Add(resources);
        flowSettingsResourcesReady = true;
    }

    // ApplySettingsWindowTheme keeps the dialog, its labels, and ModernWpf popups on one palette.
    static void ApplySettingsWindowTheme(Window window)
    {
        window.Background = settingsWindowBrush;
        window.Foreground = settingsTextBrush;
        window.SetValue(TextElement.ForegroundProperty, settingsTextBrush);
        ModernWpf.ThemeManager.SetRequestedTheme(window, settingsUseDarkTheme ? ModernWpf.ElementTheme.Dark : ModernWpf.ElementTheme.Light);
    }

    static void BringSettingsToFront(Window window)
    {
        window.Activate();
        window.Topmost = true;
        window.Topmost = false;
        window.Focus();
    }

    // ShowSettings hosts the plugin's WPF panel on this STA thread.
    // The request stays open until the user closes the window.
    void ShowSettings(string id)
    {
        if (settingProvider == null)
        {
            Write(new { id, ok = false, error = "plugin has no settings panel" });
            return;
        }
        try
        {
            EnsureFlowSettingsResources();
            api?.ApplyTranslations(Application.Current.Resources);
        }
        catch (Exception ex)
        {
            Write(new { id, ok = false, error = ex.Message });
            return;
        }
        Control panel;
        try
        {
            panel = settingProvider.CreateSettingPanel();
        }
        catch (Exception ex)
        {
            Write(new { id, ok = false, error = ex.Message });
            return;
        }
        if (panel == null)
        {
            Write(new { id, ok = false, error = "plugin did not create a settings panel" });
            return;
        }
        var title = string.IsNullOrWhiteSpace(pluginName) ? "Settings" : pluginName;
        var window = new Window
        {
            Title = title,
            Content = new ScrollViewer
            {
                Content = panel,
                VerticalScrollBarVisibility = ScrollBarVisibility.Auto,
                HorizontalScrollBarVisibility = ScrollBarVisibility.Disabled
            },
            Width = 560,
            SizeToContent = SizeToContent.Height,
            MinHeight = 240,
            MaxHeight = 720,
            WindowStartupLocation = WindowStartupLocation.CenterScreen,
            ShowActivated = true
        };
        ApplySettingsWindowTheme(window);
        // Wox grants foreground permission before this request. The window still
        // has to activate itself; a background host is not brought forward by ShowDialog alone.
        window.Loaded += (_, _) => BringSettingsToFront(window);
        window.ShowDialog();
        Write(new { id, ok = true });
    }

    void Query(string id, JsonElement root)
    {
        if (plugin == null)
        {
            throw new InvalidOperationException("plugin is not initialized");
        }
        lock (actionLock)
        {
            actions.Clear();
            actionSerial = 0;
        }
        var search = Text(root, "search");
        var raw = Text(root, "rawQuery");
        if (raw.Length == 0)
        {
            raw = search;
        }
        var keyword = Keyword(Text(root, "keyword"));
        var terms = string.IsNullOrEmpty(search)
            ? Array.Empty<string>()
            : search.Split(' ', StringSplitOptions.RemoveEmptyEntries);
        api.MarkLauncherVisible();
        var query = new Query
        {
            SearchTerms = terms,
            ActionKeyword = keyword
        };
        SetInternal(query, "Search", search);
        SetInternal(query, "TrimmedQuery", raw);
        SetInternal(query, "OriginalQuery", raw);
        using var cancel = new CancellationTokenSource();
        var results = plugin.QueryAsync(query, cancel.Token).GetAwaiter().GetResult() ?? new List<Result>();
        List<object> rows;
        lock (actionLock)
        {
            rows = new List<object>(results.Count);
            foreach (var result in results)
            {
                if (result == null)
                {
                    continue;
                }
                rows.Add(ToRow(result));
            }
        }
        Write(new { id, ok = true, results = rows });
    }

    // OnResultsUpdated publishes the event list for the current query.
    // The query response that follows a preview is the complete list, so this
    // event must not ask Wox to run the query again.
    void OnResultsUpdated(object sender, ResultUpdatedEventArgs args)
    {
        try
        {
            var search = args?.Query?.Search ?? "";
            var rows = new List<object>();
            lock (actionLock)
            {
                if (args?.Results != null)
                {
                    foreach (var result in args.Results)
                    {
                        if (result == null)
                        {
                            continue;
                        }
                        rows.Add(ToRow(result));
                    }
                }
            }
            Emit(new { @event = "results", query = search, results = rows });
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine(ex.Message);
        }
    }

    object ToRow(Result result)
    {
        var rowActions = new List<object>();
        if (result.Action != null || result.AsyncAction != null)
        {
            rowActions.Add(Register(result, "Run"));
        }
        if (contextMenus != null)
        {
            try
            {
                var extra = contextMenus.LoadContextMenus(result);
                if (extra != null)
                {
                    foreach (var item in extra)
                    {
                        if (item == null || (item.Action == null && item.AsyncAction == null))
                        {
                            continue;
                        }
                        var name = string.IsNullOrWhiteSpace(item.Title) ? "Run" : item.Title;
                        rowActions.Add(Register(item, name));
                    }
                }
            }
            catch (Exception ex)
            {
                Console.Error.WriteLine("context menu: " + ex.Message);
            }
        }
        string previewFile = "";
        string previewText = "";
        if (result.Preview != null)
        {
            previewFile = result.Preview.FilePath ?? "";
            previewText = result.Preview.Description ?? "";
        }
        // CopyText falls back to SubTitle when the plugin did not set it.
        // That fallback would put a Copy action on every row.
        var copyText = result.CopyText ?? "";
        if (copyText == (result.SubTitle ?? ""))
        {
            copyText = "";
        }
        return new
        {
            title = result.Title ?? "",
            subTitle = result.SubTitle ?? "",
            iconPath = result.IcoPath ?? "",
            score = result.Score,
            copyText,
            rounded = result.RoundedIcon,
            previewFile,
            previewText,
            actions = rowActions
        };
    }

    object Register(Result result, string name)
    {
        actionSerial++;
        var actionId = actionSerial.ToString();
        actions[actionId] = result;
        return new { id = actionId, name };
    }

    void Action(string id, string actionId)
    {
        Result result;
        lock (actionLock)
        {
            if (!actions.TryGetValue(actionId ?? "", out result))
            {
                throw new InvalidOperationException("action is no longer available");
            }
        }
        var epoch = api.MarkLauncherVisible();
        var context = new ActionContext { SpecialKeyState = SpecialKeyState.Default };
        var hide = true;
        if (result.AsyncAction != null)
        {
            hide = result.AsyncAction(context).GetAwaiter().GetResult();
        }
        else if (result.Action != null)
        {
            hide = result.Action(context);
        }
        if (hide)
        {
            Emit(new { @event = "hide", epoch });
        }
        Write(new { id, ok = true });
    }

    // ApplyVisibility records that hide has finished.
    // The notice is read on this thread, so a plugin waiting inside Action cannot observe it.
    // Window Manager waits on a background task after Action has returned.
    void ApplyVisibility(JsonElement root)
    {
        if (api == null)
        {
            return;
        }
        if (Bool(root, "visible"))
        {
            api.MarkLauncherVisible();
            return;
        }
        api.MarkLauncherHidden(Int(root, "epoch"));
    }

    void Emit(object message)
    {
        Write(message);
    }

    void Write(object message)
    {
        var json = JsonSerializer.Serialize(message);
        lock (writeLock)
        {
            stdout.WriteLine(json);
        }
    }

    // FindPluginType accepts one IAsyncPlugin or IPlugin. Most plugins implement
    // the synchronous interface, so requiring IAsyncPlugin would reject them.
    static Type FindPluginType(Assembly assembly)
    {
        Type found = null;
        foreach (var type in SafeTypes(assembly))
        {
            if (type.IsAbstract || type.IsInterface)
            {
                continue;
            }
            if (!typeof(IAsyncPlugin).IsAssignableFrom(type) && !typeof(IPlugin).IsAssignableFrom(type))
            {
                continue;
            }
            if (found != null)
            {
                throw new InvalidOperationException("plugin assembly contains more than one plugin type");
            }
            found = type;
        }
        if (found == null)
        {
            throw new InvalidOperationException("plugin assembly has no IPlugin or IAsyncPlugin type");
        }
        return found;
    }

    static IEnumerable<Type> SafeTypes(Assembly assembly)
    {
        try
        {
            return assembly.GetTypes();
        }
        catch (ReflectionTypeLoadException ex)
        {
            return ex.Types.Where(type => type != null);
        }
    }

    static void SetInternal(object target, string propertyName, object value)
    {
        var property = target.GetType().GetProperty(propertyName, BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic);
        if (property == null)
        {
            throw new MissingMemberException(target.GetType().FullName, propertyName);
        }
        property.SetValue(target, value);
    }

    static string Keyword(string keyword)
    {
        keyword = (keyword ?? "").Trim();
        return keyword == "*" ? "" : keyword;
    }

    static string Required(JsonElement root, string name)
    {
        var value = Text(root, name);
        if (value.Length == 0)
        {
            throw new InvalidOperationException(name + " is required");
        }
        return value;
    }

    static bool Bool(JsonElement root, string name)
    {
        if (!root.TryGetProperty(name, out var value))
        {
            return false;
        }
        return value.ValueKind switch
        {
            JsonValueKind.True => true,
            JsonValueKind.False => false,
            JsonValueKind.String => bool.TryParse(value.GetString(), out var parsed) && parsed,
            _ => false
        };
    }

    static int Int(JsonElement root, string name)
    {
        if (!root.TryGetProperty(name, out var value))
        {
            return 0;
        }
        if (value.TryGetInt32(out var number))
        {
            return number;
        }
        return int.TryParse(value.ToString(), out var parsed) ? parsed : 0;
    }

    static string Text(JsonElement root, string name)
    {
        if (!root.TryGetProperty(name, out var value) || value.ValueKind == JsonValueKind.Null)
        {
            return "";
        }
        return value.ValueKind == JsonValueKind.String ? value.GetString() ?? "" : value.ToString();
    }
}

// SyncPluginAdapter lets an IPlugin run through the same async host calls.
sealed class SyncPluginAdapter : IAsyncPlugin
{
    readonly IPlugin plugin;

    public SyncPluginAdapter(IPlugin plugin)
    {
        this.plugin = plugin;
    }

    public Task InitAsync(PluginInitContext context)
    {
        plugin.Init(context);
        return Task.CompletedTask;
    }

    public Task<List<Result>> QueryAsync(Query query, CancellationToken token)
    {
        token.ThrowIfCancellationRequested();
        return Task.FromResult(plugin.Query(query) ?? new List<Result>());
    }
}

// PluginLoadContext resolves dependencies from the plugin directory.
// Flow.Launcher.Plugin stays on the default context so the host and the plugin
// share one copy of the API assembly.
sealed class PluginLoadContext : AssemblyLoadContext
{
    readonly AssemblyDependencyResolver resolver;
    readonly string directory;

    public PluginLoadContext(string pluginPath) : base(isCollectible: false)
    {
        resolver = new AssemblyDependencyResolver(pluginPath);
        directory = Path.GetDirectoryName(pluginPath) ?? "";
    }

    protected override Assembly Load(AssemblyName assemblyName)
    {
        // These stay on the default context. Flow.Launcher.Plugin is the host's API
        // assembly. The WinRT projections are shared with ModernWpf, which loads them
        // from the plugin directory through ResolveProjectionAssembly.
        if (string.Equals(assemblyName.Name, "Flow.Launcher.Plugin", StringComparison.OrdinalIgnoreCase)
            || string.Equals(assemblyName.Name, "Microsoft.Windows.SDK.NET", StringComparison.OrdinalIgnoreCase)
            || string.Equals(assemblyName.Name, "WinRT.Runtime", StringComparison.OrdinalIgnoreCase))
        {
            return null;
        }
        var resolved = resolver.ResolveAssemblyToPath(assemblyName);
        if (resolved != null)
        {
            return LoadFromAssemblyPath(resolved);
        }
        var candidate = Path.Combine(directory, assemblyName.Name + ".dll");
        if (File.Exists(candidate))
        {
            return LoadFromAssemblyPath(candidate);
        }
        return null;
    }

    protected override IntPtr LoadUnmanagedDll(string unmanagedDllName)
    {
        var resolved = resolver.ResolveUnmanagedDllToPath(unmanagedDllName);
        return resolved == null ? IntPtr.Zero : LoadUnmanagedDllFromPath(resolved);
    }
}
