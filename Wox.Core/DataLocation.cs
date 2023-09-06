namespace Wox.Core;

public static class DataLocation
{
    public static readonly string LogDirectory = Path.Combine(DataDirectory, "logs");
    private static readonly string UserInstalledPluginsDirectory = Path.Combine(DataDirectory, "plugins");
    private static readonly string SystemBundledPluginsDirectory = Path.Combine(AppContext.BaseDirectory, "plugins");

    /// <summary>
    ///     Places for storing plugins, for now we have two places:
    ///     1. user installed plugins
    ///     2. system bundled plugins, which is shipped with Wox
    /// </summary>
    public static readonly List<string> PluginDirectories = new()
    {
        UserInstalledPluginsDirectory,
        SystemBundledPluginsDirectory
    };

    /// <summary>
    ///     Places for storing plugins, configs and etc
    ///     We allow user to customize the data directory, so we need to store this customized location in a fixed-location file and read the real data directory from it
    /// </summary>
    private static string DataDirectory
    {
        get
        {
            var defaultDataDirectory = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "wox");
            if (!Directory.Exists(defaultDataDirectory)) Directory.CreateDirectory(defaultDataDirectory);
            var dataLocationFile = Path.Join(defaultDataDirectory, "location.txt");
            if (!File.Exists(dataLocationFile)) File.WriteAllText(dataLocationFile, defaultDataDirectory);

            var dataLocation = File.ReadAllText(dataLocationFile);
            return dataLocation == "" ? defaultDataDirectory : dataLocation;
        }
    }

    public static void EnsureDirectoryExist()
    {
        if (!Directory.Exists(DataDirectory)) Directory.CreateDirectory(DataDirectory);
        if (!Directory.Exists(UserInstalledPluginsDirectory)) Directory.CreateDirectory(UserInstalledPluginsDirectory);
        if (!Directory.Exists(LogDirectory)) Directory.CreateDirectory(LogDirectory);
    }
}