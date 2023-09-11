using Avalonia;
using Avalonia.Controls.ApplicationLifetimes;
using Wox.Core;
using Wox.Core.Plugin;

namespace Wox.ViewModels;

public class MainWindowViewModel : ViewModelBase
{
    public string Greeting => DataLocation.PluginDirectories[0];

    public void OnDeactivated()
    {
        if (Application.Current != null && Application.Current.ApplicationLifetime != null)
        {
            var woxMainWindow = ((IClassicDesktopStyleApplicationLifetime)Application.Current.ApplicationLifetime)
                .MainWindow;
            if (woxMainWindow != null)
            {
                woxMainWindow.Hide();
                // foreach (var pluginInstance in PluginManager.GetAllPlugins())
                //     PluginManager.QueryForPlugin(pluginInstance, QueryBuilder.Build("wpm install x")!); 
            }
                
        }
    }
}