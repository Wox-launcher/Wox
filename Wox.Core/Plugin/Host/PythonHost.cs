namespace Wox.Core.Plugin.Host;

public class PythonHost : PluginHostBase
{
    public override string PluginRuntime => Plugin.PluginRuntime.Python;

    public override async Task Start()
    {
        await StartHost("python3", DataLocation.PythonHostEntry);
    }
}