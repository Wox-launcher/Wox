using Wox.Plugin;

namespace Wox.Core.Plugin.Host;

public class DotnetPlugin : IPlugin
{
    public required IPlugin RawPlugin { get; init; }

    public void Init(PluginInitContext context)
    {
        RawPlugin.Init(context);
    }

    public async Task<List<Result>> Query(Query query)
    {
        var results = await RawPlugin.Query(query);

        //set result id if not set
        foreach (var result in results)
            if (result.Id == "")
                result.Id = Guid.NewGuid().ToString();

        return results;
    }
}