using Wox.Plugin;

namespace Wox.Core.Plugin.Host;

public interface IPluginHost
{
    public string PluginRuntime { get; }

    public PluginHostStatus Status { get; }

    public Task Start();

    public void Stop();

    public Task<IPlugin?> LoadPlugin(PluginMetadata metadata, string pluginDirectory);

    public void UnloadPlugin(PluginMetadata metadata);
}