namespace Wox.Plugin;

public class PluginInitContext
{
    public PluginInitContext(PluginMetadata pluginMetadata, IPublicAPI api)
    {
        PluginMetadata = pluginMetadata;
        API = api;
    }

    /// <summary>
    ///     Plugin metadata
    /// </summary>
    public PluginMetadata PluginMetadata { get; }

    /// <summary>
    ///     Public APIs for plugin invocation
    /// </summary>
    public IPublicAPI API { get; }
}