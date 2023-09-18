namespace Wox.Plugin;

/// <summary>
///     Every CSharp plugin should implement this interface
/// </summary>
public interface IPlugin
{
    /// <summary>
    ///     This method will be called when plugin init, you can do some heavy work here, it will not block the UI
    /// </summary>
    Task Init(PluginInitContext context);

    /// <summary>
    ///     This method will be called when query changed
    /// </summary>
    Task<List<Result>> Query(Query query);
}