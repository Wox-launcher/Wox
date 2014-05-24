namespace Wox.Plugins
{
    public interface ISystemPlugin : IPlugin
    {
        string Name { get; }
        string Description { get; }
    }
}
