extern alias Harmony;

using HarmonyLib = Harmony::HarmonyLib;
using MegaCrit.Sts2.Core.Logging;
using MegaCrit.Sts2.Core.Nodes;

namespace Spire2Mind.Bridge;

[HarmonyLib.HarmonyPatch(typeof(NGame), "_Ready")]
internal static class NGameReadyPatch
{
    private static void Postfix()
    {
        try
        {
            BridgeRuntime.EnsureStarted();
        }
        catch (Exception ex)
        {
            Log.Error($"[{Entry.ModId}] Failed to bootstrap on NGame._Ready: {ex}");
        }
    }
}

[HarmonyLib.HarmonyPatch(typeof(NGame), "_ExitTree")]
internal static class NGameExitTreePatch
{
    private static void Prefix()
    {
        BridgeRuntime.Stop();
    }
}
