using System.Diagnostics;
using Wox.Core.Utils;

namespace Wox.Core.Plugin.Host;

public class NodejsHost : PluginHostBase
{
    public override string PluginRuntime => Plugin.PluginRuntime.Nodejs;

    public override async Task Start()
    {
        var nodePath = await GetNodePath();
        if (string.IsNullOrEmpty(nodePath)) throw new Exception("Nodejs is not in path");

        Logger.Info($"nodejs path is: {nodePath}");
        await StartHost(nodePath, DataLocation.NodejsHostEntry);
    }

    private async Task<string?> GetNodePath()
    {
        var process = new Process
        {
            StartInfo =
            {
                FileName = "which",
                Arguments = "node",
                UseShellExecute = false,
                RedirectStandardOutput = true,
                CreateNoWindow = true
            }
        };
        process.Start();
        var nodePath = await process.StandardOutput.ReadToEndAsync();
        await process.WaitForExitAsync();

        if (string.IsNullOrEmpty(nodePath)) return null;

        return nodePath.Trim();
    }
}