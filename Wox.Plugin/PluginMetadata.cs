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
    public required string Website { get; set; }
    public required bool Disabled { get; set; }

    public required string ExecuteFileName { get; set; }

    public required List<string> TriggerKeywords { get; set; }

    public required List<string> Commands { get; set; }

    public required string IcoPath { get; set; }

    // keep plugin raw score by not multiply selected counts
    public required bool KeepResultRawScore { get; set; }

    /// <summary>
    ///     Init time (ms) include both plugin load time and init time
    /// </summary>
    [JsonIgnore]
    public long InitTime { get; set; }

    [JsonIgnore] public long AvgQueryTime { get; set; }

    [JsonIgnore] public int QueryCount { get; set; }

    [JsonIgnore] public string ExecuteFilePath { get; set; }

    public override string? ToString()
    {
        return Name;
    }
}