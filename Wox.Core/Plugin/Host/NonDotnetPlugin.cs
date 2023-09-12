using Wox.Plugin;

namespace Wox.Core.Plugin.Host;

public class NonDotnetPlugin : IPlugin
{
    public required PluginMetadata Metadata { get; init; }
    public required PluginHostBase PluginHost { get; init; }

    public void Init(PluginInitContext context)
    {
        PluginHost.InvokeMethod(Metadata, "init").Wait();
    }

    public async Task<List<Result>> Query(Query query)
    {
        await PluginHost.InvokeMethod(Metadata, "query", new Dictionary<string, string?>
        {
            { "RawQuery", query.RawQuery },
            { "TriggerKeyword", query.TriggerKeyword },
            { "Command", query.Command },
            { "Search", query.Search }
        });

        return new List<Result>();
    }
}