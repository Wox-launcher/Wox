using System.Runtime.InteropServices;
using System.Windows;
using System.Windows.Interop;

namespace Wox.UI.Windows.Services;

public static class WindowBackdropService
{
    private const int DwmwaUseImmersiveDarkMode = 20;
    private const int DwmwaUseImmersiveDarkModeBefore20H1 = 19;
    private const int DwmwaSystemBackdropType = 38;
    private const int DwmwaMicaEffect = 1029;

    private const int DwmSystemBackdropTypeMica = 2;

    [StructLayout(LayoutKind.Sequential)]
    private struct Margins
    {
        public int CxLeftWidth;
        public int CxRightWidth;
        public int CyTopHeight;
        public int CyBottomHeight;
    }

    [DllImport("dwmapi.dll")]
    private static extern int DwmSetWindowAttribute(IntPtr hwnd, int attr, ref int attrValue, int attrSize);

    [DllImport("dwmapi.dll")]
    private static extern int DwmExtendFrameIntoClientArea(IntPtr hwnd, ref Margins margins);

    public static void ApplyMica(Window window, bool useDarkMode)
    {
        var hwnd = new WindowInteropHelper(window).Handle;
        if (hwnd == IntPtr.Zero)
        {
            return;
        }

        var margins = new Margins
        {
            CxLeftWidth = -1,
            CxRightWidth = -1,
            CyTopHeight = -1,
            CyBottomHeight = -1,
        };
        _ = DwmExtendFrameIntoClientArea(hwnd, ref margins);

        var darkMode = useDarkMode ? 1 : 0;
        if (DwmSetWindowAttribute(hwnd, DwmwaUseImmersiveDarkMode, ref darkMode, sizeof(int)) != 0)
        {
            _ = DwmSetWindowAttribute(hwnd, DwmwaUseImmersiveDarkModeBefore20H1, ref darkMode, sizeof(int));
        }

        var mica = DwmSystemBackdropTypeMica;
        if (DwmSetWindowAttribute(hwnd, DwmwaSystemBackdropType, ref mica, sizeof(int)) != 0)
        {
            var enable = 1;
            _ = DwmSetWindowAttribute(hwnd, DwmwaMicaEffect, ref enable, sizeof(int));
        }
    }
}
