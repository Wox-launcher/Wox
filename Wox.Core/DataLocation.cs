namespace Wox.Core;

public class DataLocation
{
    private const string Wox = "wox";
    private const string Plugins = "plugins";

    /// <summary>
    ///     Places for storing plugins
    /// </summary>
    public static readonly string PluginsDirectory = Path.Combine(DataDirectory, Plugins);


    /// <summary>
    ///     Places for storing plugins, configs and etc
    ///     We allow user to customize the data directory, so we need to store this customized location in a fixed-location file and read the real data directory from it
    /// </summary>
    public static string DataDirectory
    {
        get
        {
            var defaultDataDirectory = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), Wox);
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
        if (!Directory.Exists(PluginsDirectory)) Directory.CreateDirectory(PluginsDirectory);
    }
}