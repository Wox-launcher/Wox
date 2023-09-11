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
        var reconnectTimeout = TimeSpan.FromSeconds(3);
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
                    await Task.Delay(1000, _cts.Token);
                    _websocket.Send("ping");
                }
            });
        });
        _websocket.MessageReceived.Subscribe(msg => Logger.Debug($"Message received: {msg}"));

        //try to connect, if websocket server is not ready, it will try to reconnect until success
        //TOOD: what if websocket server is not started at all for some reason? add a timeout?
        await _websocket.Start();
    }

    public async Task InvokeMethod(PluginMetadata metadata, string method, Dictionary<string, string?>? parameters = default)
    {
        parameters ??= new Dictionary<string, string?>();
        parameters["PluginId"] = metadata.Id;
        parameters["PluginName"] = metadata.Name;
        Logger.Debug($"Invoke method {method} for plugin {metadata.Name}");
        await _websocket.SendInstant(Encoding.UTF8.GetBytes(JsonSerializer.Serialize(new
        {
            method,
            parameters
        })));
        Logger.Debug($"Invoke method {method} finished for plugin {metadata.Name}");
    }
}