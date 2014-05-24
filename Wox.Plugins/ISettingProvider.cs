using System.Windows.Controls;

namespace Wox.Plugins
{
    public interface ISettingProvider
    {
        Control CreateSettingPanel();
    }
}
