using System.Text.Json;
using Semver;
using Wox.Core.Utils;

namespace Wox.Core.Plugin.Store;

public static class PluginStoreManager
{
    private static List<PluginManifest> Manifests { get; set; } = new();

    private static List<PluginStore> Stores { get; } = new()
    {
        new PluginStore
        {
            Name = "Wox",
            Url = "https://raw.githubusercontent.com/Wox-launcher/Wox/v2/plugin-store.json"
        }
    };

    /// <summary>
    ///     Load plugin manifests from plugin stores, and update in the background every 10 minutes
    /// </summary>
    public static async Task Load()
    {
        Manifests = await GetManifests();
        Logger.Info($"Loaded {Manifests.Count} plugin manifests from plugin stores");

        _ = Task.Run(async () =>
        {
            var timer = new PeriodicTimer(TimeSpan.FromMinutes(10));
            while (await timer.WaitForNextTickAsync())
            {
                var newerManifest = await GetManifests();
                if (newerManifest.Count > 0)
                {
                    Logger.Info($"Update plugin manifest, count: {newerManifest.Count}");
                    Manifests = newerManifest;
                }
            }
        });
    }

    public static Task Install(PluginManifest manifest)
    {
        return Task.CompletedTask;
    }

    private static async Task<List<PluginManifest>> GetManifests()
    {
        var finalManifests = new List<PluginManifest>();
        foreach (var store in Stores)
        {
            var manifestsInStore = await GetManifestsFromPluginStore(store);
            if (manifestsInStore == null) continue;

            foreach (var pluginManifest in manifestsInStore)
            {
                var existingManifest = finalManifests.FirstOrDefault(o => o.Id == pluginManifest.Id);
                if (existingManifest != null)
                {
                    var existingVersion = SemVersion.Parse(existingManifest.Version, SemVersionStyles.Strict);
                    var currentVersion = SemVersion.Parse(pluginManifest.Version, SemVersionStyles.Strict);
                    if (existingVersion.CompareSortOrderTo(currentVersion) > 0) continue;
                }

                finalManifests.Add(pluginManifest);
            }
        }

        return finalManifests;
    }

    private static async Task<List<PluginManifest>?> GetManifestsFromPluginStore(PluginStore store)
    {
        try
        {
            Logger.Info($"Start to get plugin manifest from {store.Name}({store.Url})");
            var json = await new HttpClient().GetStringAsync(store.Url);
            var manifest = JsonSerializer.Deserialize<List<PluginManifest>>(json);
            manifest?.ForEach(o => o.Store = store);
            Logger.Info($"Got {manifest?.Count} plugin manifests from {store.Name}({store.Url})");
            return manifest;
        }
        catch (Exception e)
        {
            Logger.Error($"Fail to get plugin manifest from {store.Name}({store.Url})", e);
            return null;
        }
    }
}