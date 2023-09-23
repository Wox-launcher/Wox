using System.Text.Json.Serialization;

namespace Wox.Core.Plugin.Store;

public class PluginManifest
{
    public required string Id { get; init; }
    public required string Name { get; init; }
    public required string Author { get; init; }
    public required string Version { get; init; }
    public required string Runtime { get; init; }
    public required string Description { get; init; }
    public required string IconUrl { get; init; }
    public required string Website { get; init; }
    public required string DownloadUrl { get; init; }
    public required List<string> ScreenshotUrls { get; init; }
    public required string DateCreated { get; init; }
    public required string DateUpdated { get; init; }

    [JsonIgnore] public PluginStore? Store { get; set; }
}