using Wox.Plugin;

namespace Wox.Core.Plugin.System;

public interface ISystemPlugin : IPlugin
{
    public PluginMetadata GetMetadata();
}