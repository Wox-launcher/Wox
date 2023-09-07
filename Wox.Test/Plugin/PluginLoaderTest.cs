using NUnit.Framework;
using Wox.Core.Plugin;
using Wox.Plugin;

namespace Wox.Test.Plugin;

public class PluginLoaderTest
{
    [Test]
    public void NormalPluginParseTest()
    {
        var json = File.ReadAllText(Path.Combine(Directory.GetCurrentDirectory(), "../../../Plugins/Wox.Plugin.Calculator/plugin.json"));
        var pluginMetadata = PluginLoader.ParsePluginMetadata(json);

        Assert.That(pluginMetadata, Is.Not.Null);
        Assert.That(pluginMetadata!.Name, Is.EqualTo("Calculator"));
        Assert.That(pluginMetadata.Language.ToUpper(), Is.EqualTo(AllowedLanguage.CSharp));
    }
}