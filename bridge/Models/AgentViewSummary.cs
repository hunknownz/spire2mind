namespace Spire2Mind.Bridge.Models;

internal sealed class AgentViewSummary
{
    public string? Headline { get; init; }

    public int? Floor { get; init; }

    public int? Turn { get; init; }

    public int AvailableActionCount { get; init; }

    public int? HandCount { get; init; }

    public int? EnemyCount { get; init; }
}
