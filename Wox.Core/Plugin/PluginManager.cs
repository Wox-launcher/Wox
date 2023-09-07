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

    /// <summary>
    ///     because InitializePlugins needs API, so LoadPlugins needs to be called first
    /// </summary>
    public static void LoadPlugins(IPublicAPI api)
    {
        _pluginInstances = PluginLoader.LoadPlugins();
        InitPlugins(api);
    }

    private static void InitPlugins(IPublicAPI api)
    {
        Parallel.ForEach(_pluginInstances, pluginInstance =>
        {
            try
            {
                Logger.Debug($"Start to init plugin {pluginInstance.Metadata.Name}");
                var startTime = DateTime.Now;
                pluginInstance.Plugin.Init(new PluginInitContext(pluginInstance.Metadata, api));
                pluginInstance.Metadata.InitTime += DateTime.Now.Subtract(startTime).Milliseconds;
                Logger.Info($"init plugin {pluginInstance.Metadata.Name} success, total init cost is {pluginInstance.Metadata.InitTime}ms");
            }
            catch (Exception e)
            {
                e.Data.Add(nameof(pluginInstance.Metadata.Id), pluginInstance.Metadata.Id);
                e.Data.Add(nameof(pluginInstance.Metadata.Name), pluginInstance.Metadata.Name);
                e.Data.Add(nameof(pluginInstance.Metadata.Website), pluginInstance.Metadata.Website);
                Logger.Error($"Fail to init plugin: {pluginInstance.Metadata.Name}", e);
                pluginInstance.Metadata.Disabled = true;
                //TODO: need someway to nicely tell user this plugin failed to load
            }
        });
    }

    public static List<Result> QueryForPlugin(PluginMetadata metadata, Query query)
    {
        if (metadata.Disabled) return new List<Result>();
        var pluginInstance = _pluginInstances.FirstOrDefault(o => o.Metadata.Id == metadata.Id);
        if (pluginInstance == null)
        {
            Logger.Error($"Plugin {metadata.Name} cannot be found for query");
            return new List<Result>();
        }

        var validGlobalQuery = string.IsNullOrEmpty(query.TriggerKeyword);
        var validNonGlobalQuery = metadata.TriggerKeywords.Contains(query.TriggerKeyword);
        if (!validGlobalQuery && !validNonGlobalQuery) return new List<Result>();

        try
        {
            var startTime = DateTime.Now;
            var results = pluginInstance.Plugin.Query(query);
            MergePluginQueryResults(results, metadata, query);
            var queryTime = DateTime.Now.Subtract(startTime).Milliseconds;

            metadata.QueryCount += 1;
            metadata.AvgQueryTime = metadata.QueryCount == 1 ? queryTime : (metadata.AvgQueryTime + queryTime) / 2;
            return results;
        }
        catch (Exception e)
        {
            e.Data.Add(nameof(metadata.Id), metadata.Id);
            e.Data.Add(nameof(metadata.Name), metadata.Name);
            e.Data.Add(nameof(metadata.Website), metadata.Website);
            Logger.Error($"Exception for plugin {metadata.Name} when query <{query}>", e);
            return new List<Result>();
        }
    }

    private static void MergePluginQueryResults(List<Result> results, PluginMetadata metadata, Query query)
    {
        foreach (var r in results)
        {
            r.PluginID = metadata.Id;
            r.OriginQuery = query;

            const string key = "EmbededIcon:";
            // TODO: use icon path type enum in the future
            if (!string.IsNullOrEmpty(r.PluginDirectory) && !string.IsNullOrEmpty(r.IcoPath) && !Path.IsPathRooted(r.IcoPath) && !r.IcoPath.StartsWith(key))
                r.IcoPath = Path.Combine(r.PluginDirectory, r.IcoPath);
        }
    }
}