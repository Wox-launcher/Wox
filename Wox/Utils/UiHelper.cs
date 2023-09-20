using Avalonia;
using Avalonia.Controls.ApplicationLifetimes;
using Avalonia.Platform;

namespace Wox.Utils;

public static class UiHelper
{
    public static void ToggleWindowVisible(Screen? screen)
    {
        if (Application.Current != null && Application.Current.ApplicationLifetime != null)
        {
            var woxMainWindow = ((IClassicDesktopStyleApplicationLifetime)Application.Current.ApplicationLifetime)
                .MainWindow;
            if (woxMainWindow == null) return;
            if (screen != null)
            {
                var positionX = screen.Bounds.X + (screen.Bounds.Width - 800) / 2;
                woxMainWindow.Position = new PixelPoint(positionX, woxMainWindow.Position.Y);
            }
            woxMainWindow.IsVisible = !woxMainWindow.IsVisible;
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