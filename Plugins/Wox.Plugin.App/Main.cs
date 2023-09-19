using System.Diagnostics;
using System.Text.Json;
using Wox.Plugin.App.AppLoader;

namespace Wox.Plugin.App;

public class Main : IPlugin
{
    private IPublicAPI _api = null!;
    private IAppLoader? _appLoader;
    private List<AppInfo>? _apps;
    private readonly string _cachePath = Path.Combine(Path.GetTempPath(), "Wox.Plugin.App", "app.cache");

    public async Task Init(PluginInitContext context)
    {
        _api = context.API;

        await LoadAppCache();

        _ = Task.Delay(2000).ContinueWith(async _ => { await IndexApps(); });
    }

    private async Task LoadAppCache()
    {
        if (!File.Exists(_cachePath))
        {
            _api.Log("App cache not found");
            Directory.CreateDirectory(Path.GetDirectoryName(_cachePath) ?? string.Empty);
            return;
        }

        var cache = await File.ReadAllTextAsync(_cachePath);
        _apps = JsonSerializer.Deserialize<List<AppInfo>>(cache);
        _api.Log($"App cache loaded, count: {_apps?.Count}");
    }


    private async Task IndexApps()
    {
        _api.Log($"Start to index apps");
        var startTimestamp = Stopwatch.GetTimestamp();

        var isMacos = OperatingSystem.IsMacOS();
        if (isMacos) _appLoader = new MacAppLoader();

        if (_appLoader != null) _apps = await _appLoader.GetAllApps(_api);

        //save app to cache
        if (_apps != null)
        {
            var cache = JsonSerializer.Serialize(_apps);
            await File.WriteAllTextAsync(_cachePath, cache);
            _api.Log($"App cache saved, count: {_apps.Count}, path: {_cachePath}");
        }

        _api.Log($"Index apps cost {Stopwatch.GetElapsedTime(startTimestamp).TotalMilliseconds} ms");
    }

    public Task<List<Result>> Query(Query query)
    {
        var results = new List<Result>();
        if (_apps == null) return Task.FromResult(results);

        foreach (var app in _apps)
            if (app.Name.Contains(query.Search, StringComparison.OrdinalIgnoreCase))
                results.Add(new Result
                {
                    Title = app.Name,
                    SubTitle = app.Path,
                    Icon = new WoxImage { ImageType = WoxImageType.AbsolutePath, ImageData = app.IconPath },
                    Action = () =>
                    {
                        try
                        {
                            Process.Start(new ProcessStartInfo
                            {
                                FileName = "open",
                                Arguments = app.Path,
                                UseShellExecute = true
                            });
                        }
                        catch (Exception e)
                        {
                            _api.Log(e.Message);
                        }

                        return Task.FromResult(true);
                    }
                });

        return Task.FromResult(results);
    }
}