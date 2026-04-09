namespace Spire2Mind.Bridge.Models;

internal sealed class ChestSummary
{
    public bool? IsOpened { get; init; }

    public bool? HasRelicBeenClaimed { get; init; }

    public IReadOnlyList<ChestRelicSummary> RelicOptions { get; init; } = Array.Empty<ChestRelicSummary>();
}

internal sealed class ChestRelicSummary
{
    public int Index { get; init; }

    public string? RelicId { get; init; }

    public string? Name { get; init; }

    public string? Rarity { get; init; }
}
