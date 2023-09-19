using System.Diagnostics;
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

    /// <summary>
    ///     Timestamp when plugin start load
    /// </summary>
    public long LoadStartTimestamp { get; set; }

    /// <summary>
    ///     Timestamp when plugin load finished
    /// </summary>
    public long LoadFinishedTimestamp { get; set; }

    /// <summary>
    ///     Timestamp when plugin start init
    /// </summary>
    public long InitStartTimestamp { get; set; }

    /// <summary>
    ///     Timestamp when plugin init finished
    /// </summary>
    public long InitFinishedTimestamp { get; set; }

    public long LoadTime => Stopwatch.GetElapsedTime(LoadStartTimestamp, LoadFinishedTimestamp).Milliseconds;

    public long InitTime => Stopwatch.GetElapsedTime(InitStartTimestamp, InitFinishedTimestamp).Milliseconds;

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