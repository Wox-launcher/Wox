using System;
using System.Text.Json.Serialization;

namespace Wox.UI.Windows.Models;

public class QueryHistory
{
    [JsonPropertyName("Query")]
    public string Query { get; set; } = string.Empty;

    [JsonPropertyName("LastRunTime")]
    public DateTime LastRunTime { get; set; }
}
