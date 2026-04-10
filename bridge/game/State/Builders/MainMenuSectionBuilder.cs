using MegaCrit.Sts2.Core.Nodes;
using Spire2Mind.Bridge.Game.Util;

namespace Spire2Mind.Bridge.Game.State.Builders;

internal static class MainMenuSectionBuilder
{
    public static MainMenuState? Build()
    {
        var mainMenu = NGame.Instance?.MainMenu;
        if (mainMenu == null)
        {
            return null;
        }

        return new MainMenuState
        {
            CanContinueRun = UiControlHelper.IsAvailable(GameUiAccess.GetMainMenuContinueButton(mainMenu)),
            CanAbandonRun = UiControlHelper.IsAvailable(GameUiAccess.GetMainMenuAbandonRunButton(mainMenu)),
            CanOpenCharacterSelect =
                UiControlHelper.IsAvailable(GameUiAccess.GetMainMenuSingleplayerButton(mainMenu)) ||
                (!UiControlHelper.IsAvailable(GameUiAccess.GetMainMenuContinueButton(mainMenu)) &&
                 !UiControlHelper.IsAvailable(GameUiAccess.GetMainMenuAbandonRunButton(mainMenu))),
            CanOpenMultiplayer = UiControlHelper.IsAvailable(GameUiAccess.GetMainMenuMultiplayerButton(mainMenu)),
            CanOpenCompendium = UiControlHelper.HasAvailableControl(mainMenu, "_compendiumButton"),
            CanOpenTimeline = UiControlHelper.IsAvailable(GameUiAccess.GetMainMenuTimelineButton(mainMenu)),
            CanOpenSettings = UiControlHelper.IsAvailable(GameUiAccess.GetMainMenuSettingsButton(mainMenu)),
            CanOpenProfile = UiControlHelper.IsAvailable(GameUiAccess.GetMainMenuProfileButton(mainMenu)),
            CanViewPatchNotes = UiControlHelper.IsAvailable(GameUiAccess.GetMainMenuPatchNotesButton(mainMenu)),
            CanQuitGame = UiControlHelper.IsAvailable(GameUiAccess.GetMainMenuQuitButton(mainMenu))
        };
    }
}
