using System.Diagnostics;
using Wox.Plugin.App.AppLoader;

namespace Wox.Plugin.App;

public class Main : IPlugin
{
    private IPublicAPI _api = null!;
    private IAppLoader? _appLoader;
    private List<AppInfo>? _apps;

    public void Init(PluginInitContext context)
    {
        _api = context.API;
        var isMacos = OperatingSystem.IsMacOS();
        if (isMacos) _appLoader = new MacAppLoader();

        if (_appLoader != null) Task.Run(async () => _apps = await _appLoader.GetAllApps(_api));
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
                    Icon = WoxImage.FromAbsolutePath(app.IconPath),
                    Action = () =>
                    {
                        try
                        {
                            Process.Start(app.Path);
                        }
                        catch (Exception e)
                        {
                            _api.Log(e.Message);
                        }

                        return false;
                    }
                });

        return Task.FromResult(results);
    }
}