using Wox.Plugin;

namespace Wox.Core.Plugin;

public class PluginInstance
{
    public required IPlugin Plugin { get; init; }
    public required PluginMetadata Metadata { get; init; }

    public bool Disabled { get; set; } = false;

    public bool IsSystemPlugin { get; init; } = false;

    public required string PluginDirectory { get; init; }

    /// <summary>
    ///     for csharp plugins, we load them in a separate AssemblyLoadContext, so we need hold a reference to unload it later
    /// </summary>
    public PluginAssemblyLoadContext? AssemblyLoadContext { get; init; }

    public override string ToString()
    {
        return Metadata.Name;
    }

    public override bool Equals(object? obj)
    {
        if (obj is PluginInstance pluginInstance) return pluginInstance.Metadata.Id == Metadata.Id;

        return false;
    }

    public override int GetHashCode()
    {
        return Metadata.Id.GetHashCode();
    }
}