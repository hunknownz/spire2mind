namespace Spire2Mind.Bridge.Models;

internal sealed class CharacterSelectSummary
{
    public string? SelectedCharacterId { get; init; }

    public IReadOnlyList<CharacterOptionSummary> Characters { get; init; } = Array.Empty<CharacterOptionSummary>();

    public bool? CanEmbark { get; init; }
}

internal sealed class CharacterOptionSummary
{
    public int Index { get; init; }

    public string? CharacterId { get; init; }

    public string? Name { get; init; }

    public bool? IsLocked { get; init; }

    public bool? IsSelected { get; init; }

    public bool? IsRandom { get; init; }
}
