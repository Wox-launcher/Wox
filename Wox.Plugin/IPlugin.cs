namespace Wox.Plugin;

/// <summary>
///     Every CSharp plugin should implement this interface
/// </summary>
public interface IPlugin
{
    /// <summary>
    ///     This method will be called when plugin init, you can do some heavy work here, it will not block the UI
    /// </summary>
    void Init(PluginInitContext context);

    /// <summary>
    ///     This method will be called when query changed
    /// </summary>
    List<Result> Query(Query query);
}