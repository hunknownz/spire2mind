extern alias Harmony;

using System.Reflection;
using System.Threading;
using HarmonyLib = Harmony::HarmonyLib;
using MegaCrit.Sts2.Core.Logging;
using MegaCrit.Sts2.Core.Modding;
using MegaCrit.Sts2.Core.Nodes;

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

            Log.Info($"[{ModId}] Initialized bridge patches for {DisplayName} {BridgeVersion}.");

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
