using System.Diagnostics;
using Wox.Plugin.App.AppLoader;

namespace Wox.Plugin.App;

public class Main : IPlugin
{
    private IPublicAPI _api = null!;
    private IAppLoader? _appLoader;
    private List<AppInfo>? _apps;

    public async Task Init(PluginInitContext context)
    {
        _api = context.API;
        var isMacos = OperatingSystem.IsMacOS();
        if (isMacos) _appLoader = new MacAppLoader();

        if (_appLoader != null) _apps = await _appLoader.GetAllApps(_api);
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