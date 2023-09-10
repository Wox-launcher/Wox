namespace Wox.Core.Plugin.Host;

public class NodejsHost : PluginHostBase
{
    public override string PluginRuntime => Plugin.PluginRuntime.Nodejs;

    public override async Task Start()
    {
        await StartHost("node", DataLocation.NodejsHostEntry);
    }

    public override void UnloadPlugin(PluginMetadata metadata)
    {
    }
}