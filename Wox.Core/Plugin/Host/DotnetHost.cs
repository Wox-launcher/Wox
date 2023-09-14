using System.Reflection;
using Wox.Core.Utils;
using Wox.Plugin;

namespace Wox.Core.Plugin.Host;

public class DotnetHost : PluginHostBase
{
    private readonly Dictionary<string, PluginAssemblyLoadContext> _pluginLoadContexts = new();

    public override string PluginRuntime => Plugin.PluginRuntime.Dotnet;

    public override Task Start()
    {
        // do nothing
        return Task.CompletedTask;
    }

    public override void Stop()
    {
        // do nothing
    }

    public override async Task<IPlugin?> LoadPlugin(PluginMetadata metadata, string pluginDirectory)
    {
        try
        {
            var executeFilePath = Path.Combine(pluginDirectory, metadata.Entry);
            var pluginLoadContext = new PluginAssemblyLoadContext(executeFilePath);
            var assembly = pluginLoadContext.LoadFromAssemblyName(new AssemblyName(Path.GetFileNameWithoutExtension(executeFilePath)));
            var type = assembly.GetTypes().FirstOrDefault(o => typeof(IPlugin).IsAssignableFrom(o));
            if (type == null) return null;
            _pluginLoadContexts[metadata.Id] = pluginLoadContext;
            var rawIPlugin = Activator.CreateInstance(type) as IPlugin;
            if (rawIPlugin == null) return null;

            return await Task.Run(() => new DotnetPlugin
            {
                RawPlugin = rawIPlugin
            });
        }
        catch (Exception e)
        {
            Logger.Error($"Couldn't load assembly for dotnet plugin {metadata.Name}", e);
#if DEBUG
            throw;
#else
            return null;
#endif
        }
    }

    public override Task UnloadPlugin(PluginMetadata metadata)
    {
        if (_pluginLoadContexts.TryGetValue(metadata.Id, out var pluginLoadContext))
        {
            pluginLoadContext.Unload();
            _pluginLoadContexts.Remove(metadata.Id);
        }

        return Task.CompletedTask;
    }
}