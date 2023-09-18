using System.Diagnostics;
using Wox.Core.Utils;
using Wox.Plugin;

namespace Wox.Core.Plugin;

/// <summary>
///     The entry for managing Wox plugins
/// </summary>
public static class PluginManager
{
    private static List<PluginInstance> _pluginInstances = new();

    public static List<PluginInstance> GetAllPlugins()
    {
        return _pluginInstances;
    }

    public static async Task LoadPlugins()
    {
        _pluginInstances = await PluginLoader.LoadPlugins();
        await InitPlugins();
    }

    private static void UnloadPlugin(PluginInstance plugin, string reason)
    {
        plugin.Host.UnloadPlugin(plugin.Metadata);
        _pluginInstances.Remove(plugin);
        Logger.Info($"{plugin.Metadata.Name} plugin was unloaded because {reason}");
    }

    private static async Task InitPlugins()
    {
        await Parallel.ForEachAsync(_pluginInstances, new ParallelOptions { MaxDegreeOfParallelism = 5 }, async (pluginInstance, token) =>
        {
            try
            {
                Logger.Debug($"[{pluginInstance.Metadata.Name}] Start to init plugin ");
                var startTimestamp = Stopwatch.GetTimestamp();
                await pluginInstance.Plugin.Init(new PluginInitContext(pluginInstance.API));
                Logger.Info($"[{pluginInstance.Metadata.Name}] Plugin init cost {Stopwatch.GetElapsedTime(startTimestamp).TotalMilliseconds} ms");
            }
            catch (Exception e)
            {
                e.Data.Add(nameof(pluginInstance.Metadata.Id), pluginInstance.Metadata.Id);
                e.Data.Add(nameof(pluginInstance.Metadata.Name), pluginInstance.Metadata.Name);
                e.Data.Add(nameof(pluginInstance.Metadata.Website), pluginInstance.Metadata.Website);
                Logger.Error($"{pluginInstance.Metadata.Name} Fail to init plugin", e);
                UnloadPlugin(pluginInstance, "failed to init");
                //TODO: need someway to nicely tell user this plugin failed to load
            }
        });
    }

    public static async Task<List<PluginQueryResult>> QueryForPlugin(PluginInstance plugin, Query query)
    {
        Logger.Debug($"[{plugin.Metadata.Name}] start query: {query}");
        if (plugin.CommonSetting.Disabled) return new List<PluginQueryResult>();

        var validGlobalQuery = plugin.TriggerKeywords.Contains("*") && string.IsNullOrEmpty(query.TriggerKeyword);
        var validNonGlobalQuery = plugin.Metadata.TriggerKeywords.Contains(query.TriggerKeyword ?? string.Empty);
        if (!validGlobalQuery && !validNonGlobalQuery) return new List<PluginQueryResult>();

        try
        {
            var results = await plugin.Plugin.Query(query);
            return results.Select(r => new PluginQueryResult
            {
                Result = r,
                AssociatedQuery = query,
                Plugin = plugin
            }).ToList();
        }
        catch (Exception e)
        {
            Logger.Error($"plugin {plugin.Metadata.Name} query ({query}) failed:", e);
            return new List<PluginQueryResult>();
        }
    }
}