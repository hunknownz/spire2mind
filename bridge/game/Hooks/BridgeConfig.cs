using BaseLib.Config;

namespace Spire2Mind.Bridge.Game.Hooks;

/// <summary>
/// Game settings UI for Spire2Mind Bridge.
/// Accessible via the in-game mod settings menu.
/// Changes take effect on next game action or poll cycle.
/// </summary>
public class BridgeConfig : SimpleModConfig
{
    [ConfigSection("server")]
    [SliderRange(1000, 30000, 1000)]
    [SliderLabelFormat("{0}")]
    public static int ApiPort { get; set; } = BridgeDefaults.DefaultPort;

    [ConfigSection("events")]
    [SliderRange(50, 1000, 50)]
    [SliderLabelFormat("{0}ms")]
    public static int PollIntervalMs { get; set; } = 120;

    [ConfigSection("timeouts")]
    [SliderRange(5000, 30000, 1000)]
    [SliderLabelFormat("{0}ms")]
    public static int CombatTimeoutMs { get; set; } = 10000;

    [SliderRange(5000, 30000, 1000)]
    [SliderLabelFormat("{0}ms")]
    public static int NavigationTimeoutMs { get; set; } = 12000;

    [SliderRange(5000, 60000, 1000)]
    [SliderLabelFormat("{0}ms")]
    public static int TransitionTimeoutMs { get; set; } = 15000;

    [ConfigSection("debug")]
    public static bool VerboseLogging { get; set; } = false;
}
