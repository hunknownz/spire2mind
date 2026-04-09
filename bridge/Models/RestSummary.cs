namespace Spire2Mind.Bridge.Models;

internal sealed class RestSummary
{
    public IReadOnlyList<RestOptionSummary> Options { get; init; } = Array.Empty<RestOptionSummary>();
}

internal sealed class RestOptionSummary
{
    public int Index { get; init; }

    public string? OptionId { get; init; }

    public string? Title { get; init; }

    public string? Description { get; init; }

    public bool? IsEnabled { get; init; }
}
