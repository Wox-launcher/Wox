using System.Diagnostics;
using System.Text;
using System.Text.Json;
using WebSocketSharper;
using Wox.Core.Utils;
using Wox.Plugin;

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

    public virtual async Task UnloadPlugin(PluginMetadata metadata)
    {
        await InvokeMethod(metadata, "unloadPlugin", new Dictionary<string, string?>
        {
            { "PluginId", metadata.Id }
        });
    }

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
        if (!File.Exists(entry))
            throw new Exception($"Failed to start {fileName} plugin host, entry file {entry} doesn't exist");

        var startInfo = new ProcessStartInfo
        {
            FileName = fileName,
            Arguments = $"{entry} {websocketServerPort} \"{DataLocation.LogHostsDirectory}\"",
            CreateNoWindow = true,
            UseShellExecute = false,
            RedirectStandardOutput = true,
            RedirectStandardError = true
        };
        var process = new Process();
        process.OutputDataReceived += (sender, e) =>
        {
            if (e.Data != null)
                Logger.Debug($"[{PluginRuntime} host] console output => {e.Data}");
        };
        process.ErrorDataReceived += (sender, e) =>
        {
            if (e.Data != null)
                Logger.Error($"[{PluginRuntime} host] console error => {e.Data}");
        };
        process.StartInfo = startInfo;
        var started = process.Start();
        process.BeginOutputReadLine();
        if (!started)
            throw new Exception($"Failed to start {fileName} plugin host, not started");
        if (process.HasExited)
            throw new Exception($"Failed to start {fileName} plugin host, process has exited");

        _cts.Token.Register(() =>
        {
            Logger.Debug($"[{PluginRuntime} host] host process killed by user");
            process.Kill();
        });

        //wait a moment for plugin host to start websocket server
        await Task.Delay(1000, _cts.Token);

        Logger.Debug($"[{PluginRuntime} host] host process started, pid: {process.Id}, websocket port: {websocketServerPort}");

        await StartWebsocketServerAsync(websocketServerPort.Value);

        Logger.Debug($"[{PluginRuntime} host] host connected");
    }

    private async Task StartWebsocketServerAsync(int websocketServerPort)
    {
        var tcs = new TaskCompletionSource<bool>();

        _ws = new WebSocket(Logger.GetMicrosoftILogger(), $"ws://localhost:{websocketServerPort}", true);
        _ws.OnError += (sender, e) => { Logger.Error($"[{PluginRuntime} host] websocket server error: {e.Message}"); };
        // var retryDelay = 500;
        var retryCts = new CancellationTokenSource();
        _ws.OnClose += (sender, e) =>
        {
            Logger.Debug($"[{PluginRuntime} host] websocket connection closed");
            //
            // //try to reconnect
            // if (!_cts.IsCancellationRequested && !retryCts.IsCancellationRequested)
            //     Task.Run(async () =>
            //     {
            //         retryDelay *= 2;
            //         Logger.Debug($"[{PluginRuntime} host] websocket reconnecting in {retryDelay / 1000} second");
            //         await Task.Delay(retryDelay);
            //         Logger.Debug($"[{PluginRuntime} host] websocket reconnecting");
            //         // ReSharper disable once MethodHasAsyncOverload
            //         _ws.Connect();
            //     }, retryCts.Token);
            // else
            //     Logger.Debug($"[{PluginRuntime} host] websocket reconnecting cancelled");
        };
        _ws.OnOpen += (sender, e) =>
        {
            Logger.Debug($"[{PluginRuntime} host] websocket connected");
            tcs.TrySetResult(true);

            //send ping every seconds
            Task.Run(async () =>
            {
                while (!_cts.IsCancellationRequested && !retryCts.IsCancellationRequested)
                {
                    await Task.Delay(3000, _cts.Token);
                    _ws.Ping();
                }
            }, _cts.Token);
        };
        _ws.OnMessage += (sender, e) => { Task.Run(() => { OnReceiveWebsocketMessage(e.Data); }); };
        // ReSharper disable once MethodHasAsyncOverload
        _ws.Connect();

        //wait to connect success
        var timeout = Task.Delay(3000);
        var result = await Task.WhenAny(tcs.Task, timeout);
        if (result == timeout)
        {
            Logger.Warn($"[{PluginRuntime} host] failed to connect to websocket server, try to reconnect");
            // ReSharper disable once MethodHasAsyncOverload
            // first timeout, maybe host is starting, try to connect again
            _ws.Connect();

            var timeoutAgain = Task.Delay(3000);
            var resultAgain = await Task.WhenAny(tcs.Task, timeoutAgain);
            if (resultAgain == timeoutAgain)
            {
                // still timeout, throw exception
                retryCts.Cancel();
                Logger.Warn($"[{PluginRuntime} host] still failed to connect to websocket server, cancel retry");
                throw new Exception($"[{PluginRuntime} host] failed to connect to websocket server");
            }
        }
    }

    private void OnReceiveWebsocketMessage(string msgStr)
    {
        try
        {
            //Logger.Debug($"Received message: {msgStr}");
            if (msgStr.Contains(PluginJsonRpcType.Request))
                HandleRequestFromPlugin(msgStr);
            else if (msgStr.Contains(PluginJsonRpcType.Response))
                HandleResponseFromPlugin(msgStr);
            else
                Logger.Error($"Invalid json rpc message type: {msgStr}");
        }
        catch (Exception exc)
        {
            Logger.Error($"Failed to handle websocket message {msgStr}", exc);
        }
    }

    private void HandleResponseFromPlugin(string msg)
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

        var pluginInstance = PluginManager.GetAllPlugins().FirstOrDefault(o => o.Metadata.Id == request.PluginId);
        if (pluginInstance == null)
        {
            Logger.Error($"[{request.PluginName}] Failed to find plugin instance for request {msg}");
            return;
        }

        Logger.Debug($"[{request.PluginName}] plugin request to {request.Method}");
        switch (request.Method)
        {
            case "HideApp":
                pluginInstance.API.HideApp();
                _ws.Send(Encoding.UTF8.GetBytes(JsonSerializer.Serialize(new PluginJsonRpcResponse
                {
                    Id = request.Id,
                    Method = request.Method,
                    Type = PluginJsonRpcType.Response
                })));
                break;
            case "ShowApp":
                pluginInstance.API.ShowApp();
                _ws.Send(Encoding.UTF8.GetBytes(JsonSerializer.Serialize(new PluginJsonRpcResponse
                {
                    Id = request.Id,
                    Method = request.Method,
                    Type = PluginJsonRpcType.Response
                })));
                break;
            case "ChangeQuery":
                if (request.Params == null)
                {
                    Logger.Error($"[{request.PluginName}] ChangeQuery method must have a parameter");
                    return;
                }

                request.Params.TryGetValue("query", out var query);
                if (query == null)
                {
                    Logger.Error($"[{request.PluginName}] ChangeQuery method must have a query parameter");
                    return;
                }

                pluginInstance.API.ChangeQuery(query);
                _ws.Send(Encoding.UTF8.GetBytes(JsonSerializer.Serialize(new PluginJsonRpcResponse
                {
                    Id = request.Id,
                    Method = request.Method,
                    Type = PluginJsonRpcType.Response
                })));
                break;
            case "ShowMsg":
                if (request.Params == null)
                {
                    Logger.Error($"[{request.PluginName}] ShowMsg method must have a parameter");
                    return;
                }

                request.Params.TryGetValue("title", out var title);
                if (title == null)
                {
                    Logger.Error($"[{request.PluginName}] ShowMsg method must have a title parameter");
                    return;
                }

                request.Params.TryGetValue("description", out var description);
                request.Params.TryGetValue("iconPath", out var iconPath);
                pluginInstance.API.ShowMsg(title, description ?? "", iconPath ?? "");
                _ws.Send(Encoding.UTF8.GetBytes(JsonSerializer.Serialize(new PluginJsonRpcResponse
                {
                    Id = request.Id,
                    Method = request.Method,
                    Type = PluginJsonRpcType.Response
                })));
                break;
            case "Log":
                if (request.Params == null)
                {
                    Logger.Error($"[{request.PluginName}] Log method must have a parameter");
                    return;
                }

                request.Params.TryGetValue("msg", out var msgStr);
                if (msgStr == null)
                {
                    Logger.Error($"[{request.PluginName}] Log method must have a msg parameter");
                    return;
                }

                pluginInstance.API.Log(msgStr);

                _ws.Send(Encoding.UTF8.GetBytes(JsonSerializer.Serialize(new PluginJsonRpcResponse
                {
                    Id = request.Id,
                    Method = request.Method,
                    Type = PluginJsonRpcType.Response
                })));
                break;
            case "GetTranslation":
                if (request.Params == null)
                {
                    Logger.Error($"[{request.PluginName}] Log method must have a parameter");
                    return;
                }

                request.Params.TryGetValue("key", out var key);
                if (key == null)
                {
                    Logger.Error($"[{request.PluginName}] Log method must have a key parameter");
                    return;
                }

                var translation = pluginInstance.API.GetTranslation(key);
                var response = new PluginJsonRpcResponse
                {
                    Id = request.Id,
                    Method = request.Method,
                    Type = PluginJsonRpcType.Response,
                    Result = JsonSerializer.SerializeToElement(translation)
                };
                _ws.Send(Encoding.UTF8.GetBytes(JsonSerializer.Serialize(response)));
                break;
        }
    }

    public async Task<JsonElement?> InvokeMethod(PluginMetadata metadata, string method, Dictionary<string, string?>? parameters = default)
    {
        var request = new PluginJsonRpcRequest
        {
            Id = Guid.NewGuid().ToString(),
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

        //TODO: what if server not response? add timeout
        _ws.Send(Encoding.UTF8.GetBytes(JsonSerializer.Serialize(request)));
        var result = await tcs.Task;

        sw.Stop();
        Logger.Debug($"[{request.PluginName}] invoke jsonrpc method {method} finished, request id: {request.Id}, time elapsed: {sw.ElapsedMilliseconds}ms");

        if (result.Error != null)
            throw new Exception($"[{request.PluginName}] invoke jsonrpc method {method} failed, error: {result.Error}");

        return result.Result;
    }
}