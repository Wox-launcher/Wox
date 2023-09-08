namespace Wox.Core.Plugin;

/// <summary>
///     Supported plugin runtime
/// </summary>
public static class PluginRuntime
{
    public static string Python => "Python";

    public static string Dotnet => "Dotnet";

    public static string Nodejs => "Nodejs";

    public static bool IsAllowed(string runtime)
    {
        return runtime.ToUpper() == Python.ToUpper()
               || runtime.ToUpper() == Dotnet.ToUpper()
               || runtime.ToUpper() == Nodejs.ToUpper();
    }
}