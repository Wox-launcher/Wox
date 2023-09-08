using System.Reflection;
using System.Text.Json;
using Wox.Core.Utils;
using Wox.Plugin;

namespace Wox.Core.Plugin;

public static class PluginLoader
{
    public static List<PluginInstance> LoadPlugins()
    {
        Logger.Debug("Start to load plugins");
        var pluginInstances = new List<PluginInstance>();

        var pluginDirectories = DataLocation.PluginDirectories.SelectMany(Directory.GetDirectories);
        foreach (var pluginDirectory in pluginDirectories)
        {
            var configPath = Path.Combine(pluginDirectory, "plugin.json");
            if (!File.Exists(configPath))
            {
                Logger.Error($"Didn't find plugin config file {configPath}");
                continue;
            }

            var metadata = ParsePluginMetadataFromDirectory(pluginDirectory);
            if (metadata == null) continue;


            IPlugin? plugin = null;
            PluginAssemblyLoadContext? assemblyLoadContext = null;
            // var startLoadPluginTime = DateTime.Now;
            if (metadata.Runtime.ToUpper() == PluginRuntime.Dotnet) (plugin, assemblyLoadContext) = LoadCSharpPlugin(metadata, pluginDirectory);

            if (plugin == null) continue;

            // metadata.InitTime = DateTime.Now.Subtract(startLoadPluginTime).Milliseconds;
            pluginInstances.Add(new PluginInstance
            {
                Metadata = metadata,
                Plugin = plugin,
                AssemblyLoadContext = assemblyLoadContext,
                PluginDirectory = pluginDirectory
            });
        }

        return pluginInstances;
    }

    /// <summary>
    ///     Parse plugin metadata in giving directory
    /// </summary>
    private static PluginMetadata? ParsePluginMetadataFromDirectory(string pluginDirectory)
    {
        Logger.Debug($"Start to parse plugin in {pluginDirectory}");
        var configPath = Path.Combine(pluginDirectory, "plugin.json");
        if (!File.Exists(configPath))
        {
            Logger.Error($"Didn't find plugin config file {configPath}");
            return null;
        }

        try
        {
            var pluginJson = File.ReadAllText(configPath);
            return ParsePluginMetadata(pluginJson);
        }
        catch (Exception e)
        {
            Logger.Error($"Read plugin.json failed {configPath}", e);
            return null;
        }
    }

    public static PluginMetadata? ParsePluginMetadata(string pluginJson)
    {
        PluginMetadata? metadata;
        try
        {
            metadata = JsonSerializer.Deserialize<PluginMetadata>(pluginJson, new JsonSerializerOptions
            {
                PropertyNameCaseInsensitive = true
            });
            if (metadata == null)
            {
                Logger.Error($"Invalid json for plugin config {pluginJson}");
                return null;
            }
        }
        catch (Exception e)
        {
            Logger.Error($"Deserialize plugin config failed {pluginJson}", e);
            return null;
        }

        if (metadata.TriggerKeywords.Count == 0)
        {
            Logger.Error($"Plugin {metadata.Name} didn't register any trigger keyword");
            return null;
        }

        if (!PluginRuntime.IsAllowed(metadata.Runtime))
        {
            Logger.Error($"Invalid language {metadata.Runtime} for plugin config");
            return null;
        }

        return metadata;
    }

    private static (IPlugin?, PluginAssemblyLoadContext?) LoadCSharpPlugin(PluginMetadata metadata, string pluginDirectory)
    {
        Logger.Debug($"Start to load csharp plugin {metadata.Name}");
        try
        {
            var executeFilePath = Path.Combine(pluginDirectory, metadata.EntryFile);
            var pluginLoadContext = new PluginAssemblyLoadContext(executeFilePath);
            var assembly = pluginLoadContext.LoadFromAssemblyName(new AssemblyName(Path.GetFileNameWithoutExtension(executeFilePath)));
            var type = assembly.GetTypes().FirstOrDefault(o => typeof(IPlugin).IsAssignableFrom(o));
            if (type == null) return (null, null);
            return (Activator.CreateInstance(type) as IPlugin, pluginLoadContext);
        }
        catch (Exception e)
        {
            Logger.Error($"Couldn't load assembly for csharp plugin {metadata.Name}", e);
#if DEBUG
            throw;
#else
            return (null, null);
#endif
        }
    }
}