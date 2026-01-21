using System.Windows;
using System;
using System.Threading.Tasks;
using Wox.UI.Windows.Services;

namespace Wox.UI.Windows;

public partial class App : Application
{
    private int _serverPort;
    private int _serverPid;
    private bool _isDev;
    private string _sessionId = Guid.NewGuid().ToString();

    protected override void OnStartup(StartupEventArgs e)
    {
        base.OnStartup(e);

        // 设置全局异常处理
        AppDomain.CurrentDomain.UnhandledException += (sender, args) =>
        {
            MessageBox.Show($"未处理的异常: {args.ExceptionObject}", "错误", MessageBoxButton.OK, MessageBoxImage.Error);
        };

        DispatcherUnhandledException += (sender, args) =>
        {
            MessageBox.Show($"UI 异常: {args.Exception.Message}", "错误", MessageBoxButton.OK, MessageBoxImage.Error);
            args.Handled = true;
        };

        try
        {
            // Check if running in test mode
            if (e.Args.Length > 0 && e.Args[0] == "--test")
            {
                // Launch test window instead of main window
                var testWindow = new TestWindow();
                testWindow.Show();
                return;
            }

            // Parse command-line arguments: <ServerPort> <ServerPid> <IsDev>
            if (e.Args.Length >= 3)
            {
                if (int.TryParse(e.Args[0], out var port))
                    _serverPort = port;

                if (int.TryParse(e.Args[1], out var pid))
                    _serverPid = pid;

                if (bool.TryParse(e.Args[2], out var isDev))
                    _isDev = isDev;
            }
            else
            {
                // For development/testing: use default values
                // Note: wox.core dynamically assigns port, check ~/.wox/log/log for actual port
                _serverPort = 34987; // Update this if wox.core uses a different port
                _serverPid = 0;
                _isDev = true;

                Logger.Log($"No command-line args, using default port: {_serverPort}");
            }

            // Show main window first
            var mainWindow = new MainWindow();
            mainWindow.Show();

            // Initialize services (async, non-blocking)
            Task.Run(async () =>
            {
                try
                {
                    var woxApiService = WoxApiService.Instance;
                    woxApiService.Initialize(_serverPort, _sessionId);

                    // Connect to WebSocket
                    await woxApiService.ConnectAsync();

                    await woxApiService.LoadThemeAsync();
                    await woxApiService.LoadSettingAsync();

                    // Notify server that UI is ready
                    await woxApiService.NotifyUIReadyAsync();
                }
                catch (Exception ex)
                {
                    // 静默失败，UI 仍可用（用于测试）
                    Logger.Error("连接到服务器失败", ex);
                }
            });
        }
        catch (Exception ex)
        {
            MessageBox.Show($"启动失败: {ex.Message}\n\n{ex.StackTrace}", "错误", MessageBoxButton.OK, MessageBoxImage.Error);
            Logger.Error("启动失败", ex);
            Shutdown();
        }
    }

    protected override void OnExit(ExitEventArgs e)
    {
        WoxApiService.Instance.Dispose();
        base.OnExit(e);
    }
}
