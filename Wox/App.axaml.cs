using System;
using System.Threading.Tasks;
using Avalonia;
using Avalonia.Controls.ApplicationLifetimes;
using Avalonia.Markup.Xaml;
using Wox.Core;
using Wox.Core.I18n;
using Wox.Core.Plugin;
using Wox.Core.Utils;
using Wox.ViewModels;
using Wox.Views;

namespace Wox;

public class App : Application
{
    public override void Initialize()
    {
        AvaloniaXamlLoader.Load(this);
    }

    public override void OnFrameworkInitializationCompleted()
    {
        if (ApplicationLifetime is IClassicDesktopStyleApplicationLifetime desktop)
            desktop.MainWindow = new MainWindow
            {
                DataContext = new MainWindowViewModel()
            };

        base.OnFrameworkInitializationCompleted();

        Task.Run(async () => { await Bootstrap(); });
    }


    /// <summary>
    /// </summary>
    private async Task Bootstrap()
    {
        DataLocation.EnsureDirectoryExist();
        Logger.Info("---------------------------");
        Logger.Info("Bootstrap Wox");
        Logger.Info($"CLR version: {Environment.Version}");
        Logger.Info("Data location: ");
        Logger.Info($"  - Log: {DataLocation.LogDirectory}");
        Logger.Info("  - Hosts:");
        Logger.Info($"    - Node: {DataLocation.NodejsHostEntry}");
        Logger.Info($"    - Python: {DataLocation.PythonHostEntry}");
        Logger.Info("  - Plugins: ");
        foreach (var pluginDirectory in DataLocation.PluginDirectories) Logger.Info($"    - {pluginDirectory}");

#if DEBUG
        Logger.Info("Run Mode: Debug");
#else
        Logger.Info("Run Mode: Release");
#endif

        await PluginManager.Load();
        await I18NManager.Load();

        Logger.Info("Finish bootstrap");
    }

    private void ToggleWindowState()
    {
        if (Current?.ApplicationLifetime is IClassicDesktopStyleApplicationLifetime desktop)
            if (desktop.MainWindow is MainWindow mainWindow)
                mainWindow.IsVisible = !mainWindow.IsVisible;
    }

    private void TrayIcon_OnClicked(object? sender, EventArgs e)
    {
        ToggleWindowState();
    }
}