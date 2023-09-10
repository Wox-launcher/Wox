namespace Wox.Core.Plugin;

/// <summary>
///     Supported plugin runtime
/// </summary>
public static class PluginRuntime
{
    public static string Python => "Python";

    public static string Dotnet => "Dotnet";

    public static string Nodejs => "Nodejs";

    public static List<string> All => new() { Python, Dotnet, Nodejs };

    public static bool IsAllowed(string runtime)
    {
        return All.Select(o => o.ToUpper()).Contains(runtime.ToUpper());
    }
}