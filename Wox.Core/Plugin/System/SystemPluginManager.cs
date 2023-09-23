using Wox.Core.Plugin.Store;
using Wox.Plugin;

namespace Wox.Core.Plugin.System;

public class SystemPluginManager : ISystemPlugin
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
        if (query.Command == "install")
            if (query.Search == "")
            {
                //TODO: return featured plugins
            }
            else
            {
                foreach (var pluginManifest in PluginStoreManager.Search(query.Search))
                {
                    var result = new Result
                    {
                        Title = pluginManifest.Name,
                        SubTitle = pluginManifest.Description,
                        Icon = new WoxImage(),
                        Action = async () =>
                        {
                            await PluginStoreManager.Install(pluginManifest);
                            return false;
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
            Id = "f2a471feeff845079d902fa17a969ab1",
            Name = "Wox Plugin Manager",
            Author = "Wox Launcher",
            Version = "1.0.0",
            MinWoxVersion = "2.0.0",
            Runtime = "Dotnet",
            Description = "Plugin manager for Wox",
            Icon = "",
            Entry = "",
            TriggerKeywords = new List<string>
            {
                "wpm"
            },
            Commands = new List<string>
            {
                "install",
                "uninstall"
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