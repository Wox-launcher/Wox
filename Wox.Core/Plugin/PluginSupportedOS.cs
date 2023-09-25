namespace Wox.Core.Plugin;

public static class PluginSupportedOS
{
    public static string Macos => "Macos";

    public static string Windows => "Windows";

    public static string Linux => "Linux";

    public static bool IsAllowed(string os)
    {
        return os.ToUpper() == Macos.ToUpper()
               || os.ToUpper() == Windows.ToUpper()
               || os.ToUpper() == Linux.ToUpper();
    }

    public static string GetCurrentOS()
    {
        if (OperatingSystem.IsMacOS())
            return "Macos";
        if (OperatingSystem.IsWindows())
            return "Windows";
        if (OperatingSystem.IsLinux())
            return "Linux";

        return "Unknown";
    }
}