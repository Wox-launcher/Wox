namespace Wox.Core.Plugin;

/// <summary>
///     The entry for managing Wox plugins
/// </summary>
public static class PluginManager
{
    static PluginManager()
    {
        EnsurePluginDirectory();
    }

    private static void EnsurePluginDirectory()
    {
        if (!Directory.Exists(Const.PluginsDirectory)) Directory.CreateDirectory(Const.PluginsDirectory);
    }

    /// <summary>
    ///     because InitializePlugins needs API, so LoadPlugins needs to be called first
    /// </summary>
    public static void LoadPlugins()
    {
    }
}