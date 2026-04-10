using Spire2Mind.Bridge.Game.Hooks;

namespace Spire2Mind.Bridge;

internal static class BridgeDefaults
{
    public const int DefaultPort = 8080;

    public const int StateVersion = 1;

    public const int PortRetryCount = 20;

    public const int EventBufferCapacity = 256;

    public static readonly TimeSpan PortRetryDelay = TimeSpan.FromMilliseconds(250);

    public static readonly TimeSpan EventKeepAliveInterval = TimeSpan.FromSeconds(15);

    /// <summary>
    /// Event poll interval. Reads from BridgeConfig if available, else 120ms.
    /// </summary>
    public static TimeSpan EventPollInterval =>
        TimeSpan.FromMilliseconds(BridgeConfig.PollIntervalMs);

    /// <summary>
    /// Combat action timeout (play card, end turn). Reads from config or STS2_COMBAT_TIMEOUT_MS env var.
    /// </summary>
    public static TimeSpan CombatActionTimeout =>
        ResolveTimeout("STS2_COMBAT_TIMEOUT_MS", BridgeConfig.CombatTimeoutMs);

    /// <summary>
    /// Navigation timeout (map, proceed). Reads from config or STS2_NAV_TIMEOUT_MS env var.
    /// </summary>
    public static TimeSpan NavigationTimeout =>
        ResolveTimeout("STS2_NAV_TIMEOUT_MS", BridgeConfig.NavigationTimeoutMs);

    /// <summary>
    /// Major transition timeout (embark, return to menu). Reads from config or STS2_TRANSITION_TIMEOUT_MS env var.
    /// </summary>
    public static TimeSpan TransitionTimeout =>
        ResolveTimeout("STS2_TRANSITION_TIMEOUT_MS", BridgeConfig.TransitionTimeoutMs);

    private static TimeSpan ResolveTimeout(string envVar, int configMs)
    {
        var raw = Environment.GetEnvironmentVariable(envVar);
        return int.TryParse(raw, out var ms) ? TimeSpan.FromMilliseconds(ms) : TimeSpan.FromMilliseconds(configMs);
    }
}
