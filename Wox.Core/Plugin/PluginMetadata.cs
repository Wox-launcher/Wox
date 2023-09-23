namespace Wox.Core.Plugin;

/// <summary>
///     Metadata parsed from plugin.json, see `Plugin.json.md` for more detail
///     All properties are immutable after initialization
/// </summary>
public class PluginMetadata
{
    public required string Id { get; init; }
    public required string Name { get; init; }
    public required string Author { get; init; }
    public required string Version { get; init; }

    public required string MinWoxVersion { get; init; }
    public required string Runtime { get; init; }
    public required string Description { get; init; }

    public required string Icon { get; init; }
    public string Website { get; init; } = "";
    public required string Entry { get; init; }

    /// <summary>
    ///     User can add/update/delete trigger keywords
    ///     So don't use this property directly, use <see cref="PluginInstance.TriggerKeywords" /> instead
    /// </summary>
    public required List<string> TriggerKeywords { get; init; }

    public List<string> Commands { get; init; } = new();

    /// <summary>
    ///     See <see cref="PluginSupportedOS" />
    /// </summary>
    public required List<string> SupportedOS { get; init; }

    public override string? ToString()
    {
        return Name;
    }
}