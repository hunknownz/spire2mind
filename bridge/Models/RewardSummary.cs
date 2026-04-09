namespace Spire2Mind.Bridge.Models;

internal sealed class RewardSummary
{
    public string Phase { get; init; } = string.Empty;

    public string? SourceScreen { get; init; }

    public string? SourceHint { get; init; }

    public string? ScreenType { get; init; }

    public bool? PendingCardChoice { get; init; }

    public bool? CanProceed { get; init; }

    public int? RewardCount { get; init; }

    public int? ClaimableRewardCount { get; init; }

    public int? AlternativeCount { get; init; }

    public IReadOnlyList<RewardItemSummary> Rewards { get; init; } = Array.Empty<RewardItemSummary>();

    public IReadOnlyList<CardSummary> CardOptions { get; init; } = Array.Empty<CardSummary>();

    public IReadOnlyList<AlternativeSummary> Alternatives { get; init; } = Array.Empty<AlternativeSummary>();
}

internal sealed class RewardItemSummary
{
    public int Index { get; init; }

    public string? RewardType { get; init; }

    public string? Description { get; init; }

    public bool? Claimable { get; init; }

    public string? BlockedReason { get; init; }
}

internal sealed class AlternativeSummary
{
    public int Index { get; init; }

    public string? OptionId { get; init; }

    public string? Title { get; init; }
}
