using System;
using System.Threading.Tasks;
using Avalonia.Controls;
using Avalonia.Input;
using Wox.Core;
using Wox.Core.Plugin;
using Wox.Core.Utils;
using Wox.ViewModels;

namespace Wox.Views;

public partial class MainWindow : Window
{
    public MainWindow()
    {
        InitializeComponent();
        PointerPressed += MainWindow_PointerPressed;
    }

    private void MainWindow_PointerPressed(object? sender, PointerPressedEventArgs e)
    {
        if (e.Pointer.Type == PointerType.Mouse) BeginMoveDrag(e);
    }

    private void WoxMainWindow_OnDeactivated(object? sender, EventArgs e)
    {
        ((MainWindowViewModel)DataContext!).OnDeactivated();

        foreach (var pluginInstance in PluginManager.GetAllPlugins())
            Task.Run(async () =>
            {
                var results = await PluginManager.QueryForPlugin(pluginInstance, QueryBuilder.Build("wpm install calculator")!);
                foreach (var result in results)
                {
                    Logger.Info($"Plugin {pluginInstance.Metadata.Name} returned result {result.Result.Title}");
                    var actionResult = result.Result.Action != null && result.Result.Action();
                    Logger.Info($"Plugin {pluginInstance.Metadata.Name} returned action result {actionResult}");
                }
            });
    }
}