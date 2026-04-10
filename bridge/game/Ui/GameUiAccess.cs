using Godot;
using MegaCrit.Sts2.Core.Context;
using MegaCrit.Sts2.Core.Nodes;
using MegaCrit.Sts2.Core.Nodes.CommonUi;
using MegaCrit.Sts2.Core.Nodes.Rooms;
using MegaCrit.Sts2.Core.Nodes.Screens.MainMenu;
using MegaCrit.Sts2.Core.Nodes.Screens.Map;
using MegaCrit.Sts2.Core.Nodes.Screens.ScreenContext;
using MegaCrit.Sts2.Core.Nodes.Screens.Shops;
using MegaCrit.Sts2.Core.Nodes.Screens.TreasureRoomRelic;
using MegaCrit.Sts2.Core.Runs;

namespace Spire2Mind.Bridge.Game.Ui;

internal static partial class GameUiAccess
{
    private static readonly HashSet<string> DeckSelectionScreenTypeNames = new(StringComparer.Ordinal)
    {
        "NSimpleCardSelectScreen",
        "NChooseACardSelectionScreen",
        "NCardGridSelectionScreen",
        "NDeckCardSelectScreen",
        "NDeckUpgradeSelectScreen",
        "NDeckEnchantSelectScreen",
        "NDeckTransformSelectScreen"
    };

    private static readonly HashSet<string> NonDeckGridSelectionTypeNames = new(StringComparer.Ordinal)
    {
        "NRewardsScreen",
        "NCardRewardSelectionScreen"
    };

    private static readonly string[] ConfirmKeywords =
    {
        "yes",
        "confirm",
        "ok",
        "accept",
        "continue",
        "acknowledge",
        "proceed",
        "start",
        "next",
        "right"
    };

    private static readonly string[] DismissKeywords =
    {
        "no",
        "cancel",
        "back",
        "close",
        "dismiss",
        "skip",
        "prev",
        "previous",
        "left"
    };

    public static IEnumerable<TNode> FindDescendants<TNode>(Node? root) where TNode : Node
    {
        if (root == null)
        {
            yield break;
        }

        foreach (Node child in root.GetChildren())
        {
            if (child is TNode typed)
            {
                yield return typed;
            }

            foreach (var descendant in FindDescendants<TNode>(child))
            {
                yield return descendant;
            }
        }
    }

    public static IScreenContext? GetOpenModal()
    {
        return NModalContainer.Instance?.OpenModal;
    }

    public static object? GetLocalPlayer(RunState? runState)
    {
        if (runState == null)
        {
            return null;
        }

        var players = ReflectionUtils.Enumerate(ReflectionUtils.GetMemberValue(runState, "Players"));
        if (!LocalContext.NetId.HasValue)
        {
            return players.FirstOrDefault();
        }

        return players.FirstOrDefault(player => ReflectionUtils.GetMemberValue<ulong?>(player, "NetId") == LocalContext.NetId.Value)
               ?? players.FirstOrDefault();
    }

    public static bool HasEmptyPotionSlot(RunState? runState)
    {
        var localPlayer = GetLocalPlayer(runState);
        if (localPlayer == null)
        {
            return false;
        }

        return ReflectionUtils.Enumerate(ReflectionUtils.GetMemberValue(localPlayer, "PotionSlots")).Any(slot => slot == null);
    }

    public static NMainMenu? GetMainMenu()
    {
        return NGame.Instance?.MainMenu;
    }

    private static IScreenContext? FindVisibleGameplayScreen()
    {
        var root = NGame.Instance?.RootSceneContainer?.CurrentScene ?? (Node?)NGame.Instance;
        if (root == null)
        {
            return null;
        }

        return ReflectionUtils.Descendants(root)
            .Where(node => GodotObject.IsInstanceValid(node) && ReflectionUtils.IsVisible(node) && node is IScreenContext)
            .OrderByDescending(GetGameplayScreenPriority)
            .ThenByDescending(GetNodeDepth)
            .Cast<IScreenContext>()
            .FirstOrDefault(screen => GetGameplayScreenPriority((Node)screen) > 0);
    }

    private static int GetGameplayScreenPriority(Node node)
    {
        return node.GetType().Name switch
        {
            "NCombatRoom" => 100,
            "NRewardsScreen" => 95,
            "NCardRewardSelectionScreen" => 90,
            "NEventRoom" => 80,
            "NMerchantInventory" => 75,
            "NMerchantRoom" => 70,
            "NRestSiteRoom" => 60,
            "NTreasureRoomRelicCollection" => 55,
            "NTreasureRoom" => 50,
            "NGameOverScreen" => 40,
            _ => 0
        };
    }

    private static int GetNodeDepth(Node node)
    {
        var depth = 0;
        for (var current = node.GetParent(); current != null; current = current.GetParent())
        {
            depth++;
        }

        return depth;
    }

    public static object? ResolveGameplayScreen(object? currentScreen, RunState? runState)
    {
        var actionableMapScreen = GetActionableMapScreen(runState);
        if (actionableMapScreen != null && ShouldPreferActionableMapScreen(currentScreen))
        {
            return actionableMapScreen;
        }

        if (currentScreen != null && currentScreen is not (NMapScreen or NMapRoom))
        {
            return currentScreen;
        }

        var visibleGameplayScreen = FindVisibleGameplayScreen();
        if (visibleGameplayScreen != null)
        {
            return visibleGameplayScreen;
        }

        var currentRoom = ReflectionUtils.GetMemberValue(runState, "CurrentRoom");
        if (currentRoom == null)
        {
            return currentScreen;
        }

        var typeName = currentRoom.GetType().Name;
        return typeName switch
        {
            "NCombatRoom" or "CombatRoom" => currentRoom,
            "NEventRoom" or "EventRoom" => currentRoom,
            "NMerchantRoom" or "MerchantRoom" => currentRoom,
            "NRestSiteRoom" or "RestSiteRoom" => currentRoom,
            "NTreasureRoom" or "TreasureRoom" => currentRoom,
            "NGameOverScreen" or "GameOverScreen" => currentRoom,
            _ => currentScreen
        };
    }

    private static bool ShouldPreferActionableMapScreen(object? currentScreen)
    {
        return currentScreen == null || currentScreen switch
        {
            NMapScreen or
            NMapRoom or
            NTreasureRoom or
            NTreasureRoomRelicCollection or
            NCombatRoom or
            NMerchantRoom or
            NMerchantInventory or
            NRestSiteRoom or
            NEventRoom => true,
            _ => false
        };
    }

    public static string? ResolveCurrentRoomScreenId(RunState? runState)
    {
        var currentRoom = ReflectionUtils.GetMemberValue(runState, "CurrentRoom");
        return ResolveRoomTypeToScreenId(currentRoom?.GetType().Name);
    }

    private static string? ResolveRoomTypeToScreenId(string? typeName)
    {
        return typeName switch
        {
            "NCombatRoom" or "CombatRoom" => ScreenIds.Combat,
            "NEventRoom" or "EventRoom" => ScreenIds.Event,
            "NMerchantRoom" or "MerchantRoom" => ScreenIds.Shop,
            "NRestSiteRoom" or "RestSiteRoom" => ScreenIds.Rest,
            "NTreasureRoom" or "TreasureRoom" => ScreenIds.Chest,
            "NGameOverScreen" or "GameOverScreen" => ScreenIds.GameOver,
            "NMapScreen" or "NMapRoom" or "MapRoom" => ScreenIds.Map,
            _ => null
        };
    }
}
