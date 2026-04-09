namespace Spire2Mind.Bridge.Models;

internal sealed class EventSummary
{
    public string? EventId { get; init; }

    public string? Title { get; init; }

    public string? Description { get; init; }

    public bool? IsFinished { get; init; }

    public IReadOnlyList<EventOptionSummary> Options { get; init; } = Array.Empty<EventOptionSummary>();
}

internal sealed class EventOptionSummary
{
    public int Index { get; init; }

    public string? Title { get; init; }

    public string? Description { get; init; }

    public bool? IsLocked { get; init; }

    public bool? IsProceed { get; init; }
}
