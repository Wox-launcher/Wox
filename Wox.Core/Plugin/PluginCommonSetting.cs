namespace Wox.Core.Plugin;

public class PluginCommonSetting
{
    /// <summary>
    ///     Is this plugin disabled by user
    /// </summary>
    public bool Disabled { get; set; } = false;

    /// <summary>
    ///     User defined keywords, will be used to trigger this plugin. User may not set custom trigger keywords, which will cause this property to be null
    ///     So don't use this property directly, use <see cref="PluginInstance.TriggerKeywords" /> instead
    /// </summary>
    public List<string>? TriggerKeywords { get; set; } = null;
}