using System.Text.Json.Serialization;

namespace Wox.UI.Windows.Models;

public sealed class WoxSetting
{
    [JsonPropertyName("AppWidth")]
    public int AppWidth { get; set; }

    [JsonPropertyName("MaxResultCount")]
    public int MaxResultCount { get; set; }

    [JsonPropertyName("HideOnLostFocus")]
    public bool HideOnLostFocus { get; set; }

    [JsonPropertyName("PreviewWidthRatio")]
    public double PreviewWidthRatio { get; set; }
}
