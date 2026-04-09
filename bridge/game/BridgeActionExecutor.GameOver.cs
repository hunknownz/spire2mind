using Godot;
using MegaCrit.Sts2.Core.Nodes.Screens.GameOverScreen;
using MegaCrit.Sts2.Core.Nodes.Screens.ScreenContext;
using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Game;

internal static partial class BridgeActionExecutor
{
    private static async Task<BridgeActionResult> ExecuteContinueAfterGameOverAsync()
    {
        EnsureActionAvailable(ActionIds.ContinueAfterGameOver);

        var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        var continueButton = GameUiAccess.GetGameOverContinueButton(currentScreen)
            ?? throw StateUnavailable(ActionIds.ContinueAfterGameOver, "Game over continue button is unavailable.");

        ClickControl(continueButton);
        var stable = await WaitUntilAsync(
            () =>
            {
                var snapshot = StateSnapshotBuilder.Build();
                if (snapshot.Screen != ScreenIds.GameOver)
                {
                    return IsSnapshotActionableOrSettled(snapshot);
                }

                return snapshot.GameOver?.CanContinue != true;
            },
            TimeSpan.FromSeconds(10));

        return BuildResult(ActionIds.ContinueAfterGameOver, stable);
    }

    private static async Task<BridgeActionResult> ExecuteReturnToMainMenuAsync()
    {
        EnsureActionAvailable(ActionIds.ReturnToMainMenu);

        var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        if (currentScreen is not NGameOverScreen gameOverScreen)
        {
            throw InvalidAction(ActionIds.ReturnToMainMenu);
        }

        if (!TryInvokeMethod(gameOverScreen, NGameOverScreen.MethodName.ReturnToMainMenu))
        {
            var mainMenuButton = GameUiAccess.GetGameOverMainMenuButton(currentScreen)
                ?? throw StateUnavailable(ActionIds.ReturnToMainMenu, "Game over main menu button is unavailable.");
            ClickControl(mainMenuButton);
        }

        var stable = await WaitForStableSnapshotAsync(
            snapshot => snapshot.Screen == ScreenIds.MainMenu && snapshot.AvailableActions.Count > 0,
            TimeSpan.FromSeconds(15));

        return BuildResult(ActionIds.ReturnToMainMenu, stable);
    }
}
