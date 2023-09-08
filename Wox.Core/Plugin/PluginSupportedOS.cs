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
}