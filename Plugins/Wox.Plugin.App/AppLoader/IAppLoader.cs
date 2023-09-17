namespace Wox.Plugin.App.AppLoader;

public interface IAppLoader
{
    public Task<List<AppInfo>> GetAllApps(IPublicAPI publicAPI);
}