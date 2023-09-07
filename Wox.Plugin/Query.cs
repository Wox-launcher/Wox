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
    /// </summary>
    public string? TriggerKeyword { get; init; }

    /// <summary>
    ///     Command part of a query.
    /// </summary>
    public string? Command { get; init; }

    /// <summary>
    ///     Search part of a query.
    /// </summary>
    public required string Search { get; init; }

    public override string ToString()
    {
        return RawQuery;
    }
}