namespace Spire2Mind.Bridge.Models;

internal sealed class MapSummary
{
    public MapNodeSummary? CurrentNode { get; init; }

    public bool? IsTravelEnabled { get; init; }

    public bool? IsTraveling { get; init; }

    public IReadOnlyList<MapNodeSummary> AvailableNodes { get; init; } = Array.Empty<MapNodeSummary>();
}

internal sealed class MapNodeSummary
{
    public int Index { get; init; }

    public int? Row { get; init; }

    public int? Col { get; init; }

    public string? NodeType { get; init; }
}
