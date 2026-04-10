namespace Spire2Mind.Bridge.Models;

internal sealed class RunSummary
{
    public string? Character { get; init; }

    public int? Floor { get; init; }

    public int? CurrentHp { get; init; }

    public int? MaxHp { get; init; }

    public int? Gold { get; init; }

    public int? MaxEnergy { get; init; }

    public int? DeckCount { get; init; }

    public int? RelicCount { get; init; }

    public int? PotionCount { get; init; }

    public IReadOnlyList<RunCardSummary> Deck { get; init; } = Array.Empty<RunCardSummary>();

    public IReadOnlyList<RunRelicSummary> Relics { get; init; } = Array.Empty<RunRelicSummary>();

    public IReadOnlyList<RunPotionSummary> Potions { get; init; } = Array.Empty<RunPotionSummary>();
}

internal sealed class RunCardSummary
{
    public string? CardId { get; init; }

    public string? Name { get; init; }
}

internal sealed class RunRelicSummary
{
    public string? RelicId { get; init; }

    public string? Name { get; init; }
}

internal sealed class RunPotionSummary
{
    public string? PotionId { get; init; }

    public string? Name { get; init; }

    public bool? IsEmpty { get; init; }
}
