using System;
using System.Collections.Generic;
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
    public string? SubTitle { get; set; }

    [JsonPropertyName("Icon")]
    public WoxImage? Icon { get; set; }

    [JsonPropertyName("Preview")]
    public Preview? Preview { get; set; }

    [JsonPropertyName("Score")]
    public long Score { get; set; }

    [JsonPropertyName("ContextData")]
    public string? ContextData { get; set; }

    [JsonPropertyName("Actions")]
    public List<ActionItem>? Actions { get; set; }

    [JsonPropertyName("AutoComplete")]
    public string? AutoComplete { get; set; }
}

public class ActionItem
{
    [JsonPropertyName("Id")]
    public string Id { get; set; } = string.Empty;

    [JsonPropertyName("Name")]
    public string Name { get; set; } = string.Empty;

    [JsonPropertyName("Type")]
    public string Type { get; set; } = "execute";

    [JsonPropertyName("Icon")]
    public WoxImage? Icon { get; set; }

    [JsonPropertyName("IsDefault")]
    public bool IsDefault { get; set; }

    [JsonPropertyName("PreventHideAfterAction")]
    public bool PreventHideAfterAction { get; set; }

    [JsonPropertyName("Hotkey")]
    public string? Hotkey { get; set; }

    [JsonPropertyName("Form")]
    public List<PluginSettingDefinitionItem> Form { get; set; } = new();
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

    [JsonPropertyName("ScrollPosition")]
    public string? ScrollPosition { get; set; }
}

public class GridLayoutParams
{
    [JsonPropertyName("Columns")]
    public int Columns { get; set; } = 8;

    [JsonPropertyName("ShowTitle")]
    public bool ShowTitle { get; set; }

    [JsonPropertyName("ItemPadding")]
    public double ItemPadding { get; set; } = 12;

    [JsonPropertyName("ItemMargin")]
    public double ItemMargin { get; set; } = 6;

    [JsonPropertyName("Commands")]
    public List<string> Commands { get; set; } = new();

    public static GridLayoutParams Empty() => new();
}

public class QueryMetadata
{
    [JsonPropertyName("Icon")]
    public WoxImage? Icon { get; set; }

    [JsonPropertyName("WidthRatio")]
    public double ResultPreviewWidthRatio { get; set; } = 0.5;

    [JsonPropertyName("IsGridLayout")]
    public bool IsGridLayout { get; set; }

    [JsonPropertyName("GridLayoutParams")]
    public GridLayoutParams? GridLayoutParams { get; set; }
}

public class QueryIconInfo
{
    public WoxImage? Icon { get; set; }
    public System.Action? ClickAction { get; set; }

    public static QueryIconInfo Empty() => new() { Icon = null, ClickAction = null };
}

public class DoctorCheckResult
{
    [JsonPropertyName("Name")]
    public string Name { get; set; } = string.Empty;

    [JsonPropertyName("Type")]
    public string Type { get; set; } = string.Empty;

    [JsonPropertyName("Passed")]
    public bool Passed { get; set; }

    [JsonPropertyName("Description")]
    public string Description { get; set; } = string.Empty;

    [JsonPropertyName("ActionName")]
    public string ActionName { get; set; } = string.Empty;

    [JsonPropertyName("Preview")]
    public Dictionary<string, object>? Preview { get; set; }

    public bool IsVersionIssue => Type.Equals("update", StringComparison.OrdinalIgnoreCase) && !Passed;
    public bool IsPermissionIssue => Type.Equals("accessibility", StringComparison.OrdinalIgnoreCase) && !Passed;
}

public class DoctorCheckInfo
{
    public List<DoctorCheckResult> Results { get; set; } = new();
    public bool AllPassed { get; set; } = true;
    public WoxImage? Icon { get; set; }
    public string Message { get; set; } = string.Empty;

    public static DoctorCheckInfo Empty() => new() { Results = new(), AllPassed = true, Icon = null, Message = string.Empty };
}
