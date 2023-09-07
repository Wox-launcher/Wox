using System.Reflection;
using System.Runtime.Loader;
using Wox.Plugin;

namespace Wox.Core.Plugin;

public class PluginAssemblyLoadContext : AssemblyLoadContext
{
    private readonly AssemblyDependencyResolver _resolver;

    public PluginAssemblyLoadContext(string pluginPath)
    {
        _resolver = new AssemblyDependencyResolver(pluginPath);
    }

    protected override Assembly? Load(AssemblyName assemblyName)
    {
        // because we isolate the plugin assembly load, 
        // Wox.Plugin.dll should be loaded as shared assembly, otherwise IPlugin type in main assembly and plugin assembly is not the same type
        if (assemblyName.FullName == typeof(IPlugin).Assembly.FullName)
            return Default.Assemblies.FirstOrDefault(x => x.FullName == assemblyName.FullName);

        var assemblyPath = _resolver.ResolveAssemblyToPath(assemblyName);
        return assemblyPath != null ? LoadFromAssemblyPath(assemblyPath) : null;
    }

    protected override IntPtr LoadUnmanagedDll(string unmanagedDllName)
    {
        var libraryPath = _resolver.ResolveUnmanagedDllToPath(unmanagedDllName);
        return libraryPath != null ? LoadUnmanagedDllFromPath(libraryPath) : IntPtr.Zero;
    }
}