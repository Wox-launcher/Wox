using Wox.Core.Plugin;
using Wox.Plugin;

namespace Wox.Core;

using TriggerKeyword = String;

public static class QueryBuilder
{
    private const string TermSeparator = " ";

    public static Query Build(string text)
    {
        return BuildByPlugins(text, PluginManager.GetAllPlugins());
    }

    public static Query BuildByPlugins(string text, List<PluginInstance> pluginInstances)
    {
        // replace multiple white spaces with one white space
        var terms = text.Split(new[] { TermSeparator }, StringSplitOptions.RemoveEmptyEntries);
        if (terms.Length == 0)
            return new Query
            {
                RawQuery = text,
                TriggerKeyword = string.Empty,
                Command = string.Empty,
                Search = string.Empty
            };

        var rawQuery = string.Join(TermSeparator, terms);
        string triggerKeyword, command, search;
        var possibleTriggerKeyword = terms[0];
        var mustContainTermSeparator = text.Contains(TermSeparator);

        var pluginInstance = pluginInstances.FirstOrDefault(o => o.TriggerKeywords.Contains(possibleTriggerKeyword) && !o.CommonSetting.Disabled);
        if (pluginInstance != null && mustContainTermSeparator)
        {
            // non global trigger keyword
            triggerKeyword = possibleTriggerKeyword;

            if (!terms.Skip(1).Any())
            {
                // no command and search
                command = string.Empty;
                search = string.Empty;
            }
            else
            {
                var possibleCommand = terms[1];
                if (pluginInstance.Metadata.Commands.Any(o => o.Command.Contains(possibleCommand)))
                {
                    // command and search
                    command = possibleCommand;
                    search = string.Join(TermSeparator, terms.Skip(2));
                }
                else
                {
                    // no command, only search
                    command = string.Empty;
                    search = string.Join(TermSeparator, terms.Skip(1));
                }
            }
        }
        else
        {
            // non trigger keyword
            triggerKeyword = string.Empty;
            command = string.Empty;
            search = rawQuery;
        }

        return new Query
        {
            RawQuery = text,
            TriggerKeyword = triggerKeyword,
            Command = command,
            Search = search
        };
    }
}