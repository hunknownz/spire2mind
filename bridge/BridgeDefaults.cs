namespace Spire2Mind.Bridge;

internal static class BridgeDefaults
{
    public const int DefaultPort = 8080;

    public const int StateVersion = 1;

    public const int PortRetryCount = 20;

    public const int EventBufferCapacity = 256;

    public static readonly TimeSpan PortRetryDelay = TimeSpan.FromMilliseconds(250);

    public static readonly TimeSpan EventPollInterval = TimeSpan.FromMilliseconds(120);

    public static readonly TimeSpan EventKeepAliveInterval = TimeSpan.FromSeconds(15);

    /// <summary>
    /// Default timeout for combat actions (play card, end turn). Override with STS2_COMBAT_TIMEOUT_MS.
    /// </summary>
    public static readonly TimeSpan CombatActionTimeout = ResolveTimeout("STS2_COMBAT_TIMEOUT_MS", 10_000);

    /// <summary>
    /// Default timeout for navigation actions (map, proceed). Override with STS2_NAV_TIMEOUT_MS.
    /// </summary>
    public static readonly TimeSpan NavigationTimeout = ResolveTimeout("STS2_NAV_TIMEOUT_MS", 12_000);

    /// <summary>
    /// Default timeout for major transitions (embark, return to menu). Override with STS2_TRANSITION_TIMEOUT_MS.
    /// </summary>
    public static readonly TimeSpan TransitionTimeout = ResolveTimeout("STS2_TRANSITION_TIMEOUT_MS", 15_000);

    private static TimeSpan ResolveTimeout(string envVar, int defaultMs)
    {
        var raw = Environment.GetEnvironmentVariable(envVar);
        return int.TryParse(raw, out var ms) ? TimeSpan.FromMilliseconds(ms) : TimeSpan.FromMilliseconds(defaultMs);
    }
}
