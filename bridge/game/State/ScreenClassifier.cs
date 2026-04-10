using MegaCrit.Sts2.Core.Nodes;
using MegaCrit.Sts2.Core.Nodes.CommonUi;
using MegaCrit.Sts2.Core.Runs;

namespace Spire2Mind.Bridge.Game.State;

internal static class ScreenClassifier
{
    // Game node type names mapped to screen IDs.
    // These are class names from the game assembly; if the game renames them,
    // update here (compile will still succeed, but screens will classify as Unknown).
    private static readonly Dictionary<string, string> TypeNameToScreenId = new(StringComparer.Ordinal)
    {
        ["NRewardsScreen"] = ScreenIds.Reward,
        ["NCardRewardSelectionScreen"] = ScreenIds.Reward,
        ["NMapScreen"] = ScreenIds.Map,
        ["NMapRoom"] = ScreenIds.Map,
        ["NCombatRoom"] = ScreenIds.Combat,
        ["NEventRoom"] = ScreenIds.Event,
        ["NMerchantRoom"] = ScreenIds.Shop,
        ["NMerchantInventory"] = ScreenIds.Shop,
        ["NRestSiteRoom"] = ScreenIds.Rest,
        ["NTreasureRoom"] = ScreenIds.Chest,
        ["NTreasureRoomRelicCollection"] = ScreenIds.Chest,
        ["NGameOverScreen"] = ScreenIds.GameOver,
    };

    public static string Classify(object? currentScreen)
    {
        if (NModalContainer.Instance?.OpenModal != null)
        {
            return ScreenIds.Modal;
        }

        if (currentScreen == null)
        {
            return GameUiAccess.ResolveSelectionContext(null) != null
                ? ScreenIds.CardSelection
                : ScreenIds.Unknown;
        }

        if (GameUiAccess.ResolveSelectionContext(currentScreen) != null)
        {
            return ScreenIds.CardSelection;
        }

        var typeName = currentScreen.GetType().Name;

        if (TypeNameToScreenId.TryGetValue(typeName, out var screenId))
        {
            return screenId;
        }

        if (typeName == "NMainMenu")
        {
            return RunManager.Instance.DebugOnlyGetState() == null && HasVisibleCharacterSelect()
                ? ScreenIds.CharacterSelect
                : ScreenIds.MainMenu;
        }

        return typeName.Contains("CharacterSelect", StringComparison.Ordinal)
            ? ScreenIds.CharacterSelect
            : ScreenIds.Unknown;
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
