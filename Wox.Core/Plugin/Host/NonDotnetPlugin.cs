using System.Text.Json;
using Wox.Core.Utils;
using Wox.Plugin;

namespace Wox.Core.Plugin.Host;

public class NonDotnetPlugin : IPlugin
{
    public required PluginMetadata Metadata { get; init; }
    public required PluginHostBase PluginHost { get; init; }

    public Task Init(PluginInitContext context)
    {
        return PluginHost.InvokeMethod(Metadata, "init");
    }

    public async Task<List<Result>> Query(Query query)
    {
        var rawResults = await PluginHost.InvokeMethod(Metadata, "query", new Dictionary<string, string?>
        {
            { "RawQuery", query.RawQuery },
            { "TriggerKeyword", query.TriggerKeyword },
            { "Command", query.Command },
            { "Search", query.Search }
        });

        if (rawResults == null)
            return new List<Result>();

        var results = rawResults.Value.Deserialize<List<Result>>();
        if (results == null)
        {
            Logger.Error($"[{Metadata.Name}] Fail to deserialize query result");
            return new List<Result>();
        }

        foreach (var result in results)
            result.Action = async () =>
            {
                var actionRawResult = await PluginHost.InvokeMethod(Metadata, "action", new Dictionary<string, string?>
                {
                    { "ActionId", result.Id }
                });
                if (actionRawResult == null) return true;
                return actionRawResult.Value.Deserialize<bool>();
            };

        return results;
    }
}