namespace Spire2Mind.Bridge.Models;

internal sealed class BridgeStateSnapshot
{
    public int StateVersion { get; init; } = BridgeDefaults.StateVersion;

    public string RunId { get; init; } = "run_unknown";

    public string Screen { get; init; } = "UNKNOWN";

    public bool InCombat { get; init; }

    public int? Turn { get; init; }

    public IReadOnlyList<string> AvailableActions { get; init; } = Array.Empty<string>();

    public SessionSummary? Session { get; init; }

    public RunSummary? Run { get; init; }

    public CombatSummary? Combat { get; init; }

    public MapSummary? Map { get; init; }

    public SelectionSummary? Selection { get; init; }

    public RewardSummary? Reward { get; init; }

    public EventSummary? Event { get; init; }

    public CharacterSelectSummary? CharacterSelect { get; init; }

    public ChestSummary? Chest { get; init; }

    public ShopSummary? Shop { get; init; }

    public RestSummary? Rest { get; init; }

    public ModalSummary? Modal { get; init; }

    public GameOverSummary? GameOver { get; init; }

    public MultiplayerSummary? Multiplayer { get; init; }

    public MultiplayerLobbySummary? MultiplayerLobby { get; init; }

    public TimelineSummary? Timeline { get; init; }

    public AgentViewSummary? AgentView { get; init; }
}
