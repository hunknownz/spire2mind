namespace Spire2Mind.Bridge.Models;

internal sealed class GameOverSummary
{
    public string Stage { get; init; } = string.Empty;

    public bool? IsVictory { get; init; }

    public int? Floor { get; init; }

    public string? CharacterId { get; init; }

    public bool? CanContinue { get; init; }

    public bool? CanReturnToMainMenu { get; init; }

    public bool? ShowingSummary { get; init; }
}
