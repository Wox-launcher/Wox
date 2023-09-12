namespace Wox.Plugin;

using HideAppAfterSelect = Boolean;

public class Result
{
    /// <summary>
    ///     Result id, should be unique. It's optional, if you don't set it, Wox will assign a random id for you
    /// </summary>
    public string? Id { get; set; }

    public required string Title { get; init; }

    public required string IcoPath { get; init; }

    public string? Description { get; init; }

    public int Score { get; init; }

    public Func<HideAppAfterSelect>? Action { get; set; }
}