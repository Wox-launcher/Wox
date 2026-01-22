using System.Text.Json.Serialization;

namespace Wox.UI.Windows.Models;

public class ToolbarActionItem
{
    [JsonPropertyName("Name")]
    public string Name { get; set; } = string.Empty;

    [JsonPropertyName("Hotkey")]
    public string Hotkey { get; set; } = string.Empty;
}
