using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Text.Json.Serialization;

namespace Wox.UI.Windows.Models;

public class AIModel
{
    [JsonPropertyName("Name")]
    public string Name { get; set; } = string.Empty;

    [JsonPropertyName("Provider")]
    public string Provider { get; set; } = string.Empty;

    [JsonPropertyName("ProviderAlias")]
    public string? ProviderAlias { get; set; }
}

public class AIAgent
{
    [JsonPropertyName("Name")]
    public string Name { get; set; } = string.Empty;

    [JsonPropertyName("Prompt")]
    public string Prompt { get; set; } = string.Empty;

    [JsonPropertyName("Model")]
    public AIModel? Model { get; set; }

    [JsonPropertyName("Tools")]
    public List<string>? Tools { get; set; }

    [JsonPropertyName("Icon")]
    public WoxImage? Icon { get; set; }
}

public class AIChatData
{
    [JsonPropertyName("Id")]
    public string Id { get; set; } = string.Empty;

    [JsonPropertyName("Title")]
    public string Title { get; set; } = string.Empty;

    [JsonPropertyName("Conversations")]
    public List<AIChatConversation> Conversations { get; set; } = new();

    [JsonPropertyName("Model")]
    public AIModel? Model { get; set; }

    [JsonPropertyName("CreatedAt")]
    public long CreatedAt { get; set; }

    [JsonPropertyName("UpdatedAt")]
    public long UpdatedAt { get; set; }

    [JsonPropertyName("Tools")]
    public List<string>? Tools { get; set; }

    [JsonPropertyName("AgentName")]
    public string? AgentName { get; set; }
}

public class AIChatConversation
{
    [JsonPropertyName("Id")]
    public string Id { get; set; } = string.Empty;

    [JsonPropertyName("Role")]
    public string Role { get; set; } = string.Empty; // "user" or "assistant"

    [JsonPropertyName("Text")]
    public string Text { get; set; } = string.Empty;

    [JsonPropertyName("Reasoning")]
    public string? Reasoning { get; set; }

    [JsonPropertyName("Images")]
    public List<WoxImage>? Images { get; set; }

    [JsonPropertyName("Timestamp")]
    public long Timestamp { get; set; }

    [JsonPropertyName("ToolCallInfo")]
    public ToolCallInfo? ToolCallInfo { get; set; }

    public bool IsUser => Role == "user";
    public bool IsAssistant => Role == "assistant";
}

public class ToolCallInfo
{
    [JsonPropertyName("Id")]
    public string Id { get; set; } = string.Empty;

    [JsonPropertyName("Name")]
    public string Name { get; set; } = string.Empty;

    [JsonPropertyName("Arguments")]
    public Dictionary<string, object>? Arguments { get; set; }

    [JsonPropertyName("Response")]
    public string Response { get; set; } = string.Empty;

    [JsonPropertyName("Status")]
    public string Status { get; set; } = "pending"; // streaming, pending, running, succeeded, failed

    [JsonPropertyName("Delta")]
    public string Delta { get; set; } = string.Empty;

    [JsonPropertyName("StartTimestamp")]
    public long StartTimestamp { get; set; }

    [JsonPropertyName("EndTimestamp")]
    public long EndTimestamp { get; set; }
}
