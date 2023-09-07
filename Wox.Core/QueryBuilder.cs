using Wox.Plugin;

namespace Wox.Core;

using TriggerKeyword = String;

public static class QueryBuilder
{
    private const string TermSeperater = " ";

    public static Query? Build(string text, Dictionary<TriggerKeyword, PluginMetadata> plugins)
    {
        // replace multiple white spaces with one white space
        var terms = text.Split(new[] { TermSeperater }, StringSplitOptions.RemoveEmptyEntries);
        if (terms.Length == 0)
            // nothing was typed
            return null;

        var rawQuery = string.Join(TermSeperater, terms);
        string triggerKeyword, command, search;
        var possibleTriggerKeyword = terms[0];

        if (plugins.TryGetValue(possibleTriggerKeyword, out var pluginMetadata) && !pluginMetadata.Disabled)
        {
            // non global trigger keyword
            triggerKeyword = possibleTriggerKeyword;

            if (!terms.Skip(1).Any())
            {
                // no command and search
                command = string.Empty;
                search = string.Empty;
            }

            var possibleCommand = terms[1];
            if (pluginMetadata.Commands.Contains(possibleCommand))
            {
                // command and search
                command = possibleCommand;
                search = string.Join(TermSeperater, terms.Skip(2));
            }
            else
            {
                // no command, only search
                command = string.Empty;
                search = string.Join(TermSeperater, terms.Skip(1));
            }
        }
        else
        {
            // non trigger keyword
            triggerKeyword = string.Empty;
            command = string.Empty;
            search = rawQuery;
        }

        return new Query(text, triggerKeyword, command, search);
    }
}