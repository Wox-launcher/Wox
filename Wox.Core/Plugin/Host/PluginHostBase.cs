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
    private readonly TaskCompletionSource _tcs = new();
    private WebsocketClient websocket = null!;

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

        Logger.Debug($"Nodejs plugin host started, pid: {process.Id}, port: {websocketServerPort}");

        StartWebsocketServerAsync(websocketServerPort.Value);

        Logger.Debug("Nodejs plugin host connected");
    }

    private void StartWebsocketServerAsync(int websocketServerPort)
    {
        Logger.Debug($"Start websocket server on port {websocketServerPort}");

        var exitEvent = new ManualResetEvent(false);
        websocket = new WebsocketClient(new Uri($"ws://localhost:{websocketServerPort}"));
        websocket.ReconnectTimeout = TimeSpan.FromSeconds(30);
        websocket.ReconnectionHappened.Subscribe(info =>
        {
            if (info.Type == ReconnectionType.Initial) exitEvent.Set();
            Logger.Debug($"Reconnection happened, type: {info.Type}");

            //ping-pong 
            Task.Run(async () =>
            {
                while (!_cts.IsCancellationRequested && websocket.IsRunning)
                {
                    await Task.Delay(3000, _cts.Token);
                    websocket.Send("ping");
                }
            });
        });
        websocket.MessageReceived.Subscribe(msg => Logger.Debug($"Message received: {msg}"));
        websocket.Start();
        exitEvent.WaitOne();
    }

    public async Task InvokeMethod(PluginMetadata metadata, string method, Dictionary<string, string?>? parameters = default)
    {
        parameters ??= new Dictionary<string, string?>();
        parameters["PluginId"] = metadata.Id;
        Logger.Debug($"Invoke method {method} for plugin {metadata.Name}");
        await websocket.SendInstant(Encoding.UTF8.GetBytes(JsonSerializer.Serialize(new
        {
            method,
            parameters
        })));
        Logger.Debug($"Invoke method {method} finished for plugin {metadata.Name}");
    }
}