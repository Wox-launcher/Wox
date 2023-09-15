using Wox.Core.Plugin.Host;
using Wox.Plugin;

namespace Wox.Core.Plugin;

public class PluginInstance
{
    /// <summary>
    ///     Plugin implementation
    /// </summary>
    public required IPlugin Plugin { get; init; }

    /// <summary>
    ///     APIs exposed to plugin
    /// </summary>
    public required IPublicAPI API { get; init; }

    /// <summary>
    ///     Immutable metadata parsed from plugin.json
    /// </summary>
    public required PluginMetadata Metadata { get; init; }

    /// <summary>
    ///     Is system plugin, see `plugin.md` for more detail
    /// </summary>
    public bool IsSystemPlugin { get; init; } = false;

    /// <summary>
    ///     Trigger keywords to trigger this plugin. Maybe user defined or pre-defined in plugin.json
    /// </summary>
    public List<string> TriggerKeywords => CommonSetting.TriggerKeywords ?? Metadata.TriggerKeywords;

    /// <summary>
    ///     Absolute path to plugin directory
    /// </summary>
    public required string PluginDirectory { get; init; }

    /// <summary>
    ///     Settings that are common to all plugins
    /// </summary>
    public PluginCommonSetting CommonSetting { get; set; } = new();

    /// <summary>
    ///     Settings that are specific to this plugin, See `Doc\Plugin.json.md` Setting specification
    /// </summary>
    public Dictionary<string, string>? PluginSpecificSetting { get; set; } = null;

    /// <summary>
    ///     Plugin host to run this plugin
    /// </summary>
    public required IPluginHost Host { get; init; }

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