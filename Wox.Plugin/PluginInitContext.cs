namespace Wox.Plugin;

public class PluginInitContext
{
    public PluginInitContext(IPublicAPI api)
    {
        API = api;
    }

    /// <summary>
    ///     Public APIs for plugin invocation
    /// </summary>
    public IPublicAPI API { get; }
}