using System.Text.Json.Serialization;
using Wox.Plugin;

namespace Wox.Core.Plugin;

public class PluginMetadata
{
    public required string Id { get; init; }
    public required string Name { get; init; }
    public required string Author { get; init; }
    public required string Version { get; init; }
    public required string Language { get; init; }
    public required string Description { get; init; }
    public required string IcoPath { get; init; }
    public string Website { get; init; } = "";
    public required string ExecuteFileName { get; init; }
    public required List<string> TriggerKeywords { get; init; }
    public List<string> Commands { get; init; } = new();

    [JsonConverter(typeof(JsonPluginSupportedOSConverter))]
    public required List<PluginSupportedOS> SupportedOS { get; init; }

    public override string? ToString()
    {
        return Name;
    }
}