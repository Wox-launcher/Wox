using System.Text.Json;

namespace Wox.Core.Plugin.Host;

public static class PluginJsonRpcType
{
    public const string Request = "WOX_JSONRPC_REQUEST";
    public const string Response = "WOX_JSONRPC_RESPONSE";
}

public class PluginJsonRpcRequest
{
    public required string Id { get; set; }

    public required string PluginId { get; set; }

    public required string PluginName { get; set; }
    public required string Method { get; set; }

    public required string Type { get; set; }

    public Dictionary<string, string?>? Params { get; set; }
}

public class PluginJsonRpcResponse
{
    public required string Id { get; set; }

    public required string Method { get; set; }

    public required string Type { get; set; }

    public string? Error { get; set; }

    public JsonElement? Result { get; set; }
}