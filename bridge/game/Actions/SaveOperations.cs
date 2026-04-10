using MegaCrit.Sts2.Core.Helpers;
using MegaCrit.Sts2.Core.Multiplayer;
using MegaCrit.Sts2.Core.Multiplayer.Game;
using MegaCrit.Sts2.Core.Nodes;
using MegaCrit.Sts2.Core.Runs;
using MegaCrit.Sts2.Core.Saves;

namespace Spire2Mind.Bridge.Game.Actions;

/// <summary>
/// Save/load operations for run management.
/// Based on patterns from StS2-Quick-Restart community mod.
/// Must run on the game thread.
/// </summary>
internal static class SaveOperations
{
    public static object GetSaveStatus()
    {
        var hasSave = SaveManager.Instance?.HasRunSave ?? false;
        var isSingleplayer = RunManager.Instance?.NetService?.Type == NetGameType.Singleplayer;

        return new
        {
            hasSave,
            isSingleplayer,
            canRestart = hasSave && isSingleplayer
        };
    }

    public static object RestartRoom()
    {
        if (RunManager.Instance?.NetService?.Type != NetGameType.Singleplayer)
        {
            return new { success = false, message = "Restart only available in singleplayer." };
        }

        if (SaveManager.Instance?.HasRunSave != true)
        {
            return new { success = false, message = "No run save available." };
        }

        try
        {
            // Cleanup current state
            RunManager.Instance.ActionQueueSet.Reset();
            RunManager.Instance.CleanUp();

            // Load save
            var runSave = SaveManager.Instance.LoadRunSave();
            var serializableRun = runSave.SaveData;
            var runState = RunState.FromSerializable(serializableRun);

            // Reconstruct game state
            RunManager.Instance.SetUpSavedSinglePlayer(runState, serializableRun);
            NGame.Instance!.ReactionContainer.InitializeNetworking(new NetSingleplayerGameService());
            TaskHelper.RunSafely(NGame.Instance.LoadRun(runState, serializableRun.PreFinishedRoom));

            return new { success = true, message = "Room restart initiated." };
        }
        catch (Exception ex)
        {
            return new { success = false, message = $"Restart failed: {ex.Message}" };
        }
    }
}
