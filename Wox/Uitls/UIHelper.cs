using Avalonia;
using Avalonia.Controls.ApplicationLifetimes;

namespace Wox.Uitls;

public class UIHelper
{
    public static void ToggleWindowVisible()
    {
        if (Application.Current != null && Application.Current.ApplicationLifetime != null)
        {
            var woxMainWindow = ((IClassicDesktopStyleApplicationLifetime)Application.Current.ApplicationLifetime)
                .MainWindow;
            if (woxMainWindow != null) woxMainWindow.IsVisible = !woxMainWindow.IsVisible;
        }
    }

    public static void HideWindow()
    {
        if (Application.Current != null && Application.Current.ApplicationLifetime != null)
        {
            var woxMainWindow = ((IClassicDesktopStyleApplicationLifetime)Application.Current.ApplicationLifetime)
                .MainWindow;
            if (woxMainWindow != null) woxMainWindow.Hide();
        }
    }
}