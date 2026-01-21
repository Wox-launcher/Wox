using System;
using System.Collections.Generic;
using System.Net.Http;
using System.Text;
using System.Text.Json;
using System.Threading.Tasks;
using System.Windows;
using Wox.UI.Windows.Models;
using WS = WebSocketSharp;

namespace Wox.UI.Windows.Services;

public class WoxApiService : IDisposable
{
    private static readonly Lazy<WoxApiService> _instance = new(() => new WoxApiService());
    public static WoxApiService Instance => _instance.Value;

    private WS.WebSocket? _wsClient;
    private HttpClient? _httpClient;
    private int _serverPort;
    private string _sessionId = string.Empty;
    private readonly Dictionary<string, TaskCompletionSource<WebsocketMsg>> _pendingRequests = new();
    private bool _isConnected = false;
    private TaskCompletionSource<bool> _connectionTcs = new();

    public event EventHandler<QueryResult>? ResultsReceived;
    public event EventHandler<string>? QueryChanged;
    public event EventHandler? ShowRequested;
    public event EventHandler? HideRequested;
    public event EventHandler? ToggleRequested;
    public event EventHandler<WoxSetting>? SettingLoaded;
    public event EventHandler<bool>? RefreshQueryRequested;
    public event EventHandler<string>? ToolbarMsgReceived;
    public event Func<object?>? GetCurrentQueryRequested;
    public event EventHandler<string>? UpdateResultReceived;
    public event Func<string, string[]?>? PickFilesRequested;
    public event EventHandler? FocusToChatInputRequested;
    public event EventHandler<string>? ChatResponseReceived;
    public event EventHandler<string>? ReloadChatResourcesRequested;

    private WoxApiService() { }

    public void Initialize(int serverPort, string sessionId)
    {
        _serverPort = serverPort;
        _sessionId = sessionId;
        _httpClient = new HttpClient
        {
            BaseAddress = new Uri($"http://localhost:{serverPort}")
        };
        _httpClient.DefaultRequestHeaders.Add("SessionId", _sessionId);
    }

    public async Task ConnectAsync()
    {
        var wsUrl = $"ws://localhost:{_serverPort}/ws";
        Logger.Log($"Connecting to WebSocket: {wsUrl}");

        _wsClient = new WS.WebSocket(wsUrl);

        _wsClient.OnOpen += (sender, e) =>
        {
            Logger.Log("WebSocket connected!");
            _isConnected = true;
            _connectionTcs.TrySetResult(true);
        };

        _wsClient.OnMessage += (sender, e) =>
        {
            if (e.Data != null)
            {
                Logger.LogLocal($"Received message: {e.Data.Substring(0, Math.Min(100, e.Data.Length))}");
                HandleWebSocketMessage(e.Data);
            }
        };

        _wsClient.OnClose += (sender, e) =>
        {
            Logger.Log($"WebSocket disconnected: {e.Code} - {e.Reason}");
            _isConnected = false;
        };

        _wsClient.OnError += (sender, e) =>
        {
            Logger.Error("WebSocket error", new Exception(e.Message));
            _connectionTcs.TrySetException(new Exception(e.Message));
        };

        _wsClient.Connect();

        // Wait for connection to complete (with timeout)
        await Task.WhenAny(_connectionTcs.Task, Task.Delay(5000));

        if (!_isConnected)
        {
            Logger.Log("WebSocket connection timeout or failed");
            throw new Exception("Failed to connect to WebSocket");
        }
    }

    private void HandleWebSocketMessage(string message)
    {
        try
        {
            var msg = JsonSerializer.Deserialize<WebsocketMsg>(message);
            if (msg == null) return;

            if (msg.Type == WebsocketMsgType.Response)
            {
                // Handle response to our request
                if (_pendingRequests.TryGetValue(msg.RequestId, out var tcs))
                {
                    tcs.SetResult(msg);
                    _pendingRequests.Remove(msg.RequestId);
                }

                // Query responses also contain results in Data
                if (msg.Method == "Query" && msg.Data != null)
                {
                    Application.Current.Dispatcher.Invoke(() =>
                    {
                        try
                        {
                            var json = JsonSerializer.Serialize(msg.Data);

                            // Check if Data is an empty array (no results)
                            if (json == "[]")
                            {
                                Logger.Log("Query response with empty results array");
                                var emptyResult = new QueryResult
                                {
                                    QueryId = msg.RequestId,
                                    Results = new List<ResultItem>(),
                                    IsFinal = true
                                };
                                ResultsReceived?.Invoke(this, emptyResult);
                                return;
                            }

                            Logger.Log($"Query response data: {json.Substring(0, Math.Min(300, json.Length))}");

                            var queryResult = JsonSerializer.Deserialize<QueryResult>(json);
                            if (queryResult != null && queryResult.Results != null)
                            {
                                Logger.Log($"Parsed {queryResult.Results.Count} results, QueryId: {queryResult.QueryId}, IsFinal: {queryResult.IsFinal}");
                                ResultsReceived?.Invoke(this, queryResult);
                            }
                            else
                            {
                                Logger.Log("Failed to parse QueryResult or Results is null");
                            }
                        }
                        catch (Exception ex)
                        {
                            Logger.Error("Error parsing Query response Data", ex);
                        }
                    });
                }
            }
            else if (msg.Type == WebsocketMsgType.Request)
            {
                // Handle request from server
                Application.Current.Dispatcher.Invoke(() =>
                {
                    HandleServerRequest(msg);
                });
            }
        }
        catch (Exception ex)
        {
            Logger.Error("Error handling WebSocket message", ex);
        }
    }

    private void HandleServerRequest(WebsocketMsg msg)
    {
        Logger.Log($"Received server request: {msg.Method}");

        switch (msg.Method)
        {
            case "PushResults":
                if (msg.Data != null)
                {
                    try
                    {
                        var json = JsonSerializer.Serialize(msg.Data);
                        Logger.Log($"PushResults data: {json.Substring(0, Math.Min(200, json.Length))}");

                        var queryResult = JsonSerializer.Deserialize<QueryResult>(json);
                        if (queryResult != null && queryResult.Results != null)
                        {
                            Logger.Log($"Parsed {queryResult.Results.Count} results, QueryId: {queryResult.QueryId}, IsFinal: {queryResult.IsFinal}");
                            ResultsReceived?.Invoke(this, queryResult);
                        }
                        else
                        {
                            Logger.Log("Failed to parse QueryResult or Results is null");
                        }
                    }
                    catch (Exception ex)
                    {
                        Logger.Error("Error parsing PushResults", ex);
                    }
                }
                SendResponse(msg.RequestId, msg.TraceId, true, null);
                break;

            case "ChangeQuery":
                if (msg.Data != null)
                {
                    var json = JsonSerializer.Serialize(msg.Data);
                    var queryData = JsonSerializer.Deserialize<Dictionary<string, object>>(json);
                    if (queryData != null && queryData.TryGetValue("RawQuery", out var rawQuery))
                    {
                        QueryChanged?.Invoke(this, rawQuery?.ToString() ?? "");
                    }
                }
                SendResponse(msg.RequestId, msg.TraceId, true, null);
                break;

            case "ShowApp":
                ShowRequested?.Invoke(this, EventArgs.Empty);
                SendResponse(msg.RequestId, msg.TraceId, true, null);
                break;

            case "HideApp":
                HideRequested?.Invoke(this, EventArgs.Empty);
                SendResponse(msg.RequestId, msg.TraceId, true, null);
                break;

            case "ToggleApp":
                ToggleRequested?.Invoke(this, EventArgs.Empty);
                SendResponse(msg.RequestId, msg.TraceId, true, null);
                break;

            case "ChangeTheme":
                if (msg.Data != null)
                {
                    var json = JsonSerializer.Serialize(msg.Data);
                    ThemeService.Instance.ApplyTheme(json);
                }
                SendResponse(msg.RequestId, msg.TraceId, true, null);
                break;
            case "ReloadSetting":
                _ = LoadSettingAsync();
                SendResponse(msg.RequestId, msg.TraceId, true, null);
                break;

            case "RefreshQuery":
                if (msg.Data != null)
                {
                    var json = JsonSerializer.Serialize(msg.Data);
                    var refreshData = JsonSerializer.Deserialize<Dictionary<string, object>>(json);
                    var preserveSelectedIndex = false;
                    if (refreshData != null && refreshData.TryGetValue("preserveSelectedIndex", out var preserve))
                    {
                        preserveSelectedIndex = preserve is JsonElement je && je.GetBoolean();
                    }
                    RefreshQueryRequested?.Invoke(this, preserveSelectedIndex);
                }
                SendResponse(msg.RequestId, msg.TraceId, true, null);
                break;

            case "ShowToolbarMsg":
                if (msg.Data != null)
                {
                    var json = JsonSerializer.Serialize(msg.Data);
                    ToolbarMsgReceived?.Invoke(this, json);
                }
                SendResponse(msg.RequestId, msg.TraceId, true, null);
                break;

            case "GetCurrentQuery":
                var currentQuery = GetCurrentQueryRequested?.Invoke();
                SendResponse(msg.RequestId, msg.TraceId, true, currentQuery);
                break;

            case "UpdateResult":
                if (msg.Data != null)
                {
                    var json = JsonSerializer.Serialize(msg.Data);
                    UpdateResultReceived?.Invoke(this, json);
                }
                SendResponse(msg.RequestId, msg.TraceId, true, null);
                break;

            case "PickFiles":
                if (msg.Data != null)
                {
                    var json = JsonSerializer.Serialize(msg.Data);
                    var files = PickFilesRequested?.Invoke(json);
                    SendResponse(msg.RequestId, msg.TraceId, true, files);
                }
                else
                {
                    SendResponse(msg.RequestId, msg.TraceId, true, new string[0]);
                }
                break;

            case "FocusToChatInput":
                FocusToChatInputRequested?.Invoke(this, EventArgs.Empty);
                SendResponse(msg.RequestId, msg.TraceId, true, null);
                break;

            case "SendChatResponse":
                if (msg.Data != null)
                {
                    var json = JsonSerializer.Serialize(msg.Data);
                    ChatResponseReceived?.Invoke(this, json);
                }
                SendResponse(msg.RequestId, msg.TraceId, true, null);
                break;

            case "ReloadChatResources":
                if (msg.Data != null)
                {
                    var resourceName = msg.Data?.ToString() ?? "";
                    ReloadChatResourcesRequested?.Invoke(this, resourceName);
                }
                SendResponse(msg.RequestId, msg.TraceId, true, null);
                break;
        }
    }

    private void SendResponse(string requestId, string traceId, bool success, object? data)
    {
        var response = new WebsocketMsg
        {
            RequestId = requestId,
            TraceId = traceId,
            SessionId = _sessionId,
            Type = WebsocketMsgType.Response,
            Success = success,
            Data = data
        };

        var json = JsonSerializer.Serialize(response);
        _wsClient?.Send(json);
    }

    public async Task SendQueryAsync(string queryText)
    {
        var queryData = new
        {
            queryId = Guid.NewGuid().ToString(),
            queryType = "input",
            queryText = queryText,
            querySelection = new
            {
                Type = "",
                Text = "",
                FilePaths = new string[0]
            }
        };

        await SendRequestAsync("Query", queryData);
    }

    public async Task SendActionAsync(string queryId, string resultId, string actionId)
    {
        var actionData = new
        {
            QueryId = queryId,
            ResultId = resultId,
            ActionId = actionId
        };

        await SendRequestAsync("Action", actionData);
    }

    private async Task SendRequestAsync(string method, object data)
    {
        if (!_isConnected || _wsClient == null)
        {
            Logger.Log($"WebSocket not connected, waiting... (Method: {method})");
            await _connectionTcs.Task;
        }

        var msg = new WebsocketMsg
        {
            RequestId = Guid.NewGuid().ToString(),
            TraceId = Guid.NewGuid().ToString(),
            SessionId = _sessionId,
            Method = method,
            Type = WebsocketMsgType.Request,
            Data = data
        };

        var json = JsonSerializer.Serialize(msg);
        Logger.Log($"Sending request: {method}");
        _wsClient?.Send(json);
        await Task.CompletedTask;
    }

    public bool TrySendLog(string traceId, string level, string message)
    {
        if (!_isConnected || _wsClient == null || string.IsNullOrWhiteSpace(_sessionId))
        {
            return false;
        }

        try
        {
            var logData = new
            {
                traceId,
                level,
                message
            };

            var msg = new WebsocketMsg
            {
                RequestId = Guid.NewGuid().ToString(),
                TraceId = traceId,
                SessionId = _sessionId,
                Method = "Log",
                Type = WebsocketMsgType.Request,
                Data = logData
            };

            var json = JsonSerializer.Serialize(msg);
            _wsClient.Send(json);
            return true;
        }
        catch
        {
            return false;
        }
    }

    public async Task NotifyUIReadyAsync()
    {
        try
        {
            if (_httpClient != null)
            {
                await _httpClient.PostAsync("/on/ready", null);
            }
        }
        catch (Exception ex)
        {
            Logger.Error("Error notifying UI ready", ex);
        }
    }

    public async Task NotifyFocusLostAsync()
    {
        try
        {
            if (_httpClient != null)
            {
                await _httpClient.PostAsync("/on/focus/lost", null);
            }
        }
        catch (Exception ex)
        {
            Logger.Error("Error notifying focus lost", ex);
        }
    }

    public async Task LoadThemeAsync()
    {
        try
        {
            if (_httpClient == null)
            {
                return;
            }

            var response = await _httpClient.PostAsync("/theme", null);
            response.EnsureSuccessStatusCode();

            var themeJson = await response.Content.ReadAsStringAsync();
            if (!string.IsNullOrWhiteSpace(themeJson))
            {
                ThemeService.Instance.ApplyTheme(themeJson);
            }
        }
        catch (Exception ex)
        {
            Logger.Error("Error loading theme", ex);
        }
    }

    public async Task LoadSettingAsync()
    {
        try
        {
            if (_httpClient == null)
            {
                return;
            }

            var response = await _httpClient.PostAsync("/setting/wox", null);
            response.EnsureSuccessStatusCode();

            var json = await response.Content.ReadAsStringAsync();
            if (string.IsNullOrWhiteSpace(json))
            {
                return;
            }

            var setting = JsonSerializer.Deserialize<WoxSetting>(json);
            if (setting != null)
            {
                SettingLoaded?.Invoke(this, setting);
            }
        }
        catch (Exception ex)
        {
            Logger.Error("Error loading setting", ex);
        }
    }

    public void Dispose()
    {
        _wsClient?.Close();
        _httpClient?.Dispose();
    }
}
