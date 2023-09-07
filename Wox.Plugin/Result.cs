namespace Wox.Plugin;

using HideAppAfterSelect = Boolean;

public class Result
{
    public required string Title { get; init; }

    public required string IcoPath { get; init; }

    public string? Description { get; init; }

    public int? Score { get; init; }

    public Func<HideAppAfterSelect>? Action { get; init; }
}