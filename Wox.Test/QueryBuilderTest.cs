using NUnit.Framework;
using Wox.Core;
using Wox.Core.Plugin;

namespace Wox.Test;

public class QueryBuilderTest
{
    private List<PluginInstance> GeneratePlugins(List<string> triggerKeywords, List<string> commands)
    {
        return new List<PluginInstance>
        {
            new()
            {
                Metadata = new PluginMetadata
                {
                    TriggerKeywords = triggerKeywords,
                    Commands = commands,
                    Id = Guid.NewGuid()
                        .ToString(),
                    Name = Guid.NewGuid()
                        .ToString(),
                    Author = Guid.NewGuid()
                        .ToString(),
                    Version = Guid.NewGuid()
                        .ToString(),
                    Runtime = PluginRuntime.Dotnet,
                    Description = Guid.NewGuid()
                        .ToString(),
                    Website = Guid.NewGuid()
                        .ToString(),
                    Entry = Guid.NewGuid()
                        .ToString(),
                    Icon = Guid.NewGuid()
                        .ToString(),
                    SupportedOS = new List<string>
                    {
                        PluginSupportedOS.Macos
                    },
                    MinWoxVersion = "2.0.0"
                },
                Plugin = null!,
                PluginDirectory = "",
                Host = null!,
                API = null!
            }
        };
    }

    [Test]
    public void NormalQueryTest()
    {
        var plugins = GeneratePlugins(new List<string>
            {
                "wpm",
                "p"
            },
            new List<string>
            {
                "install"
            });

        var q = QueryBuilder.BuildByPlugins("wpm install calculator", plugins);

        Assert.That(q!.TriggerKeyword, Is.EqualTo("wpm"));
        Assert.That(q.Command, Is.EqualTo("install"));
        Assert.That(q.Search, Is.EqualTo("calculator"));
    }

    [Test]
    public void NoCommandQueryTest()
    {
        var plugins = GeneratePlugins(new List<string>
            {
                "wpm",
                "p"
            },
            new List<string>
            {
                "install"
            });

        var q = QueryBuilder.BuildByPlugins("wpm   file.txt    file2 file3", plugins);

        Assert.That(q!.Search, Is.EqualTo("file.txt file2 file3"));
        Assert.That(q.TriggerKeyword, Is.EqualTo("wpm"));
    }

    [Test]
    public void OnlyTriggerTest()
    {
        var plugins = GeneratePlugins(new List<string>
            {
                "wpm",
                "p"
            },
            new List<string>
            {
                "install"
            });

        var q = QueryBuilder.BuildByPlugins("wpm ", plugins);

        Assert.That(q.Command, Is.Empty);
        Assert.That(q.TriggerKeyword, Is.EqualTo("wpm"));
        Assert.That(q.Search, Is.Empty);
    }

    [Test]
    public void OnlyTriggerWithoutSpaceTest()
    {
        var plugins = GeneratePlugins(new List<string>
            {
                "wpm",
                "p"
            },
            new List<string>
            {
                "install"
            });

        var q = QueryBuilder.BuildByPlugins("wpm", plugins);
        Assert.That(q.TriggerKeyword, Is.Empty);
        Assert.That(q.Search, Is.EqualTo("wpm"));
        Assert.That(q.Command, Is.Empty);
    }

    [Test]
    public void GlobalTriggerKeywordTest()
    {
        var plugins = GeneratePlugins(new List<string>
            {
                "*"
            },
            new List<string>());

        var q = QueryBuilder.BuildByPlugins("wpm file.txt    file2 file3", plugins);

        Assert.That(q!.TriggerKeyword, Is.Empty);
        Assert.That(q.Command, Is.Empty);
        Assert.That(q.Search, Is.EqualTo("wpm file.txt file2 file3"));
    }
}