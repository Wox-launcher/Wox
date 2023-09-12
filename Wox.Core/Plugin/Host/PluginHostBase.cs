using System.Diagnostics;
using System.Text;
using System.Text.Json;
using WebSocketSharp;
using Wox.Core.Utils;
using Wox.Plugin;
using Logger = Wox.Core.Utils.Logger;

namespace Wox.Core.Plugin.Host;

public abstract class PluginHostBase : IPluginHost
{
    private readonly CancellationTokenSource _cts = new();
    private readonly Dictionary<string, TaskCompletionSource<PluginJsonRpcResponse>> _invokeMethodTaskCompletes = new();
    private WebSocket _ws = null!;

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

    public virtual async Task<IPlugin?> LoadPlugin(PluginMetadata metadata, string pluginDirectory)
    {
        await InvokeMethod(metadata, "loadPlugin", new Dictionary<string, string?>
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
        await Task.Delay(1000, _cts.Token);

        Logger.Debug($"Nodejs plugin host started, pid: {process.Id}, port: {websocketServerPort}");

        StartWebsocketServerAsync(websocketServerPort.Value);

        Logger.Debug("Nodejs plugin host connected");
    }

    private void StartWebsocketServerAsync(int websocketServerPort)
    {
        Logger.Debug($"Start websocket server on port {websocketServerPort}");

        _ws = new WebSocket($"ws://localhost:{websocketServerPort}");
        _ws.Log.Output = (data, s) => { Logger.Debug($"[{PluginRuntime}] Websocket server log: {data}"); };
        _ws.OnError += (sender, e) => { Logger.Error($"[{PluginRuntime}] Websocket server error: {e.Message}"); };
        _ws.OnClose += (sender, e) => { Logger.Debug($"[{PluginRuntime}] Websocket server closed"); };
        _ws.OnOpen += (sender, e) => { Logger.Debug($"[{PluginRuntime}] Websocket server connected"); };
        _ws.OnMessage += (sender, e) => { Task.Run(() => { OnReceiveWebsocketMessage(e.Data); }); };
        _ws.Connect();
    }

    private void OnReceiveWebsocketMessage(string msgStr)
    {
        try
        {
            Logger.Debug($"Received message: {msgStr}");
            if (msgStr.Contains(PluginJsonRpcType.Request))
                HandleRequestFromPlugin(msgStr);
            else if (msgStr.Contains(PluginJsonRpcType.Response))
                HandleInvokeMethodResponse(msgStr);
            else
                Logger.Error($"Invalid json rpc message type: {msgStr}");
        }
        catch (Exception exc)
        {
            Logger.Error($"Failed to handle websocket message {msgStr}", exc);
        }
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
                Logger.Info($"[{request.PluginName}] plugin request to {request.Method}");
                break;
            case "ShowApp":
                Logger.Info($"[{request.PluginName}] plugin request to {request.Method}");
                break;
            default:
                Logger.Error($"Invalid json rpc request method {request.Method}");
                break;
        }
    }

    public async Task<JsonElement?> InvokeMethod(PluginMetadata metadata, string method, Dictionary<string, string?>? parameters = default)
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

        _ws.Send(Encoding.UTF8.GetBytes(JsonSerializer.Serialize(request)));
        var result = await tcs.Task;

        sw.Stop();
        Logger.Debug($"[{request.PluginName}] invoke jsonrpc method {method} finished, request id: {request.Id}, time elapsed: {sw.ElapsedMilliseconds}ms");

        if (result.Error != null)
            throw new Exception($"[{request.PluginName}] invoke jsonrpc method {method} failed, error: {result.Error}");

        return result.Result;
    }
}