using System.Diagnostics;
using System.Text;
using System.Text.Json;
using Websocket.Client;
using Wox.Core.Utils;
using Wox.Plugin;

namespace Wox.Core.Plugin.Host;

public abstract class PluginHostBase : IPluginHost
{
    private readonly CancellationTokenSource _cts = new();
    private readonly Dictionary<string, TaskCompletionSource<PluginJsonRpcResponse>> _invokeMethodTaskCompletes = new();
    private WebsocketClient _websocket = null!;

    /// <summary>
    ///     Is this host started successfully
    /// </summary>
    protected bool IsStarted { get; set; } = false;

    public abstract string PluginRuntime { get; }

    public PluginHostStatus Status { get; private set; } = PluginHostStatus.Init;

    public abstract Task Start();

    public virtual void Stop()
    {
        _cts.Cancel();
        Status = PluginHostStatus.Stopped;
    }

    public abstract void UnloadPlugin(PluginMetadata metadata);

    public virtual IPlugin? LoadPlugin(PluginMetadata metadata, string pluginDirectory)
    {
        InvokeMethod(metadata, "loadPlugin", new Dictionary<string, string?>
        {
            { "PluginId", metadata.Id },
            { "PluginDirectory", pluginDirectory },
            { "Entry", metadata.Entry }
        });

        return new NonDotnetPlugin
        {
            Metadata = metadata,
            PluginHost = this
        };
    }

    protected async Task StartHost(string fileName, string entry)
    {
        var websocketServerPort = Network.GetAvailableTcpPort();
        if (websocketServerPort == null)
            throw new Exception($"Failed to start {fileName} plugin host, failed to get random tcp port");

        var process = Process.Start(new ProcessStartInfo
        {
            FileName = fileName,
            Arguments = $"{entry} {websocketServerPort} \"{DataLocation.LogDirectory}\"",
            UseShellExecute = true
        });
        if (process == null)
            throw new Exception($"Failed to start {fileName} plugin host, process is null");
        if (process.HasExited)
            throw new Exception($"Failed to start {fileName} plugin host, process has exited");
        _cts.Token.Register(() => process.Kill());

        //wait a moment for plugin host to start websocket server
        //if the websocket server didn't start within 500ms, then websocket client may failed to connect and will hence wait another 3 seconds to reconnect
        await Task.Delay(500, _cts.Token);

        Logger.Debug($"Nodejs plugin host started, pid: {process.Id}, port: {websocketServerPort}");

        await StartWebsocketServerAsync(websocketServerPort.Value);

        Logger.Debug("Nodejs plugin host connected");
    }

    private async Task StartWebsocketServerAsync(int websocketServerPort)
    {
        Logger.Debug($"Start websocket server on port {websocketServerPort}");

        _websocket = new WebsocketClient(new Uri($"ws://localhost:{websocketServerPort}"));
        var reconnectTimeout = TimeSpan.FromSeconds(5);
        var pingInterval = TimeSpan.FromSeconds(3);
        _websocket.ReconnectTimeout = reconnectTimeout;
        _websocket.ErrorReconnectTimeout = reconnectTimeout;
        _websocket.LostReconnectTimeout = reconnectTimeout;
        _websocket.ReconnectionHappened.Subscribe(info =>
        {
            Logger.Debug($"Reconnection happened, type: {info.Type}");

            //ping-pong 
            Task.Run(async () =>
            {
                while (!_cts.IsCancellationRequested && _websocket.IsRunning)
                {
                    await Task.Delay(pingInterval, _cts.Token);
                    await _websocket.SendInstant(Encoding.UTF8.GetBytes(JsonSerializer.Serialize(new PluginJsonRpcRequest
                    {
                        Method = "ping",
                        PluginId = "",
                        PluginName = "",
                        Type = PluginJsonRpcType.Request
                    })));
                }
            });
        });
        _websocket.MessageReceived.Subscribe(msg =>
            {
                var msgStr = msg.ToString();
                if (msgStr.Contains(PluginJsonRpcType.Request))
                    HandleRequestFromPlugin(msgStr);
                else if (msgStr.Contains(PluginJsonRpcType.Response))
                    HandleInvokeMethodResponse(msgStr);
                else
                    Logger.Error($"Invalid json rpc message type: {msgStr}");
            }
        );

        //try to connect, if websocket server is not ready, it will try to reconnect until success
        //TOOD: what if websocket server is not started at all for some reason? add a timeout?
        await _websocket.Start();
    }

    private void HandleInvokeMethodResponse(string msg)
    {
        PluginJsonRpcResponse? response;
        try
        {
            response = JsonSerializer.Deserialize<PluginJsonRpcResponse>(msg, new JsonSerializerOptions
            {
                PropertyNameCaseInsensitive = true
            });
            if (response == null)
            {
                Logger.Error($"Failed to deserialize json rpc response message {msg}");
                return;
            }
        }
        catch (Exception e)
        {
            Logger.Error($"Failed to deserialize json rpc response message {msg}", e);
            return;
        }

        if (response.Method == "ping")
            return;

        if (_invokeMethodTaskCompletes.TryGetValue(response.Id, out var tcs))
        {
            tcs.SetResult(response);
            _invokeMethodTaskCompletes.Remove(response.Id);
        }
        else
        {
            Logger.Error($"Failed to find task completion source for json rpc response {msg}");
        }
    }

    private void HandleRequestFromPlugin(string msg)
    {
        PluginJsonRpcRequest? request;
        try
        {
            request = JsonSerializer.Deserialize<PluginJsonRpcRequest>(msg, new JsonSerializerOptions
            {
                PropertyNameCaseInsensitive = true
            });
            if (request == null)
            {
                Logger.Error($"Failed to deserialize json rpc request message {msg}");
                return;
            }
        }
        catch (Exception e)
        {
            Logger.Error($"Failed to deserialize json rpc request message {msg}", e);
            return;
        }

        switch (request.Method)
        {
            case "HideApp":
                Logger.Info($"[{request.PluginName}] plugin request to ${request.Method}");
                break;
            case "ShowApp":
                Logger.Info($"[{request.PluginName}] plugin request to ${request.Method}");
                break;
            default:
                Logger.Error($"Invalid json rpc request method {request.Method}");
                break;
        }
    }

    public async Task InvokeMethod(PluginMetadata metadata, string method, Dictionary<string, string?>? parameters = default)
    {
        var request = new PluginJsonRpcRequest
        {
            Method = method,
            PluginId = metadata.Id,
            Type = PluginJsonRpcType.Request,
            PluginName = metadata.Name,
            Params = parameters ?? new Dictionary<string, string?>()
        };
        Logger.Debug($"[{request.PluginName}] invoke jsonrpc method {method}, request id: {request.Id}");

        Stopwatch sw = new();
        sw.Start();
        var tcs = new TaskCompletionSource<PluginJsonRpcResponse>();
        _invokeMethodTaskCompletes.Add(request.Id, tcs);

        await _websocket.SendInstant(Encoding.UTF8.GetBytes(JsonSerializer.Serialize(request)));
        var result = await tcs.Task;

        sw.Stop();
        Logger.Debug($"[{request.PluginName}] invoke jsonrpc method {method} finished, request id: {request.Id}, time elapsed: {sw.ElapsedMilliseconds}ms");
    }
}