namespace Spire2Mind.Bridge.Models;

internal sealed class SelectionSummary
{
    public string Kind { get; init; } = string.Empty;

    public string? SourceScreen { get; init; }

    public string? SourceHint { get; init; }

    public string? Mode { get; init; }

    public bool? IsCombatEmbedded { get; init; }

    public string? Prompt { get; init; }

    public int? MinSelect { get; init; }

    public int? MaxSelect { get; init; }

    public int? SelectedCount { get; init; }

    public bool? RequiresConfirmation { get; init; }

    public bool? CanConfirm { get; init; }

    public IReadOnlyList<CardSummary> Cards { get; init; } = Array.Empty<CardSummary>();
}
