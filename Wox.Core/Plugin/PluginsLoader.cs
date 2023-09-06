using System.Reflection;
using System.Runtime.Loader;
using Newtonsoft.Json;
using Wox.Core.Utils;
using Wox.Plugin;

namespace Wox.Core.Plugin;

public static class PluginsLoader
{
    public static List<PluginPair> LoadPlugins()
    {
        Logger.Debug("Start to load plugins");
        var plugins = new List<PluginPair>();

        var pluginDirectories = DataLocation.PluginDirectories.SelectMany(Directory.GetDirectories);
        foreach (var pluginDirectory in pluginDirectories)
        {
            var metadata = ParsePluginMetadata(pluginDirectory);
            if (metadata == null) continue;

            IPlugin? pluginInstance = null;
            var startLoadPluginTime = DateTime.Now;
            if (metadata.Language!.ToUpper() == AllowedLanguage.CSharp) pluginInstance = LoadCSharpPlugin(metadata);
            if (pluginInstance == null) continue;

            metadata.InitTime = DateTime.Now.Subtract(startLoadPluginTime).Milliseconds;

            plugins.Add(new PluginPair
            {
                Metadata = metadata,
                Plugin = pluginInstance
            });
        }

        return plugins;
    }

    /// <summary>
    ///     Parse plugin metadata in giving directory
    /// </summary>
    private static PluginMetadata? ParsePluginMetadata(string pluginDirectory)
    {
        var configPath = Path.Combine(pluginDirectory, "plugin.json");
        if (!File.Exists(configPath))
        {
            Logger.Error($"Didn't find plugin config file {configPath}");
            return null;
        }

        PluginMetadata? metadata;
        try
        {
            metadata = JsonConvert.DeserializeObject<PluginMetadata>(File.ReadAllText(configPath));
            if (metadata == null)
            {
                Logger.Error($"Invalid json for plugin config {configPath}");
                return null;
            }
        }
        catch (Exception e)
        {
            Logger.Error($"Invalid json for plugin config {configPath}", e);
            return null;
        }

        //TODO: should not set this here
        metadata.PluginDirectory = pluginDirectory;

        if (metadata.ActionKeywords == null || metadata.ActionKeywords.Count == 0)
        {
            Logger.Error($"Plugin {metadata.Name} didn't register any action keyword");
            return null;
        }

        if (metadata.Language == null || !AllowedLanguage.IsAllowed(metadata.Language))
        {
            Logger.Error($"Invalid language {metadata.Language} for plugin config {configPath}");
            return null;
        }

        if (!File.Exists(metadata.ExecuteFilePath))
        {
            Logger.Error($"Execute file path ({metadata.ExecuteFilePath}) didn't exist for plugin config {configPath}");
            return null;
        }

        return metadata;
    }

    /// <summary>
    ///     TODO: Use unload-able assembly to load plugin, ref: https://learn.microsoft.com/zh-cn/dotnet/standard/assembly/unloadability
    /// </summary>
    private static IPlugin? LoadCSharpPlugin(PluginMetadata metadata)
    {
        IPlugin? plugin = null;
#if DEBUG
        var assembly = new AssemblyLoadContext(metadata.Name).LoadFromAssemblyName(AssemblyName.GetAssemblyName(metadata.ExecuteFilePath));
        var types = assembly.GetTypes();
        var type = types.First(o => o is { IsClass: true, IsAbstract: false } && o.GetInterfaces().Contains(typeof(IPlugin)));
        plugin = Activator.CreateInstance(type) as IPlugin;
#else
        Assembly assembly;
        try
        {
            assembly = Assembly.Load(AssemblyName.GetAssemblyName(metadata.ExecuteFilePath));
        }
        catch (Exception e)
        {
            e.Data.Add(nameof(metadata.ID), metadata.ID);
            e.Data.Add(nameof(metadata.Name), metadata.Name);
            e.Data.Add(nameof(metadata.PluginDirectory), metadata.PluginDirectory);
            e.Data.Add(nameof(metadata.Website), metadata.Website);
            Logger.Error($"Couldn't load assembly for {metadata.Name}", e);
            return null;
        }

        var types = assembly.GetTypes();
        Type type;
        try
        {
            type = types.First(o => o.IsClass && !o.IsAbstract && o.GetInterfaces().Contains(typeof(IPlugin)));
        }
        catch (InvalidOperationException e)
        {
            e.Data.Add(nameof(metadata.ID), metadata.ID);
            e.Data.Add(nameof(metadata.Name), metadata.Name);
            e.Data.Add(nameof(metadata.PluginDirectory), metadata.PluginDirectory);
            e.Data.Add(nameof(metadata.Website), metadata.Website);
            Logger.Error($"Can't find class implement IPlugin for {metadata.Name}", e);
            return null;
        }

        try
        {
            plugin = Activator.CreateInstance(type) as IPlugin;
        }
        catch (Exception e)
        {
            e.Data.Add(nameof(metadata.ID), metadata.ID);
            e.Data.Add(nameof(metadata.Name), metadata.Name);
            e.Data.Add(nameof(metadata.PluginDirectory), metadata.PluginDirectory);
            e.Data.Add(nameof(metadata.Website), metadata.Website);
            Logger.Error($"Can't create instance for {metadata.Name}", e);
        }
#endif
        return plugin;
    }
}