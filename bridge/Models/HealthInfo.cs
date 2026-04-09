namespace Spire2Mind.Bridge.Models;

internal sealed class HealthInfo
{
    public string BridgeVersion { get; init; } = Entry.BridgeVersion;

    public string? GameVersion { get; init; }

    public string? GameBuildDate { get; init; }

    public int Port { get; init; }

    public bool Ready { get; init; }
}
