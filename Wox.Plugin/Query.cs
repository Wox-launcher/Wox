namespace Wox.Plugin;

/// <summary>
///     Query from Wox. See <see href="Doc/Query.md" /> for details.
/// </summary>
public class Query
{
    public Query(string rawQuery, string triggerKeyword, string command, string search)
    {
        RawQuery = rawQuery;
        TriggerKeyword = triggerKeyword;
        Command = command;
        Search = search;
    }

    /// <summary>
    ///     Raw query, this includes trigger keyword if it has
    ///     We didn't recommend use this property directly. You should always use Search property.
    /// </summary>
    public string RawQuery { get; }

    /// <summary>
    ///     Trigger keyword of a query. It can be empty if user is using global trigger keyword.
    /// </summary>
    public string TriggerKeyword { get; }

    /// <summary>
    ///     Command part of a query.
    /// </summary>
    public string Command { get; }

    /// <summary>
    ///     Search part of a query.
    /// </summary>
    public string Search { get; }

    public override string ToString()
    {
        return RawQuery;
    }
}