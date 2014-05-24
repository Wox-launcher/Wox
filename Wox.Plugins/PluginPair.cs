namespace Wox.Plugins
{
    public class PluginPair
    {
        public IPlugin Plugin { get; set; }
        public PluginMetadata Metadata { get; set; }
        public PluginInitContext InitContext { get; set; }
    }
}
