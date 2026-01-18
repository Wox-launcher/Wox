using System.Text.Json.Serialization;

namespace Wox.UI.Windows.Models;

public class WebsocketMsg
{
    [JsonPropertyName("RequestId")]
    public string RequestId { get; set; } = string.Empty;

    [JsonPropertyName("TraceId")]
    public string TraceId { get; set; } = string.Empty;

    [JsonPropertyName("SessionId")]
    public string SessionId { get; set; } = string.Empty;

    [JsonPropertyName("Method")]
    public string Method { get; set; } = string.Empty;

    [JsonPropertyName("Type")]
    public string Type { get; set; } = string.Empty;

    [JsonPropertyName("Data")]
    public object? Data { get; set; }

    [JsonPropertyName("Success")]
    public bool Success { get; set; }
}

public static class WebsocketMsgType
{
    public const string Request = "WebsocketMsgTypeRequest";
    public const string Response = "WebsocketMsgTypeResponse";
}
