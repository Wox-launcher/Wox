using System.Text.Json.Serialization;

namespace Wox.Plugin;

public class PluginMetadata
{
    public required string Id { get; set; }
    public required string Name { get; set; }
    public required string Author { get; set; }
    public required string Version { get; set; }
    public required string Language { get; set; }
    public required string Description { get; set; }
    public required string IcoPath { get; set; }
    public string Website { get; set; } = "";
    public bool Disabled { get; set; }
    public required string ExecuteFileName { get; set; }
    public required List<string> TriggerKeywords { get; set; }
    public List<string> Commands { get; set; } = new();

    [JsonConverter(typeof(JsonPluginSupportedOSConverter))]
    public required List<PluginSupportedOS> SupportedOS { get; set; }

    public override string? ToString()
    {
        return Name;
    }
}