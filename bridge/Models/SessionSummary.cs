namespace Spire2Mind.Bridge.Models;

internal sealed class SessionSummary
{
    public string Mode { get; init; } = "singleplayer";

    public string Phase { get; init; } = "menu";

    public string ControlScope { get; init; } = "local_player";
}
