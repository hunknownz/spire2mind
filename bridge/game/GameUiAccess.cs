using Godot;
using MegaCrit.Sts2.Core.Context;
using MegaCrit.Sts2.Core.Entities.Merchant;
using MegaCrit.Sts2.Core.Nodes;
using MegaCrit.Sts2.Core.Nodes.Cards.Holders;
using MegaCrit.Sts2.Core.Nodes.CommonUi;
using MegaCrit.Sts2.Core.Nodes.Rooms;
using MegaCrit.Sts2.Core.Nodes.Screens.CardSelection;
using MegaCrit.Sts2.Core.Nodes.Screens.CharacterSelect;
using MegaCrit.Sts2.Core.Nodes.Screens.GameOverScreen;
using MegaCrit.Sts2.Core.Nodes.Screens.MainMenu;
using MegaCrit.Sts2.Core.Nodes.Screens.Map;
using MegaCrit.Sts2.Core.Nodes.Screens.ScreenContext;
using MegaCrit.Sts2.Core.Nodes.Screens.Shops;
using MegaCrit.Sts2.Core.Nodes.Screens.TreasureRoomRelic;
using MegaCrit.Sts2.Core.Runs;
using System.Text.RegularExpressions;

namespace Spire2Mind.Bridge.Game;

internal static class GameUiAccess
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

    public static IScreenContext? GetOpenModal()
    {
        return NModalContainer.Instance?.OpenModal;
    }

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

    public static NMainMenu? GetMainMenu()
    {
        return NGame.Instance?.MainMenu;
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

    public static NMainMenuTextButton? GetMainMenuContinueButton(NMainMenu mainMenu)
    {
        return mainMenu.GetNodeOrNull<NMainMenuTextButton>("MainMenuTextButtons/ContinueButton");
    }

    public static NMainMenuTextButton? GetMainMenuAbandonRunButton(NMainMenu mainMenu)
    {
        return mainMenu.GetNodeOrNull<NMainMenuTextButton>("MainMenuTextButtons/AbandonRunButton");
    }

    public static NMainMenuTextButton? GetMainMenuSingleplayerButton(NMainMenu mainMenu)
    {
        return mainMenu.GetNodeOrNull<NMainMenuTextButton>("MainMenuTextButtons/SingleplayerButton");
    }

    public static NMainMenuTextButton? GetMainMenuMultiplayerButton(NMainMenu mainMenu)
    {
        return mainMenu.GetNodeOrNull<NMainMenuTextButton>("MainMenuTextButtons/MultiplayerButton");
    }

    public static NMainMenuTextButton? GetMainMenuTimelineButton(NMainMenu mainMenu)
    {
        return mainMenu.GetNodeOrNull<NMainMenuTextButton>("MainMenuTextButtons/TimelineButton");
    }

    public static NMainMenuTextButton? GetMainMenuSettingsButton(NMainMenu mainMenu)
    {
        return mainMenu.GetNodeOrNull<NMainMenuTextButton>("MainMenuTextButtons/SettingsButton");
    }

    public static NMainMenuTextButton? GetMainMenuProfileButton(NMainMenu mainMenu)
    {
        return mainMenu.GetNodeOrNull<NMainMenuTextButton>("MainMenuTextButtons/ProfileButton");
    }

    public static NMainMenuTextButton? GetMainMenuPatchNotesButton(NMainMenu mainMenu)
    {
        return mainMenu.GetNodeOrNull<NMainMenuTextButton>("MainMenuTextButtons/PatchNotesButton");
    }

    public static NMainMenuTextButton? GetMainMenuQuitButton(NMainMenu mainMenu)
    {
        return mainMenu.GetNodeOrNull<NMainMenuTextButton>("MainMenuTextButtons/QuitButton");
    }

    public static NCharacterSelectScreen? GetCharacterSelectScreen()
    {
        var mainMenu = GetMainMenu();
        if (mainMenu == null)
        {
            return null;
        }

        return FindDescendants<NCharacterSelectScreen>(mainMenu)
            .FirstOrDefault(node => GodotObject.IsInstanceValid(node) && node.IsVisibleInTree());
    }

    public static IReadOnlyList<NCharacterSelectButton> GetCharacterSelectButtons()
    {
        var screen = GetCharacterSelectScreen();
        if (screen == null)
        {
            return Array.Empty<NCharacterSelectButton>();
        }

        return FindDescendants<NCharacterSelectButton>(screen)
            .Where(node => GodotObject.IsInstanceValid(node))
            .OrderBy(node => node.GlobalPosition.Y)
            .ThenBy(node => node.GlobalPosition.X)
            .ToArray();
    }

    public static NConfirmButton? GetCharacterEmbarkButton()
    {
        return GetCharacterSelectScreen()?.GetNodeOrNull<NConfirmButton>("ConfirmButton");
    }

    public static Node? GetGameOverContinueButton(IScreenContext? currentScreen)
    {
        return currentScreen is NGameOverScreen screen
            ? screen.GetNodeOrNull<Node>("%ContinueButton")
            : null;
    }

    public static Node? GetGameOverMainMenuButton(IScreenContext? currentScreen)
    {
        return currentScreen is NGameOverScreen screen
            ? screen.GetNodeOrNull<Node>("%MainMenuButton")
            : null;
    }

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

    private static int GetNodeDepth(Node node)
    {
        var depth = 0;
        for (var current = node.GetParent(); current != null; current = current.GetParent())
        {
            depth++;
        }

        return depth;
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

    public static bool IsDeckSelectionScreen(object? currentScreen)
    {
        return ResolveSelectionContext(currentScreen) != null;
    }

    public static SelectionContextRef? ResolveSelectionContext(object? currentScreen)
    {
        var selectionRoot = ResolveDeckSelectionRoot(currentScreen, out var selectionScreenContext);
        if (selectionRoot != null)
        {
            return BuildDeckSelectionContext(selectionRoot, selectionScreenContext);
        }

        return ResolveCombatHandSelection(currentScreen as IScreenContext);
    }

    public static string? ResolveSelectionSourceScreen(SelectionContextRef selectionContext, RunState? runState)
    {
        if (selectionContext.IsCombatEmbedded)
        {
            return ScreenIds.Combat;
        }

        return ResolveCurrentRoomScreenId(runState);
    }

    public static string? ResolveSelectionSourceHint(SelectionContextRef selectionContext, RunState? runState)
    {
        var parts = new List<string>();
        var sourceScreen = ResolveSelectionSourceScreen(selectionContext, runState);
        if (!string.IsNullOrWhiteSpace(sourceScreen))
        {
            parts.Add(sourceScreen!.ToLowerInvariant());
        }

        if (!string.IsNullOrWhiteSpace(selectionContext.Kind))
        {
            parts.Add(selectionContext.Kind.ToLowerInvariant());
        }

        if (!string.IsNullOrWhiteSpace(selectionContext.Mode))
        {
            parts.Add(selectionContext.Mode!.ToLowerInvariant());
        }

        return parts.Count == 0 ? null : string.Join(":", parts);
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

    public static string? ResolveCurrentRoomScreenId(RunState? runState)
    {
        var currentRoom = ReflectionUtils.GetMemberValue(runState, "CurrentRoom");
        return ResolveRoomTypeToScreenId(currentRoom?.GetType().Name);
    }

    public static IScreenContext? ResolveDeckSelectionScreen(object? currentScreen)
    {
        _ = ResolveDeckSelectionRoot(currentScreen, out var screenContext);
        return screenContext;
    }

    public static IReadOnlyList<NCardHolder> GetDeckSelectionOptions(object? currentScreen)
    {
        var selectionContext = ResolveSelectionContext(currentScreen);
        if (selectionContext == null)
        {
            return Array.Empty<NCardHolder>();
        }

        if (selectionContext.IsCombatEmbedded)
        {
            return ReflectionUtils.Enumerate(ReflectionUtils.GetMemberValue(selectionContext.Root, "ActiveHolders"))
                .OfType<NCardHolder>()
                .Where(node => GodotObject.IsInstanceValid(node) && ReflectionUtils.IsVisible(node) && node.CardModel != null)
                .OrderBy(node => node.GetIndex())
                .ToArray();
        }

        return GetDeckSelectionOptionHolders(selectionContext.Root);
    }

    public static string? GetDeckSelectionPrompt(object? currentScreen)
    {
        var selectionContext = ResolveSelectionContext(currentScreen);
        if (selectionContext == null)
        {
            return null;
        }

        return GetDeckSelectionPrompt(selectionContext.Root, selectionContext.IsCombatEmbedded);
    }

    private static string? GetDeckSelectionPrompt(Node selectionRoot, bool isCombatEmbedded)
    {
        if (isCombatEmbedded)
        {
            var header = selectionRoot.GetNodeOrNull<Node>("%SelectionHeader")
                ?? selectionRoot.GetNodeOrNull<Node>("SelectionHeader");
            var headerText = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(header, "Text", "Label"));
            if (!string.IsNullOrWhiteSpace(headerText))
            {
                return headerText;
            }
        }

        var prompt = ReflectionUtils.LocalizedText(
            ReflectionUtils.GetMemberValue(selectionRoot, "_title", "Title", "Prompt"));
        if (!string.IsNullOrWhiteSpace(prompt))
        {
            return prompt;
        }

        var bottomLabel = selectionRoot.GetNodeOrNull<Node>("%BottomLabel");
        prompt = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(bottomLabel, "Text", "Label"));
        if (!string.IsNullOrWhiteSpace(prompt))
        {
            return prompt;
        }

        var banner = selectionRoot.GetNodeOrNull<Node>("Banner") ?? selectionRoot.GetNodeOrNull<Node>("%Banner");
        prompt = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(ReflectionUtils.GetMemberValue(banner, "label", "Label"), "Text", "Label"));
        if (!string.IsNullOrWhiteSpace(prompt))
        {
            return prompt;
        }

        return ReflectionUtils.Descendants(selectionRoot)
            .Where(node => GodotObject.IsInstanceValid(node) && ReflectionUtils.IsVisible(node))
            .Select(node =>
                ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(node, "Text", "Label")) ??
                ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(ReflectionUtils.GetMemberValue(node, "label", "Label"), "Text", "Label")))
            .FirstOrDefault(text => !string.IsNullOrWhiteSpace(text));
    }

    public static bool DeckSelectionRequiresConfirmation(object? currentScreen)
    {
        var selectionContext = ResolveSelectionContext(currentScreen);
        return selectionContext?.RequiresConfirmation == true ||
               selectionContext?.ScreenContext is NCardGridSelectionScreen;
    }

    public static bool TryGetDeckSelectionConfirmButton(object? currentScreen, out Node? confirmButton)
    {
        confirmButton = null;
        var selectionContext = ResolveSelectionContext(currentScreen);
        if (selectionContext == null)
        {
            return false;
        }

        return TryGetDeckSelectionConfirmButton(selectionContext, out confirmButton);
    }

    private static bool TryGetDeckSelectionConfirmButton(SelectionContextRef selectionContext, out Node? confirmButton)
    {
        confirmButton = null;
        if (selectionContext.IsCombatEmbedded)
        {
            confirmButton = GetCombatHandConfirmButton(selectionContext.Root);
            return confirmButton != null && ReflectionUtils.IsAvailable(confirmButton);
        }

        if (selectionContext.ScreenContext is NSimpleCardSelectScreen)
        {
            return false;
        }

        switch (selectionContext?.ScreenContext)
        {
            case NDeckUpgradeSelectScreen upgradeScreen:
                if (TryGetDeckUpgradeConfirmButton(upgradeScreen, out var upgradeConfirm))
                {
                    confirmButton = upgradeConfirm;
                    return true;
                }
                break;

            case NDeckEnchantSelectScreen enchantScreen:
                if (TryGetDeckEnchantConfirmButton(enchantScreen, out var enchantConfirm))
                {
                    confirmButton = enchantConfirm;
                    return true;
                }
                break;

            case NDeckTransformSelectScreen transformScreen:
                if (TryGetDeckTransformConfirmButton(transformScreen, out var transformConfirm))
                {
                    confirmButton = transformConfirm;
                    return true;
                }
                break;

            case NCardGridSelectionScreen gridScreen:
                var previewContainer = gridScreen.GetNodeOrNull<Control>("%PreviewContainer");
                var previewConfirm = gridScreen.GetNodeOrNull<NConfirmButton>("%PreviewConfirm")
                    ?? previewContainer?.GetNodeOrNull<NConfirmButton>("Confirm");
                if (previewContainer?.Visible == true && previewConfirm?.IsEnabled == true)
                {
                    confirmButton = previewConfirm;
                    return true;
                }

                var directConfirm = gridScreen.GetNodeOrNull<NConfirmButton>("%Confirm")
                    ?? gridScreen.GetNodeOrNull<NConfirmButton>("Confirm");
                if (directConfirm?.IsEnabled == true)
                {
                    confirmButton = directConfirm;
                    return true;
                }
                break;
        }

        confirmButton = FindAnySelectionConfirmButton(selectionContext.Root!);
        return confirmButton != null;
    }

    public static IReadOnlyList<NGridCardHolder> GetVisibleGridCardHolders(Node? root)
    {
        if (root == null)
        {
            return Array.Empty<NGridCardHolder>();
        }

        return FindDescendants<NGridCardHolder>(root)
            .Where(node => GodotObject.IsInstanceValid(node) && node.IsVisibleInTree() && node.CardModel != null)
            .OrderBy(node => node.GlobalPosition.Y)
            .ThenBy(node => node.GlobalPosition.X)
            .ToArray();
    }

    private static SelectionContextRef? ResolveCombatHandSelection(IScreenContext? currentScreen)
    {
        if (currentScreen is not NCombatRoom combatRoom)
        {
            return null;
        }

        var handNode = ReflectionUtils.GetMemberValue(ReflectionUtils.GetMemberValue(combatRoom, "Ui"), "Hand") as Node;
        if (handNode == null || !GodotObject.IsInstanceValid(handNode) || !ReflectionUtils.IsVisible(handNode))
        {
            return null;
        }

        if (ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(handNode, "IsInCardSelection")) != true)
        {
            return null;
        }

        var mode = ReflectionUtils.GetMemberValue(handNode, "CurrentMode")?.ToString();
        if (!string.Equals(mode, "SimpleSelect", StringComparison.Ordinal) &&
            !string.Equals(mode, "UpgradeSelect", StringComparison.Ordinal))
        {
            return null;
        }

        var prefs = ReflectionUtils.GetMemberValue(handNode, "_prefs");
        var requiresConfirmation = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(prefs, "RequireManualConfirmation")) == true;
        var confirmButton = GetCombatHandConfirmButton(handNode);
        var selectedCount = CountSelectedCombatHandCards(handNode);

        return new SelectionContextRef
        {
            Kind = string.Equals(mode, "UpgradeSelect", StringComparison.Ordinal)
                ? "combat_hand_upgrade_select"
                : "combat_hand_select",
            Root = handNode,
            ScreenContext = combatRoom,
            IsCombatEmbedded = true,
            Mode = mode,
            MinSelect = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(prefs, "MinSelect")) ?? 1,
            MaxSelect = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(prefs, "MaxSelect")) ?? 1,
            SelectedCount = selectedCount,
            RequiresConfirmation = requiresConfirmation,
            CanConfirm = requiresConfirmation && confirmButton != null && ReflectionUtils.IsAvailable(confirmButton)
        };
    }

    private static int CountSelectedCombatHandCards(Node handNode)
    {
        var selectedCards = ReflectionUtils.GetMemberValue(handNode, "_selectedCards");
        return selectedCards switch
        {
            System.Collections.ICollection collection => collection.Count,
            _ => ReflectionUtils.Enumerate(selectedCards).Count()
        };
    }

    private static Node? GetCombatHandConfirmButton(Node handNode)
    {
        return handNode.GetNodeOrNull<Node>("%SelectModeConfirmButton")
            ?? handNode.GetNodeOrNull<Node>("SelectModeConfirmButton");
    }

    private static bool IsDeckSelectionType(object? currentScreen)
    {
        return currentScreen != null && DeckSelectionScreenTypeNames.Contains(currentScreen.GetType().Name);
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

    private static Node? ResolveDeckSelectionRoot(object? currentScreen, out IScreenContext? screenContext)
    {
        screenContext = currentScreen as IScreenContext;

        if (currentScreen is Node currentNode &&
            IsDeckSelectionCandidate(currentNode))
        {
            return currentNode;
        }

        var root = NGame.Instance?.RootSceneContainer?.CurrentScene ?? (Node?)NGame.Instance;
        if (root == null)
        {
            return null;
        }

        var selectionNode = ReflectionUtils.Descendants(root)
            .OfType<Node>()
            .Where(node => GodotObject.IsInstanceValid(node) && ReflectionUtils.IsVisible(node))
            .Where(IsDeckSelectionCandidate)
            .OrderByDescending(GetDeckSelectionPriority)
            .ThenByDescending(GetNodeDepth)
            .FirstOrDefault();

        screenContext = selectionNode as IScreenContext;
        return selectionNode;
    }

    private static bool IsDeckSelectionCandidate(Node node)
    {
        if (NonDeckGridSelectionTypeNames.Contains(node.GetType().Name))
        {
            return false;
        }

        if (IsDeckSelectionType(node))
        {
            return true;
        }

        return GetVisibleGridCardHolders(node).Count > 0;
    }

    private static int GetDeckSelectionPriority(Node node)
    {
        if (IsDeckSelectionType(node))
        {
            return 100;
        }

        return GetVisibleGridCardHolders(node).Count > 0 ? 50 : 0;
    }

    private static string ResolveDeckSelectionKind(IScreenContext? screenContext, Node selectionRoot)
    {
        if (screenContext != null)
        {
            return screenContext.GetType().Name;
        }

        return "grid_card_selection";
    }

    private static bool TryGetDeckUpgradeConfirmButton(NDeckUpgradeSelectScreen screen, out NConfirmButton? confirmButton)
    {
        var singlePreview = screen.GetNodeOrNull<Control>("%UpgradeSinglePreviewContainer");
        if (singlePreview?.Visible == true)
        {
            confirmButton = singlePreview.GetNodeOrNull<NConfirmButton>("Confirm");
            return confirmButton?.IsEnabled == true;
        }

        var multiPreview = screen.GetNodeOrNull<Control>("%UpgradeMultiPreviewContainer");
        if (multiPreview?.Visible == true)
        {
            confirmButton = multiPreview.GetNodeOrNull<NConfirmButton>("Confirm");
            return confirmButton?.IsEnabled == true;
        }

        confirmButton = null;
        return false;
    }

    private static bool TryGetDeckTransformConfirmButton(NDeckTransformSelectScreen screen, out NConfirmButton? confirmButton)
    {
        var previewContainer = screen.GetNodeOrNull<Control>("%PreviewContainer");
        if (previewContainer?.Visible == true)
        {
            confirmButton = previewContainer.GetNodeOrNull<NConfirmButton>("Confirm");
            if (confirmButton?.IsEnabled == true)
            {
                return true;
            }
        }

        confirmButton = screen.GetNodeOrNull<NConfirmButton>("%Confirm")
            ?? screen.GetNodeOrNull<NConfirmButton>("Confirm");
        if (confirmButton?.IsEnabled == true)
        {
            return true;
        }

        confirmButton = FindAnySelectionConfirmButton(screen) as NConfirmButton;
        return confirmButton?.IsEnabled == true;
    }

    private static bool TryGetDeckEnchantConfirmButton(NDeckEnchantSelectScreen screen, out NConfirmButton? confirmButton)
    {
        var singlePreview = screen.GetNodeOrNull<Control>("%EnchantSinglePreviewContainer");
        if (singlePreview?.Visible == true)
        {
            confirmButton = singlePreview.GetNodeOrNull<NConfirmButton>("Confirm");
            return confirmButton?.IsEnabled == true;
        }

        var multiPreview = screen.GetNodeOrNull<Control>("%EnchantMultiPreviewContainer");
        if (multiPreview?.Visible == true)
        {
            confirmButton = multiPreview.GetNodeOrNull<NConfirmButton>("Confirm");
            return confirmButton?.IsEnabled == true;
        }

        confirmButton = null;
        return false;
    }

    private static SelectionContextRef BuildDeckSelectionContext(Node selectionRoot, IScreenContext? screenContext)
    {
        var prompt = GetDeckSelectionPrompt(selectionRoot, false);
        var options = GetDeckSelectionOptionHolders(selectionRoot);
        var selectedCount = ResolveDeckSelectionSelectedCount(selectionRoot, screenContext, options);
        var requiresConfirmation = ResolveDeckSelectionRequiresConfirmation(selectionRoot, screenContext);
        var canConfirm = TryGetDeckSelectionConfirmButton(
            new SelectionContextRef
            {
                Kind = ResolveDeckSelectionKind(screenContext, selectionRoot),
                Root = selectionRoot,
                ScreenContext = screenContext,
                IsCombatEmbedded = false
            },
            out _);

        var inferredCount = InferSelectionCountFromPrompt(prompt);
        var minSelect = ResolveDeckSelectionCount(selectionRoot, screenContext, inferredCount, "MinSelect", "_minSelect", "RequiredSelectionCount", "SelectionCount", "_selectionCount");
        var maxSelect = ResolveDeckSelectionCount(selectionRoot, screenContext, inferredCount, "MaxSelect", "_maxSelect", "RequiredSelectionCount", "SelectionCount", "_selectionCount");

        if (minSelect == null)
        {
            minSelect = inferredCount ?? 1;
        }
        if (maxSelect == null)
        {
            maxSelect = inferredCount ?? minSelect ?? 1;
        }
        if (maxSelect < minSelect)
        {
            maxSelect = minSelect;
        }

        return new SelectionContextRef
        {
            Kind = ResolveDeckSelectionKind(screenContext, selectionRoot),
            Root = selectionRoot,
            ScreenContext = screenContext,
            IsCombatEmbedded = false,
            Prompt = prompt,
            MinSelect = minSelect,
            MaxSelect = maxSelect,
            SelectedCount = selectedCount,
            RequiresConfirmation = requiresConfirmation,
            CanConfirm = canConfirm
        };
    }

    private static IReadOnlyList<NCardHolder> GetDeckSelectionOptionHolders(Node selectionRoot)
    {
        var visibleGridHolders = GetVisibleGridCardHolders(selectionRoot);
        if (visibleGridHolders.Count > 0)
        {
            return visibleGridHolders
                .Cast<NCardHolder>()
                .ToArray();
        }

        return FindDescendants<NCardHolder>(selectionRoot)
            .Where(node => GodotObject.IsInstanceValid(node) && node.IsVisibleInTree() && node.CardModel != null)
            .OrderBy(node => node.GlobalPosition.Y)
            .ThenBy(node => node.GlobalPosition.X)
            .ToArray();
    }

    private static int ResolveDeckSelectionSelectedCount(Node selectionRoot, IScreenContext? screenContext, IReadOnlyList<NCardHolder> options)
    {
        var explicitCount = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(screenContext, "SelectedCount", "_selectedCount", "SelectionCount"))
            ?? ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(selectionRoot, "SelectedCount", "_selectedCount", "SelectionCount"));
        if (explicitCount != null)
        {
            return explicitCount.Value;
        }

        var selectedCollections = new[]
        {
            ReflectionUtils.GetMemberValue(screenContext, "SelectedCards", "_selectedCards", "SelectedCardHolders", "_selectedCardHolders", "SelectedHolders", "_selectedHolders", "CurrentSelection", "_currentSelection"),
            ReflectionUtils.GetMemberValue(selectionRoot, "SelectedCards", "_selectedCards", "SelectedCardHolders", "_selectedCardHolders", "SelectedHolders", "_selectedHolders", "CurrentSelection", "_currentSelection")
        };

        foreach (var selectedCollection in selectedCollections)
        {
            var count = TryCountSelectionCollection(selectedCollection);
            if (count != null)
            {
                return count.Value;
            }
        }

        return options.Count(holder => ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(holder, "IsSelected", "_isSelected", "Selected", "_selected")) == true);
    }

    private static int? TryCountSelectionCollection(object? collection)
    {
        if (collection == null)
        {
            return null;
        }

        if (collection is System.Collections.ICollection typedCollection)
        {
            return typedCollection.Count;
        }

        var items = ReflectionUtils.Enumerate(collection).ToList();
        return items.Count > 0 ? items.Count : null;
    }

    private static bool ResolveDeckSelectionRequiresConfirmation(Node selectionRoot, IScreenContext? screenContext)
    {
        if (screenContext is NSimpleCardSelectScreen)
        {
            return false;
        }

        var explicitValue = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(screenContext, "RequiresConfirmation", "_requiresConfirmation", "RequireManualConfirmation"))
            ?? ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(selectionRoot, "RequiresConfirmation", "_requiresConfirmation", "RequireManualConfirmation"));
        if (explicitValue != null)
        {
            return explicitValue == true;
        }

        return screenContext is NDeckUpgradeSelectScreen
            or NDeckEnchantSelectScreen
            or NDeckTransformSelectScreen
            or NCardGridSelectionScreen;
    }

    private static int? ResolveDeckSelectionCount(Node selectionRoot, IScreenContext? screenContext, int? promptCount, params string[] memberNames)
    {
        var explicitCount = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(screenContext, memberNames))
            ?? ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(selectionRoot, memberNames));

        if (explicitCount != null && explicitCount > 0)
        {
            return explicitCount;
        }

        return promptCount > 0 ? promptCount : null;
    }

    private static int? InferSelectionCountFromPrompt(string? prompt)
    {
        if (string.IsNullOrWhiteSpace(prompt))
        {
            return null;
        }

        var match = Regex.Match(prompt, "(\\d+)");
        if (!match.Success)
        {
            return null;
        }

        return int.TryParse(match.Groups[1].Value, out var count) && count > 0 ? count : null;
    }

    private static Node? FindAnySelectionConfirmButton(Node? root)
    {
        if (root == null)
        {
            return null;
        }

        return ReflectionUtils.Descendants(root)
            .Where(node => GodotObject.IsInstanceValid(node) && ReflectionUtils.IsAvailable(node))
            .OfType<NConfirmButton>()
            .OrderBy(node => node.GetPath().ToString().Length)
            .FirstOrDefault();
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

    public static Node? GetProceedButton(IScreenContext? currentScreen)
    {
        if (currentScreen == null || currentScreen.GetType().Name == "NCardRewardSelectionScreen")
        {
            return null;
        }

        if (currentScreen.GetType().Name == "NRewardsScreen")
        {
            var rewardProceedButton = GetRewardProceedButton(currentScreen);
            return IsProceedButtonUsable(rewardProceedButton) ? rewardProceedButton : null;
        }

        TryShowRestProceedButtonIfComplete(currentScreen);
        TryShowTreasureProceedButtonIfComplete(currentScreen);

        if (currentScreen is IRoomWithProceedButton roomWithProceedButton &&
            IsProceedButtonUsable(roomWithProceedButton.ProceedButton))
        {
            return roomWithProceedButton.ProceedButton;
        }

        var directProceedButton = ReflectionUtils.GetMemberValue(currentScreen, "ProceedButton", "_proceedButton");
        if (IsProceedButtonUsable(directProceedButton as Node))
        {
            return directProceedButton as Node;
        }

        if (currentScreen is not Node rootNode)
        {
            return null;
        }

        return ReflectionUtils.DescendantsByTypeName(rootNode, "NProceedButton").FirstOrDefault(IsProceedButtonUsable);
    }

    public static Node? GetModalConfirmButton()
    {
        return FindModalButton(
            ConfirmKeywords,
            "VerticalPopup/YesButton",
            "ConfirmButton",
            "RightArrow",
            "%ConfirmButton",
            "%Confirm",
            "%AcknowledgeButton",
            "%RightArrow",
            "%OkButton");
    }

    public static Node? GetModalCancelButton()
    {
        return FindModalButton(
            DismissKeywords,
            "VerticalPopup/NoButton",
            "CancelButton",
            "LeftArrow",
            "%CancelButton",
            "%BackButton",
            "%LeftArrow",
            "%CloseButton");
    }

    private static Node? FindModalButton(string[] keywords, params string[] paths)
    {
        if (GetOpenModal() is not Node modalNode)
        {
            return null;
        }

        foreach (var path in paths)
        {
            var button = modalNode.GetNodeOrNull<Node>(path);
            if (button != null && GodotObject.IsInstanceValid(button) && ReflectionUtils.IsAvailable(button))
            {
                return button;
            }
        }

        var directControl = keywords == ConfirmKeywords
            ? ReflectionUtils.GetMemberValue(modalNode, "_nextButton", "NextButton", "_confirmButton", "ConfirmButton", "_yesButton", "YesButton")
            : ReflectionUtils.GetMemberValue(modalNode, "_prevButton", "PrevButton", "_dismissButton", "DismissButton", "_cancelButton", "CancelButton", "_noButton", "NoButton");
        if (directControl is Node directNode &&
            GodotObject.IsInstanceValid(directNode) &&
            ReflectionUtils.IsAvailable(directNode))
        {
            return directNode;
        }

        var candidates = ReflectionUtils.Descendants(modalNode)
            .Where(candidate =>
                GodotObject.IsInstanceValid(candidate) &&
                ReflectionUtils.IsAvailable(candidate) &&
                IsButtonLike(candidate))
            .ToArray();

        var matched = candidates
            .Where(candidate => MatchesKeywords(candidate, keywords))
            .OrderBy(candidate => candidate.GetPath().ToString().Length)
            .FirstOrDefault();

        if (matched != null)
        {
            return matched;
        }

        if (keywords == ConfirmKeywords && candidates.Length == 1)
        {
            return candidates[0];
        }

        return null;
    }

    private static bool IsButtonLike(Node candidate)
    {
        return candidate is BaseButton || candidate.GetType().Name.Contains("Button", StringComparison.OrdinalIgnoreCase);
    }

    private static bool MatchesKeywords(Node candidate, IEnumerable<string> keywords)
    {
        var haystacks = new[]
        {
            candidate.Name.ToString(),
            candidate.GetType().Name,
            ReflectionUtils.LocalizedText(candidate),
            ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(candidate, "Text", "Label", "Title"))
        };

        return haystacks
            .Where(value => !string.IsNullOrWhiteSpace(value))
            .Select(value => value!.ToLowerInvariant())
            .Any(value => keywords.Any(value.Contains));
    }

    private static bool IsProceedButtonUsable(Node? button)
    {
        return button != null &&
               GodotObject.IsInstanceValid(button) &&
               ReflectionUtils.IsAvailable(button);
    }

    private static void TryShowRestProceedButtonIfComplete(IScreenContext? currentScreen)
    {
        if (currentScreen is not NRestSiteRoom restSiteRoom)
        {
            return;
        }

        if (IsProceedButtonUsable(restSiteRoom.ProceedButton))
        {
            return;
        }

        try
        {
            var synchronizer = ReflectionUtils.GetMemberValue(RunManager.Instance, "RestSiteSynchronizer");
            var options = ReflectionUtils.Enumerate(ReflectionUtils.InvokeMethod(synchronizer, "GetLocalOptions")).ToList();
            if (options.Count != 0)
            {
                return;
            }

            restSiteRoom.Call(NRestSiteRoom.MethodName.ShowProceedButton);
            ActiveScreenContext.Instance.Update();
        }
        catch
        {
            // Best-effort UI sync only; fall back to existing proceed button discovery.
        }
    }

    private static void TryShowTreasureProceedButtonIfComplete(IScreenContext? currentScreen)
    {
        if (currentScreen is not NTreasureRoom treasureRoom)
        {
            return;
        }

        if (currentScreen is IRoomWithProceedButton roomWithProceedButton &&
            IsProceedButtonUsable(roomWithProceedButton.ProceedButton))
        {
            return;
        }

        var chestButton = treasureRoom.GetNodeOrNull<Node>("%Chest");
        var isOpened = chestButton == null || !GodotObject.IsInstanceValid(chestButton) || ReflectionUtils.IsEnabled(chestButton) == false;
        if (!isOpened)
        {
            return;
        }

        if (GetTreasureRelicCollection(currentScreen) != null)
        {
            return;
        }

        try
        {
            var synchronizer = ReflectionUtils.GetMemberValue(RunManager.Instance, "TreasureRoomRelicSynchronizer");
            var currentRelics = ReflectionUtils.Enumerate(ReflectionUtils.GetMemberValue(synchronizer, "CurrentRelics")).ToList();
            if (currentRelics.Count > 0)
            {
                return;
            }

            ReflectionUtils.InvokeMethod(treasureRoom, "ShowProceedButton");
            treasureRoom.Call("ShowProceedButton");
            ActiveScreenContext.Instance.Update();
        }
        catch
        {
            // Best-effort UI sync only; fall back to existing proceed button discovery.
        }
    }
}
