using MegaCrit.Sts2.Core.Nodes;
using MegaCrit.Sts2.Core.Nodes.CommonUi;
using MegaCrit.Sts2.Core.Runs;

namespace Spire2Mind.Bridge.Game.State;

internal static class ScreenClassifier
{
    public static string Classify(object? currentScreen)
    {
        if (NModalContainer.Instance?.OpenModal != null)
        {
            return ScreenIds.Modal;
        }

        if (currentScreen == null)
        {
            if (GameUiAccess.ResolveSelectionContext(null) != null)
            {
                return ScreenIds.CardSelection;
            }

            return ScreenIds.Unknown;
        }

        if (GameUiAccess.ResolveSelectionContext(currentScreen) != null)
        {
            return ScreenIds.CardSelection;
        }

        var typeName = currentScreen.GetType().Name;
        return typeName switch
        {
            "NRewardsScreen" => ScreenIds.Reward,
            "NCardRewardSelectionScreen" => ScreenIds.Reward,
            "NMapScreen" => ScreenIds.Map,
            "NMapRoom" => ScreenIds.Map,
            "NCombatRoom" => ScreenIds.Combat,
            "NEventRoom" => ScreenIds.Event,
            "NMerchantRoom" => ScreenIds.Shop,
            "NMerchantInventory" => ScreenIds.Shop,
            "NRestSiteRoom" => ScreenIds.Rest,
            "NTreasureRoom" => ScreenIds.Chest,
            "NTreasureRoomRelicCollection" => ScreenIds.Chest,
            "NGameOverScreen" => ScreenIds.GameOver,
            "NMainMenu" => RunManager.Instance.DebugOnlyGetState() == null && HasVisibleCharacterSelect()
                ? ScreenIds.CharacterSelect
                : ScreenIds.MainMenu,
            _ => typeName.Contains("CharacterSelect", StringComparison.Ordinal) ? ScreenIds.CharacterSelect : ScreenIds.Unknown
        };
    }

    private static bool HasVisibleCharacterSelect()
    {
        var mainMenu = NGame.Instance?.MainMenu;
        if (mainMenu == null)
        {
            return false;
        }

        return ReflectionUtils.DescendantsByTypeName(mainMenu, "NCharacterSelectScreen").Any(ReflectionUtils.IsVisible);
    }
}
