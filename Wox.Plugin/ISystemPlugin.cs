namespace Wox.Plugin.SystemPlugins
{
    public interface ISystemPlugin : IPlugin
    {
        string Name { get; }
        string Description { get; }
    }
}
