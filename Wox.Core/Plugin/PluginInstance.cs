using Wox.Plugin;

namespace Wox.Core.Plugin;

public class PluginInstance
{
    public PluginInstance(IPlugin plugin, PluginMetadata metadata, PluginAssemblyLoadContext? assemblyLoadContext)
    {
        AssemblyLoadContext = assemblyLoadContext;
        Plugin = plugin;
        Metadata = metadata;
    }

    public IPlugin Plugin { get; private set; }
    public PluginMetadata Metadata { get; }

    /// <summary>
    ///     for csharp plugins, we load them in a separate AssemblyLoadContext, so we need hold a reference to unload it later
    /// </summary>
    public PluginAssemblyLoadContext? AssemblyLoadContext { get; private set; }

    public override string ToString()
    {
        return Metadata.Name;
    }
}