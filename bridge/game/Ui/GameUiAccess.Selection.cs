using Godot;
using MegaCrit.Sts2.Core.Nodes;
using MegaCrit.Sts2.Core.Nodes.Cards.Holders;
using MegaCrit.Sts2.Core.Nodes.CommonUi;
using MegaCrit.Sts2.Core.Nodes.Rooms;
using MegaCrit.Sts2.Core.Nodes.Screens.CardSelection;
using MegaCrit.Sts2.Core.Nodes.Screens.ScreenContext;
using MegaCrit.Sts2.Core.Runs;
using System.Text.RegularExpressions;

namespace Spire2Mind.Bridge.Game.Ui;

internal static partial class GameUiAccess
{
    public static bool IsDeckSelectionScreen(object? currentScreen)
    {
        return ResolveSelectionContext(currentScreen) != null;
    }

    private static bool IsDeckSelectionType(object? currentScreen)
    {
        return currentScreen != null && DeckSelectionScreenTypeNames.Contains(currentScreen.GetType().Name);
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

    public static IScreenContext? ResolveDeckSelectionScreen(object? currentScreen)
    {
        _ = ResolveDeckSelectionRoot(currentScreen, out var screenContext);
        return screenContext;
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
}
