using Wox.Plugin;

namespace Wox.Core.Plugin.System;

public class SystemPluginIndicator : ISystemPlugin
{
    private IPublicAPI _api = null!;

    public Task Init(PluginInitContext context)
    {
        _api = context.API;
        return Task.CompletedTask;
    }

    public Task<List<Result>> Query(Query query)
    {
        List<Result> results = new();
        foreach (var pluginInstance in PluginManager.GetAllPlugins())
        {
            var triggerKeyword = pluginInstance.TriggerKeywords.Find(o => o != "*" && o.Contains(query.Search));
            if (triggerKeyword != null)
            {
                var result = new Result
                {
                    Title = triggerKeyword,
                    SubTitle = $"Activate {pluginInstance.Metadata.Name} plugin",
                    Icon = new WoxImage(),
                    Action = () =>
                    {
                        _api.ChangeQuery($"{triggerKeyword} ");
                        return Task.FromResult(false);
                    }
                };
                results.Add(result);
            }
        }

        return Task.FromResult(results);
    }

    public PluginMetadata GetMetadata()
    {
        return new PluginMetadata
        {
            Id = "39a4a6155f094ef89778188ae4a3ca03",
            Name = "System Plugin Indicator",
            Author = "Wox Launcher",
            Version = "1.0.0",
            MinWoxVersion = "2.0.0",
            Runtime = "Dotnet",
            Description = "Indicator for plugin trigger keywords",
            Icon = "",
            Entry = "",
            TriggerKeywords = new List<string>
            {
                "*"
            },
            SupportedOS = new List<string>
            {
                PluginSupportedOS.Windows,
                PluginSupportedOS.Linux,
                PluginSupportedOS.Macos
            }
        };
    }
}