using NUnit.Framework;
using Wox.Core;
using Wox.Core.Plugin;

namespace Wox.Test;

public class QueryBuilderTest
{
    private Dictionary<string, PluginInstance> GeneratePlugins(List<string> triggerKeywords, List<string> commands)
    {
        return new Dictionary<string, PluginInstance>
        {
            {
                triggerKeywords[0], new PluginInstance
                {
                    Metadata = new PluginMetadata
                    {
                        TriggerKeywords = triggerKeywords,
                        Commands = commands,
                        Id = Guid.NewGuid().ToString(),
                        Name = Guid.NewGuid().ToString(),
                        Author = Guid.NewGuid().ToString(),
                        Version = Guid.NewGuid().ToString(),
                        Runtime = PluginRuntime.Dotnet,
                        Description = Guid.NewGuid().ToString(),
                        Website = Guid.NewGuid().ToString(),
                        EntryFile = Guid.NewGuid().ToString(),
                        Icon = Guid.NewGuid().ToString(),
                        SupportedOS = new List<string>
                        {
                            PluginSupportedOS.Macos
                        }
                    },
                    Plugin = null!,
                    PluginDirectory = ""
                }
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

        var q = QueryBuilder.Build("wpm install calculator", plugins);

        Assert.That(q, Is.Not.Null);
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

        var q = QueryBuilder.Build("wpm   file.txt    file2 file3", plugins);

        Assert.That(q, Is.Not.Null);
        Assert.That(q!.Search, Is.EqualTo("file.txt file2 file3"));
        Assert.That(q.TriggerKeyword, Is.EqualTo("wpm"));
    }

    [Test]
    public void GlobalTriggerKeywordTest()
    {
        var plugins = GeneratePlugins(new List<string>
            {
                "*"
            },
            new List<string>());

        var q = QueryBuilder.Build("wpm file.txt    file2 file3", plugins);

        Assert.That(q, Is.Not.Null);
        Assert.That(q!.TriggerKeyword, Is.Empty);
        Assert.That(q.Command, Is.Empty);
        Assert.That(q.Search, Is.EqualTo("wpm file.txt file2 file3"));
    }
}