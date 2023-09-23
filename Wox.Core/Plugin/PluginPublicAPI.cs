using Serilog;
using Serilog.Core;
using Wox.Core.I18n;
using Wox.Plugin;

namespace Wox.Core.Plugin;

public class PluginPublicAPI : IPublicAPI
{
    private readonly PluginMetadata _metadata;
    private readonly Logger _pluginLogger;

    public PluginPublicAPI(PluginMetadata metadata)
    {
        _metadata = metadata;
        _pluginLogger = new LoggerConfiguration()
            .WriteTo.File(
                Path.Combine(DataLocation.LogDirectory, "plugins", metadata.Name, "log.txt"),
                outputTemplate: "{Timestamp:yyyy-MM-dd HH:mm:ss.fff} [{Level:u3}] {Message:lj}{NewLine}{Exception}",
                rollOnFileSizeLimit: true,
                retainedFileCountLimit: 3,
                fileSizeLimitBytes: 1024 * 1024 * 100 /*100M*/)
            .MinimumLevel.Debug()
            .CreateLogger();
    }

    public void ChangeQuery(string query)
    {
        ChangeQueryEvent?.Invoke(query);
    }

    public void HideApp()
    {
    }

    public void ShowApp()
    {
    }

    public void ShowMsg(string title, string description = "", string iconPath = "")
    {
    }

    public void Log(string msg)
    {
        _pluginLogger.Information(msg);
    }

    public string GetTranslation(string key)
    {
        return I18NManager.GetPluginTranslation(_metadata.Id, key);
    }

    public static event ChangeQueryEventHandler? ChangeQueryEvent;
}

public delegate void ChangeQueryEventHandler(string query);