using Wox.Core;

namespace Wox.ViewModels;

public class MainWindowViewModel : ViewModelBase
{
    public string Greeting => DataLocation.PluginDirectories[0];
}