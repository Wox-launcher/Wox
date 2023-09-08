using Wox.Plugin;

namespace Wox.Core.Plugin;

public class PluginQueryResult
{
    public required Result Result { get; init; }

    /// <summary>
    ///     Associated query that generated this result.
    /// </summary>
    public required Query AssociatedQuery { get; init; }

    /// <summary>
    ///     Plugin that generated this result.
    /// </summary>
    public required PluginInstance Plugin { get; init; }

    public string IconPath
    {
        get
        {
            if (!string.IsNullOrEmpty(Plugin.PluginDirectory) && !string.IsNullOrEmpty(Result.IcoPath) && !Path.IsPathRooted(Result.IcoPath))
                return Path.Combine(Plugin.PluginDirectory, Result.IcoPath);

            return "";
        }
    }
}