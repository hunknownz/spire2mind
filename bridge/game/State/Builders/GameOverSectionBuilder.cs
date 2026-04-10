using Godot;
using MegaCrit.Sts2.Core.Context;
using MegaCrit.Sts2.Core.Nodes.Screens.GameOverScreen;
using MegaCrit.Sts2.Core.Runs;
using Spire2Mind.Bridge.Models;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.Ui;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.Util;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.Ui;
using Spire2Mind.Bridge.Game.Threading;

namespace Spire2Mind.Bridge.Game.State.Builders;

internal static class GameOverSectionBuilder
{
    public static GameOverSummary? Build(object? currentScreen, RunState? runState)
    {
        if (currentScreen is not NGameOverScreen screen)
        {
            return null;
        }

        var continueButton = GameUiAccess.GetGameOverContinueButton(screen);
        var mainMenuButton = GameUiAccess.GetGameOverMainMenuButton(screen);
        var localPlayer = LocalContext.GetMe(runState);
        var history = ReflectionUtils.GetMemberValue(RunManager.Instance, "History");

        return new GameOverSummary
        {
            Stage = ResolveStage(continueButton, mainMenuButton),
            IsVictory = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(history, "Win"))
                ?? ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(ReflectionUtils.GetMemberValue(runState, "CurrentRoom"), "IsVictoryRoom")),
            Floor = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(runState, "TotalFloor", "ActFloor")),
            CharacterId = ReflectionUtils.ModelId(ReflectionUtils.GetMemberValue(localPlayer, "Character")),
            CanContinue = UiControlHelper.IsAvailable(continueButton),
            CanReturnToMainMenu = UiControlHelper.IsAvailable(mainMenuButton),
            ShowingSummary = ReflectionUtils.IsVisible(mainMenuButton) || ReflectionUtils.IsEnabled(mainMenuButton)
        };
    }

    private static string ResolveStage(Node? continueButton, Node? mainMenuButton)
    {
        if (UiControlHelper.IsAvailable(continueButton))
        {
            return "results";
        }

        if (UiControlHelper.IsAvailable(mainMenuButton) ||
            ReflectionUtils.IsVisible(mainMenuButton) ||
            ReflectionUtils.IsEnabled(mainMenuButton))
        {
            return "summary";
        }

        return "transition";
    }
}
