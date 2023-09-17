namespace Wox.Plugin;

/// <summary>
///     Query from Wox. See <see href="Doc/Query.md" /> for details.
/// </summary>
public class Query
{
    /// <summary>
    ///     Raw query, this includes trigger keyword if it has
    ///     We didn't recommend use this property directly. You should always use Search property.
    /// </summary>
    public required string RawQuery { get; init; }

    /// <summary>
    ///     Trigger keyword of a query. It can be empty if user is using global trigger keyword.
    ///     Empty trigger keyword means this query will be a global query.
    /// </summary>
    public required string TriggerKeyword { get; init; }

    /// <summary>
    ///     Command part of a query.
    ///     Empty command means this query doesn't have a command.
    /// </summary>
    public required string Command { get; init; }

    /// <summary>
    ///     Search part of a query.
    ///     Empty search means this query doesn't have a search part.
    /// </summary>
    public required string Search { get; init; }

    public bool IsEmpty => string.IsNullOrEmpty(TriggerKeyword) && string.IsNullOrEmpty(Command) && string.IsNullOrEmpty(Search);

    public override string ToString()
    {
        return RawQuery;
    }

    public override bool Equals(object? obj)
    {
        if (obj is Query query) return RawQuery == query.RawQuery;

        return false;
    }

    public override int GetHashCode()
    {
        return RawQuery.GetHashCode();
    }
}