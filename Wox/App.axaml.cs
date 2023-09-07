using Avalonia;
using Avalonia.Controls.ApplicationLifetimes;
using Avalonia.Markup.Xaml;
using Wox.Core;
using Wox.Core.Plugin;
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

        Bootstrap();
    }


    /// <summary>
    /// </summary>
    private void Bootstrap()
    {
        DataLocation.EnsureDirectoryExist();
        PluginManager.LoadPlugins(new PublicAPIInstance());
    }
}