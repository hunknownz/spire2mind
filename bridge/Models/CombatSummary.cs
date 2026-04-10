namespace Spire2Mind.Bridge.Models;

internal sealed class CombatSummary
{
    public bool? ActionWindowOpen { get; init; }

    public bool? IsInCardPlay { get; init; }

    public bool? IsInCardSelection { get; init; }

    public bool? PlayerActionsDisabled { get; init; }

    public bool? IsOverOrEnding { get; init; }

    public PlayerSummary? Player { get; init; }

    public IReadOnlyList<EnemySummary> Enemies { get; init; } = Array.Empty<EnemySummary>();

    public IReadOnlyList<CardSummary> Hand { get; init; } = Array.Empty<CardSummary>();

    public int? DrawPileCount { get; init; }

    public int? DiscardPileCount { get; init; }

    public int? ExhaustPileCount { get; init; }

    public IReadOnlyList<CardSummary> DrawPile { get; init; } = Array.Empty<CardSummary>();

    public IReadOnlyList<CardSummary> DiscardPile { get; init; } = Array.Empty<CardSummary>();

    public IReadOnlyList<CardSummary> ExhaustPile { get; init; } = Array.Empty<CardSummary>();
}

internal sealed class PlayerSummary
{
    public int? CurrentHp { get; init; }

    public int? MaxHp { get; init; }

    public int? Block { get; init; }

    public int? Energy { get; init; }

    public int? Stars { get; init; }

    public IReadOnlyList<PowerSummary> Powers { get; init; } = Array.Empty<PowerSummary>();
}

internal sealed class EnemySummary
{
    public int Index { get; init; }

    public string? EnemyId { get; init; }

    public string? Name { get; init; }

    public int? CurrentHp { get; init; }

    public int? MaxHp { get; init; }

    public int? Block { get; init; }

    public bool? IsAlive { get; init; }

    public bool? IsHittable { get; init; }

    public string? MoveId { get; init; }

    public IReadOnlyList<PowerSummary> Powers { get; init; } = Array.Empty<PowerSummary>();

    public IReadOnlyList<EnemyIntentSummary> Intents { get; init; } = Array.Empty<EnemyIntentSummary>();
}

internal sealed class EnemyIntentSummary
{
    public int Index { get; init; }

    public string? IntentType { get; init; }

    public string? Label { get; init; }

    public int? Damage { get; init; }

    public int? Hits { get; init; }

    public int? TotalDamage { get; init; }

    public int? StatusCardCount { get; init; }
}

internal sealed class PowerSummary
{
    public string? PowerId { get; init; }

    public string? Name { get; init; }

    public int? Amount { get; init; }

    public string? PowerType { get; init; }
}

internal sealed class CardSummary
{
    public int Index { get; init; }

    public string? CardId { get; init; }

    public string? Name { get; init; }

    public int? EnergyCost { get; init; }

    public int? StarCost { get; init; }

    public bool? CostsX { get; init; }

    public bool? StarCostsX { get; init; }

    public string? TargetType { get; init; }

    public bool? RequiresTarget { get; init; }

    public bool? Playable { get; init; }

    public bool? IsSelected { get; init; }

    public IReadOnlyList<int> ValidTargetIndices { get; init; } = Array.Empty<int>();
}
