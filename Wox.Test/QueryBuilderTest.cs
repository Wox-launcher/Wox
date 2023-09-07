using NUnit.Framework;
using Wox.Core;
using Wox.Plugin;

namespace Wox.Test;

public class QueryBuilderTest
{
    [Test]
    public void NormalQueryTest()
    {
        var plugins = new Dictionary<string, PluginMetadata>
        {
            { "wpm", new PluginMetadata { TriggerKeywords = new List<string> { "wpm", "p" }, Commands = new List<string> { "install" } } }
        };

        var q = QueryBuilder.Build("wpm install calculator", plugins);

        Assert.That(q, Is.Not.Null);
        Assert.That(q!.TriggerKeyword, Is.EqualTo("wpm"));
        Assert.That(q.Command, Is.EqualTo("install"));
        Assert.That(q.Search, Is.EqualTo("calculator"));
    }

    [Test]
    public void NoCommandQueryTest()
    {
        var plugins = new Dictionary<string, PluginMetadata>
        {
            { "wpm", new PluginMetadata { TriggerKeywords = new List<string> { "wpm", "p" }, Commands = new List<string> { "install" } } }
        };

        var q = QueryBuilder.Build("wpm   file.txt    file2 file3", plugins);

        Assert.That(q, Is.Not.Null);
        Assert.That(q!.Search, Is.EqualTo("file.txt file2 file3"));
        Assert.That(q.TriggerKeyword, Is.EqualTo("wpm"));
    }

    [Test]
    public void GlobalTriggerKeywordTest()
    {
        var plugins = new Dictionary<string, PluginMetadata>
        {
            { "*", new PluginMetadata { TriggerKeywords = new List<string> { "*" } } }
        };

        var q = QueryBuilder.Build("wpm file.txt    file2 file3", plugins);

        Assert.That(q, Is.Not.Null);
        Assert.That(q!.TriggerKeyword, Is.Empty);
        Assert.That(q.Command, Is.Empty);
        Assert.That(q.Search, Is.EqualTo("wpm file.txt file2 file3"));
    }
}