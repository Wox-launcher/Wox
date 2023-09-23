using System.Diagnostics;
using System.Reflection;
using System.Text.Json;
using Wox.Core.Plugin.Host;
using Wox.Core.Plugin.System;
using Wox.Core.Utils;

namespace Wox.Core.Plugin;

public static class PluginLoader
{
    private static List<PluginHostBase> PluginHosts { get; } = new()
    {
        new DotnetHost(),
        new NodejsHost(),
        new PythonHost()
    };

    public static event Action<PluginInstance>? PluginLoaded;

    public static async Task Load()
    {
        Logger.Debug("Start to load plugins");

        // load system plugin first
        LoadSystemPlugins();

        // load other plugins
        foreach (var pluginRuntime in PluginRuntime.All)
            try
            {
                await LoadPluginsByRuntime(pluginRuntime);
            }
            catch (Exception e)
            {
                Logger.Error($"[{pluginRuntime} host] load host plugin failed", e);
            }
    }

    private static void LoadSystemPlugins()
    {
        try
        {
            var loadStartTimestamp = Stopwatch.GetTimestamp();
            var systemTypes = Assembly.GetExecutingAssembly().GetTypes().Where(o => typeof(ISystemPlugin).IsAssignableFrom(o) && o.IsClass);
            foreach (var type in systemTypes)
            {
                var rawIPlugin = Activator.CreateInstance(type) as ISystemPlugin;
                if (rawIPlugin == null) return;
                var pluginInstance = new PluginInstance
                {
                    Metadata = rawIPlugin.GetMetadata(),
                    Plugin = rawIPlugin,
                    API = new PluginPublicAPI(rawIPlugin.GetMetadata()),
                    PluginDirectory = "",
                    Host = new DotnetHost(),
                    IsSystemPlugin = true,
                    LoadStartTimestamp = loadStartTimestamp,
                    LoadFinishedTimestamp = Stopwatch.GetTimestamp()
                };
                PluginLoaded?.Invoke(pluginInstance);
                Logger.Debug($"Start to load system plugin: {pluginInstance.Metadata.Name}");
            }
        }
        catch (Exception e)
        {
            Logger.Error("Couldn't load system plugins", e);
#if DEBUG
            throw;
#else
            return;
#endif
        }
    }

    private static async Task LoadPluginsByRuntime(string pluginRuntime)
    {
        List<(PluginMetadata, string)> pluginMetas = new();
        var pluginDirectories = DataLocation.PluginDirectories.SelectMany(Directory.GetDirectories);
        foreach (var pluginDirectory in pluginDirectories)
        {
            var configPath = Path.Combine(pluginDirectory, "plugin.json");
            if (!File.Exists(configPath))
            {
                Logger.Error($"Didn't find plugin config file {configPath}");
                continue;
            }

            var metadata = ParsePluginMetadataFromDirectory(pluginDirectory);
            if (metadata == null) continue;

            if (metadata.Runtime.ToUpper() != pluginRuntime.ToUpper()) continue;

            pluginMetas.Add((metadata, pluginDirectory));
        }

        Logger.Debug($"[{pluginRuntime} host] start to load plugins");
        var pluginHost = PluginHosts.FirstOrDefault(o => o.PluginRuntime.ToUpper() == pluginRuntime.ToUpper());
        if (pluginHost == null) throw new Exception($"[{pluginRuntime}] there is no host for {pluginRuntime}");

        Logger.Debug($"[{pluginHost.PluginRuntime} host] starting plugin host");
        await pluginHost.Start();
        foreach (var (metadata, pluginDirectory) in pluginMetas)
        {
            Logger.Debug($"[{metadata.Runtime} host] start to load plugin: {metadata.Name}");
            var loadStartTimestamp = Stopwatch.GetTimestamp();
            var plugin = await pluginHost.LoadPlugin(metadata, pluginDirectory);
            var loadFinishedTimestamp = Stopwatch.GetTimestamp();
            if (plugin == null) continue;

            var pluginInstance = new PluginInstance
            {
                Metadata = metadata,
                Plugin = plugin,
                API = new PluginPublicAPI(metadata),
                PluginDirectory = pluginDirectory,
                Host = pluginHost,
                LoadStartTimestamp = loadStartTimestamp,
                LoadFinishedTimestamp = loadFinishedTimestamp
            };
            PluginLoaded?.Invoke(pluginInstance);
        }
    }

    /// <summary>
    ///     Parse plugin metadata in giving directory
    /// </summary>
    private static PluginMetadata? ParsePluginMetadataFromDirectory(string pluginDirectory)
    {
        Logger.Debug($"Start to parse plugin metadata in {pluginDirectory}");
        var configPath = Path.Combine(pluginDirectory, "plugin.json");
        if (!File.Exists(configPath))
        {
            Logger.Error($"Didn't find plugin config file {configPath}");
            return null;
        }

        try
        {
            var pluginJson = File.ReadAllText(configPath);
            return ParsePluginMetadata(pluginJson);
        }
        catch (Exception e)
        {
            Logger.Error($"Read plugin.json failed {configPath}", e);
            return null;
        }
    }

    public static PluginMetadata? ParsePluginMetadata(string pluginJson)
    {
        PluginMetadata? metadata;
        try
        {
            metadata = JsonSerializer.Deserialize<PluginMetadata>(pluginJson, new JsonSerializerOptions
            {
                PropertyNameCaseInsensitive = true
            });
            if (metadata == null)
            {
                Logger.Error($"Invalid json for plugin config {pluginJson}");
                return null;
            }
        }
        catch (Exception e)
        {
            Logger.Error($"Deserialize plugin config failed {pluginJson}", e);
            return null;
        }

        if (metadata.TriggerKeywords.Count == 0)
        {
            Logger.Error($"Plugin {metadata.Name} didn't register any trigger keyword");
            return null;
        }

        if (!PluginRuntime.IsAllowed(metadata.Runtime))
        {
            Logger.Error($"Invalid runtime {metadata.Runtime} for plugin config");
            return null;
        }

        return metadata;
    }
}