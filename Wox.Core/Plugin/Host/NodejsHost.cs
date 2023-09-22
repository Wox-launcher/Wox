using System.Diagnostics;
using Wox.Core.Utils;

namespace Wox.Core.Plugin.Host;

public class NodejsHost : PluginHostBase
{
    public override string PluginRuntime => Plugin.PluginRuntime.Nodejs;

    public override async Task Start()
    {
        await StartHost("/opt/homebrew/bin/node", DataLocation.NodejsHostEntry);
    }
}