using Godot;
using MegaCrit.Sts2.Core.Entities.Merchant;
using MegaCrit.Sts2.Core.Nodes;
using MegaCrit.Sts2.Core.Nodes.Cards.Holders;
using MegaCrit.Sts2.Core.Nodes.Rooms;
using MegaCrit.Sts2.Core.Nodes.Screens.Map;
using MegaCrit.Sts2.Core.Nodes.Screens.ScreenContext;
using MegaCrit.Sts2.Core.Nodes.Screens.Shops;
using MegaCrit.Sts2.Core.Nodes.Screens.TreasureRoomRelic;
using MegaCrit.Sts2.Core.Runs;

namespace Spire2Mind.Bridge.Game.Ui;

internal static partial class GameUiAccess
{
    public static IReadOnlyList<NMapPoint> GetAvailableMapNodes(IScreenContext? currentScreen, RunState? runState)
    {
        if (!TryGetMapScreen(currentScreen, runState, out var mapScreen))
        {
            return Array.Empty<NMapPoint>();
        }

        return FindDescendants<NMapPoint>(mapScreen!)
            .Where(node => GodotObject.IsInstanceValid(node))
            .GroupBy(node => $"{node.Point.coord.row}:{node.Point.coord.col}")
            .Select(group => group
                .OrderBy(node => node.GlobalPosition.Y)
                .ThenBy(node => node.GlobalPosition.X)
                .First())
            .Where(node => node.IsEnabled)
            .OrderBy(node => node.Point.coord.row)
            .ThenBy(node => node.Point.coord.col)
            .ToArray();
    }

    public static bool TryGetMapScreen(IScreenContext? currentScreen, RunState? runState, out NMapScreen? mapScreen)
    {
        mapScreen = currentScreen as NMapScreen ?? NMapScreen.Instance;
        if (runState == null || currentScreen is not (NMapScreen or NMapRoom))
        {
            return false;
        }

        if (mapScreen == null || !GodotObject.IsInstanceValid(mapScreen))
        {
            return false;
        }

        return mapScreen.IsVisibleInTree() && mapScreen.IsOpen;
    }

    public static NMapScreen? GetActionableMapScreen(RunState? runState)
    {
        if (runState == null)
        {
            return null;
        }

        var mapScreen = NMapScreen.Instance;
        if (mapScreen == null || !GodotObject.IsInstanceValid(mapScreen) || !mapScreen.IsVisibleInTree() || !mapScreen.IsOpen)
        {
            return null;
        }

        if (!mapScreen.IsTravelEnabled)
        {
            return null;
        }

        return GetAvailableMapNodes(mapScreen, runState).Count > 0 ? mapScreen : null;
    }

    public static IReadOnlyList<Node> GetRewardButtons(IScreenContext? currentScreen)
    {
        if (currentScreen is not Node rewardScreen || currentScreen.GetType().Name != "NRewardsScreen")
        {
            return Array.Empty<Node>();
        }

        return ReflectionUtils.DescendantsByTypeName(rewardScreen, "NRewardButton")
            .Where(node => GodotObject.IsInstanceValid(node))
            .ToArray();
    }

    public static Node? GetRewardProceedButton(IScreenContext? currentScreen)
    {
        if (currentScreen is not Node rewardScreen || currentScreen.GetType().Name != "NRewardsScreen")
        {
            return null;
        }

        return ReflectionUtils.DescendantsByTypeName(rewardScreen, "NProceedButton")
            .FirstOrDefault(node => GodotObject.IsInstanceValid(node));
    }

    public static IReadOnlyList<NCardHolder> GetCardRewardOptions(IScreenContext? currentScreen)
    {
        if (currentScreen is not Node rewardScreen || currentScreen.GetType().Name != "NCardRewardSelectionScreen")
        {
            return Array.Empty<NCardHolder>();
        }

        return FindDescendants<NCardHolder>(rewardScreen)
            .Where(node => GodotObject.IsInstanceValid(node) && node.CardModel != null)
            .OrderBy(node => node.GlobalPosition.Y)
            .ThenBy(node => node.GlobalPosition.X)
            .ToArray();
    }

    public static IReadOnlyList<Node> GetCardRewardAlternativeButtons(IScreenContext? currentScreen)
    {
        if (currentScreen is not Node rewardScreen || currentScreen.GetType().Name != "NCardRewardSelectionScreen")
        {
            return Array.Empty<Node>();
        }

        return ReflectionUtils.DescendantsByTypeName(rewardScreen, "NCardRewardAlternativeButton")
            .Where(node => GodotObject.IsInstanceValid(node) && ReflectionUtils.IsVisible(node))
            .ToArray();
    }

    public static string? ResolveRewardSourceScreen(object? currentScreen, RunState? runState)
    {
        return ResolveCurrentRoomScreenId(runState);
    }

    public static string? ResolveRewardSourceHint(object? currentScreen, RunState? runState)
    {
        var parts = new List<string>();
        var sourceScreen = ResolveRewardSourceScreen(currentScreen, runState);
        if (!string.IsNullOrWhiteSpace(sourceScreen))
        {
            parts.Add(sourceScreen!.ToLowerInvariant());
        }

        var screenTypeName = currentScreen?.GetType().Name;
        if (!string.IsNullOrWhiteSpace(screenTypeName))
        {
            parts.Add(screenTypeName!.ToLowerInvariant());
        }

        return parts.Count == 0 ? null : string.Join(":", parts);
    }

    public static NTreasureRoomRelicCollection? GetTreasureRelicCollection(IScreenContext? currentScreen)
    {
        if (currentScreen is NTreasureRoomRelicCollection relicCollection)
        {
            return relicCollection;
        }

        if (currentScreen is NTreasureRoom treasureRoom)
        {
            var nestedCollection = treasureRoom.GetNodeOrNull<NTreasureRoomRelicCollection>("%RelicCollection");
            if (nestedCollection != null &&
                GodotObject.IsInstanceValid(nestedCollection) &&
                nestedCollection.Visible)
            {
                return nestedCollection;
            }
        }

        return null;
    }

    public static NMerchantRoom? GetMerchantRoom(IScreenContext? currentScreen)
    {
        return currentScreen switch
        {
            NMerchantRoom room => room,
            NMerchantInventory => NMerchantRoom.Instance,
            _ => null
        };
    }

    public static NMerchantInventory? GetMerchantInventoryScreen(IScreenContext? currentScreen)
    {
        return currentScreen switch
        {
            NMerchantInventory inventory => inventory,
            NMerchantRoom room when room.Inventory != null => room.Inventory,
            _ => null
        };
    }

    public static MerchantInventory? GetMerchantInventory(IScreenContext? currentScreen)
    {
        return GetMerchantInventoryScreen(currentScreen)?.Inventory ?? GetMerchantRoom(currentScreen)?.Inventory?.Inventory;
    }

    public static IReadOnlyList<MerchantCardEntry> GetMerchantCardEntries(IScreenContext? currentScreen)
    {
        var inventory = GetMerchantInventory(currentScreen);
        if (inventory == null)
        {
            return Array.Empty<MerchantCardEntry>();
        }

        return inventory.CharacterCardEntries.Concat(inventory.ColorlessCardEntries).ToArray();
    }

    public static IReadOnlyList<MerchantRelicEntry> GetMerchantRelicEntries(IScreenContext? currentScreen)
    {
        return GetMerchantInventory(currentScreen)?.RelicEntries?.ToArray() ?? Array.Empty<MerchantRelicEntry>();
    }

    public static IReadOnlyList<MerchantPotionEntry> GetMerchantPotionEntries(IScreenContext? currentScreen)
    {
        return GetMerchantInventory(currentScreen)?.PotionEntries?.ToArray() ?? Array.Empty<MerchantPotionEntry>();
    }

    public static MerchantCardRemovalEntry? GetMerchantCardRemovalEntry(IScreenContext? currentScreen)
    {
        return GetMerchantInventory(currentScreen)?.CardRemovalEntry;
    }
}
