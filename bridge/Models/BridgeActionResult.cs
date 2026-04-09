namespace Spire2Mind.Bridge.Models;

internal sealed class BridgeActionResult
{
    public string Action { get; init; } = string.Empty;

    public string Status { get; init; } = "failed";

    public bool Stable { get; init; }

    public string Message { get; init; } = string.Empty;

    public BridgeStateSnapshot State { get; init; } = new();
}
