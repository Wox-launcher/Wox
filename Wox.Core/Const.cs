namespace Wox.Core;

public class Const
{
    private const string Wox = "Wox";
    private const string Plugins = "Plugins";

    /// <summary>
    ///     Places for storing plugins, configs and etc
    /// </summary>
    public static readonly string DataDirectory = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), Wox);

    /// <summary>
    ///     Places for storing plugins
    /// </summary>
    public static readonly string PluginsDirectory = Path.Combine(DataDirectory, Plugins);
}