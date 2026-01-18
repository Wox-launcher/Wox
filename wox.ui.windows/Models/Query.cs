using System.Text.Json.Serialization;

namespace Wox.UI.Windows.Models;

public class Query
{
    [JsonPropertyName("QueryId")]
    public string QueryId { get; set; } = string.Empty;

    [JsonPropertyName("RawQuery")]
    public string RawQuery { get; set; } = string.Empty;

    [JsonPropertyName("TriggerKeyword")]
    public string? TriggerKeyword { get; set; }

    [JsonPropertyName("Command")]
    public string Command { get; set; } = string.Empty;

    [JsonPropertyName("Type")]
    public string Type { get; set; } = "input";
}

public class QueryResult
{
    [JsonPropertyName("QueryId")]
    public string QueryId { get; set; } = string.Empty;

    [JsonPropertyName("Results")]
    public List<ResultItem> Results { get; set; } = new();

    [JsonPropertyName("IsFinal")]
    public bool IsFinal { get; set; } = false;
}

public class ResultItem
{
    [JsonPropertyName("Id")]
    public string Id { get; set; } = string.Empty;

    [JsonPropertyName("Title")]
    public string Title { get; set; } = string.Empty;

    [JsonPropertyName("SubTitle")]
    public string SubTitle { get; set; } = string.Empty;

    [JsonPropertyName("Icon")]
    public WoxImage? Icon { get; set; }

    [JsonPropertyName("Preview")]
    public Preview? Preview { get; set; }

    [JsonPropertyName("Score")]
    public long Score { get; set; }

    [JsonPropertyName("ContextData")]
    public string? ContextData { get; set; }

    [JsonPropertyName("Actions")]
    public List<ResultAction>? Actions { get; set; }
}

public class ResultAction
{
    [JsonPropertyName("Id")]
    public string Id { get; set; } = string.Empty;

    [JsonPropertyName("Name")]
    public string Name { get; set; } = string.Empty;

    [JsonPropertyName("Icon")]
    public WoxImage? Icon { get; set; }

    [JsonPropertyName("IsDefault")]
    public bool IsDefault { get; set; }

    [JsonPropertyName("PreventHideAfterAction")]
    public bool PreventHideAfterAction { get; set; }
}

public class WoxImage
{
    [JsonPropertyName("ImageType")]
    public string ImageType { get; set; } = string.Empty;

    [JsonPropertyName("ImageData")]
    public string ImageData { get; set; } = string.Empty;
}

public class Preview
{
    [JsonPropertyName("PreviewType")]
    public string PreviewType { get; set; } = string.Empty;

    [JsonPropertyName("PreviewData")]
    public string PreviewData { get; set; } = string.Empty;

    [JsonPropertyName("PreviewProperties")]
    public Dictionary<string, object>? PreviewProperties { get; set; }
}
