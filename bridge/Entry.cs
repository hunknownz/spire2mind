extern alias Harmony;

using System.Reflection;
using System.Threading;
using HarmonyLib = Harmony::HarmonyLib;
using MegaCrit.Sts2.Core.Logging;
using MegaCrit.Sts2.Core.Modding;
using MegaCrit.Sts2.Core.Nodes;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.Hooks;
using Spire2Mind.Bridge.Game.Util;

namespace Spire2Mind.Bridge;

[ModInitializer(nameof(Initialize))]
public static class Entry
{
    internal const string ModId = "Spire2Mind.Bridge";

    internal const string DisplayName = "Spire2Mind Bridge";

    internal const string BridgeVersion = "0.1.0-mvp";

    private static int _initialized;

    private static HarmonyLib.Harmony? _harmony;

    public static void Initialize()
    {
        if (Interlocked.Exchange(ref _initialized, 1) == 1)
        {
            return;
        }

        try
        {
            _harmony = new HarmonyLib.Harmony(ModId);
            _harmony.PatchAll(Assembly.GetExecutingAssembly());

            // BridgeHooker (CustomSingletonModel) registers combat/run hooks.
            // Guard inside constructor prevents duplicate subscription if BaseLib
            // also discovers and instantiates it during post-mod-init scan.
            _ = new BridgeHooker();

            Log.Info($"[{ModId}] Initialized bridge patches and hooks for {DisplayName} {BridgeVersion}.");

            if (NGame.Instance != null)
            {
                BridgeRuntime.EnsureStarted();
            }
        }
        catch (Exception ex)
        {
            Log.Error($"[{ModId}] Failed during initialization: {ex}");
            throw;
        }
    }
}
