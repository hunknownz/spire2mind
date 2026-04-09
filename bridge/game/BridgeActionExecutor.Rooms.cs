using Godot;
using MegaCrit.Sts2.Core.Entities.Merchant;
using MegaCrit.Sts2.Core.Entities.Players;
using MegaCrit.Sts2.Core.Logging;
using MegaCrit.Sts2.Core.Nodes.Cards.Holders;
using MegaCrit.Sts2.Core.Nodes.Rooms;
using MegaCrit.Sts2.Core.Nodes.Screens.CardSelection;
using MegaCrit.Sts2.Core.Nodes.Screens.Map;
using MegaCrit.Sts2.Core.Nodes.Screens.ScreenContext;
using MegaCrit.Sts2.Core.Nodes.Screens.Shops;
using MegaCrit.Sts2.Core.Runs;
using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Game;

internal static partial class BridgeActionExecutor
{
    private static async Task<BridgeActionResult> ExecuteChooseMapNodeAsync(BridgeActionRequest request)
    {
        EnsureActionAvailable(ActionIds.ChooseMapNode);

        var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        var availableNodes = GameUiAccess.GetAvailableMapNodes(currentScreen, RunManager.Instance.DebugOnlyGetState());
        var optionIndex = RequireOptionIndex(ActionIds.ChooseMapNode, request.OptionIndex, availableNodes.Count);
        var node = availableNodes[optionIndex];

        node.ForceClick();
        var stable = await WaitForStableSnapshotAsync(
            snapshot => snapshot.Screen != ScreenIds.Map && IsSnapshotActionableOrSettled(snapshot),
            TimeSpan.FromSeconds(12));

        return BuildResult(ActionIds.ChooseMapNode, stable);
    }

    private static async Task<BridgeActionResult> ExecuteClaimRewardAsync(BridgeActionRequest request)
    {
        EnsureActionAvailable(ActionIds.ClaimReward);

        var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        var rewardButtons = GameUiAccess.GetRewardButtons(currentScreen);
        var optionIndex = RequireOptionIndex(ActionIds.ClaimReward, request.OptionIndex, rewardButtons.Count);
        var button = rewardButtons[optionIndex];
        var previousCount = rewardButtons.Count(ReflectionUtils.IsAvailable);

        ClickControl(button);
        var stable = await WaitUntilAsync(
            () =>
            {
                var snapshot = StateSnapshotBuilder.Build();
                if (!string.Equals(snapshot.Screen, ScreenIds.Reward, StringComparison.Ordinal) ||
                    !snapshot.AvailableActions.Contains(ActionIds.ClaimReward, StringComparer.Ordinal))
                {
                    return true;
                }

                var screen = ActiveScreenContext.Instance.GetCurrentScreen();
                if (!ReferenceEquals(screen, currentScreen))
                {
                    return true;
                }

                return GameUiAccess.GetRewardButtons(screen).Count(ReflectionUtils.IsAvailable) != previousCount;
            },
            TimeSpan.FromSeconds(10));

        return BuildResult(ActionIds.ClaimReward, stable);
    }

    private static async Task<BridgeActionResult> ExecuteChooseRewardCardAsync(BridgeActionRequest request)
    {
        EnsureActionAvailable(ActionIds.ChooseRewardCard);

        var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        var options = GameUiAccess.GetCardRewardOptions(currentScreen);
        var optionIndex = RequireOptionIndex(ActionIds.ChooseRewardCard, request.OptionIndex, options.Count);
        var selected = options[optionIndex];
        var previousCount = options.Count;

        selected.EmitSignal(NCardHolder.SignalName.Pressed, selected);
        var stable = await WaitUntilAsync(
            () =>
            {
                var screen = ActiveScreenContext.Instance.GetCurrentScreen();
                if (!ReferenceEquals(screen, currentScreen))
                {
                    return true;
                }

                return GameUiAccess.GetCardRewardOptions(screen).Count != previousCount;
            },
            TimeSpan.FromSeconds(10));

        return BuildResult(ActionIds.ChooseRewardCard, stable);
    }

    private static async Task<BridgeActionResult> ExecuteSkipRewardCardsAsync()
    {
        EnsureActionAvailable(ActionIds.SkipRewardCards);

        var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        var button = GameUiAccess.GetCardRewardAlternativeButtons(currentScreen).FirstOrDefault()
            ?? throw StateUnavailable(ActionIds.SkipRewardCards, "Skip button is unavailable.");

        ClickControl(button);
        var stable = await WaitUntilAsync(
            () => !ReferenceEquals(ActiveScreenContext.Instance.GetCurrentScreen(), currentScreen),
            TimeSpan.FromSeconds(10));

        return BuildResult(ActionIds.SkipRewardCards, stable);
    }

    private static async Task<BridgeActionResult> ExecuteSelectDeckCardAsync(BridgeActionRequest request)
    {
        EnsureActionAvailable(ActionIds.SelectDeckCard);

        var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        var selectionContext = GameUiAccess.ResolveSelectionContext(currentScreen);
        if (selectionContext == null)
        {
            throw InvalidAction(ActionIds.SelectDeckCard);
        }

        var options = GameUiAccess.GetDeckSelectionOptions(currentScreen);
        var optionIndex = RequireOptionIndex(ActionIds.SelectDeckCard, request.OptionIndex, options.Count);
        var selected = options[optionIndex];
        var initialProgress = CaptureSelectionProgress(selectionContext, optionIndex);

        if (selectionContext.IsCombatEmbedded)
        {
            var methodName = string.Equals(selectionContext.Mode, "UpgradeSelect", StringComparison.Ordinal)
                ? "SelectCardInUpgradeMode"
                : "SelectCardInSimpleMode";
            selectionContext.Root.Call(methodName, selected);
            selectionContext.Root.Call("CheckIfSelectionComplete");
        }
        else
        {
            if (!await TryApplyDeckSelectionChoiceAsync(selectionContext, selected, optionIndex, initialProgress))
            {
                Log.Warn($"[{Entry.ModId}] deck selection choice produced no observable progress: kind={selectionContext.Kind} option={optionIndex} selectedCount={initialProgress.SelectedCount} canConfirm={initialProgress.CanConfirm}");
            }
        }

        var stable = await ConfirmDeckSelectionAsync(selectionContext, initialProgress, optionIndex);

        return BuildResult(ActionIds.SelectDeckCard, stable);
    }

    private static async Task<BridgeActionResult> ExecuteConfirmSelectionAsync()
    {
        EnsureActionAvailable(ActionIds.ConfirmSelection);

        var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        var selectionContext = GameUiAccess.ResolveSelectionContext(currentScreen);
        if (selectionContext == null)
        {
            throw InvalidAction(ActionIds.ConfirmSelection);
        }

        if (!GameUiAccess.TryGetDeckSelectionConfirmButton(currentScreen, out var confirmButton) || confirmButton == null)
        {
            throw StateUnavailable(ActionIds.ConfirmSelection, "Selection confirm button is unavailable.");
        }

        ClickControl(confirmButton);
        var stable = await WaitForStableSnapshotAsync(
            snapshot =>
            {
                if (snapshot.Screen != ScreenIds.CardSelection || snapshot.Selection == null)
                {
                    return IsSnapshotActionableOrSettled(snapshot);
                }

                return !snapshot.AvailableActions.Contains(ActionIds.ConfirmSelection, StringComparer.Ordinal) &&
                    !snapshot.AvailableActions.Contains(ActionIds.SelectDeckCard, StringComparer.Ordinal);
            },
            TimeSpan.FromSeconds(10));

        return BuildResult(ActionIds.ConfirmSelection, stable);
    }

    private static async Task<BridgeActionResult> ExecuteProceedAsync()
    {
        EnsureActionAvailable(ActionIds.Proceed);

        var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        var proceedButton = GameUiAccess.GetProceedButton(currentScreen) ?? throw StateUnavailable(ActionIds.Proceed, "Proceed button is unavailable.");

        ClickControl(proceedButton);
        var stable = await WaitForStableSnapshotAsync(
            snapshot =>
                !ReferenceEquals(ActiveScreenContext.Instance.GetCurrentScreen(), currentScreen) &&
                IsSnapshotActionableOrSettled(snapshot),
            TimeSpan.FromSeconds(12));

        return BuildResult(ActionIds.Proceed, stable);
    }

    private static async Task<BridgeActionResult> ExecuteChooseEventOptionAsync(BridgeActionRequest request)
    {
        EnsureActionAvailable(ActionIds.ChooseEventOption);

        var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        var eventSynchronizer = ReflectionUtils.GetMemberValue(RunManager.Instance, "EventSynchronizer");
        var eventModel = ReflectionUtils.InvokeMethod(eventSynchronizer, "GetLocalEvent") ?? throw StateUnavailable(ActionIds.ChooseEventOption, "Event state is unavailable.");
        var isFinished = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(eventModel, "IsFinished")) == true;

        if (isFinished)
        {
            if (request.OptionIndex != null && request.OptionIndex != 0)
            {
                throw new BridgeApiException(409, "invalid_target", "Event is finished. Only option_index 0 is valid.", new
                {
                    action = ActionIds.ChooseEventOption,
                    option_index = request.OptionIndex,
                    is_finished = true
                });
            }

            await NEventRoom.Proceed();
            var stable = await WaitUntilAsync(
                () => ScreenClassifier.Classify(ActiveScreenContext.Instance.GetCurrentScreen()) != ScreenIds.Event,
                TimeSpan.FromSeconds(10));

            return BuildResult(ActionIds.ChooseEventOption, stable);
        }

        var options = ReflectionUtils.Enumerate(ReflectionUtils.GetMemberValue(eventModel, "CurrentOptions")).ToList();
        var optionIndex = RequireOptionIndex(ActionIds.ChooseEventOption, request.OptionIndex, options.Count);
        if (ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(options[optionIndex], "IsLocked")) == true)
        {
            throw new BridgeApiException(409, "invalid_target", "The selected event option is locked.", new
            {
                action = ActionIds.ChooseEventOption,
                option_index = optionIndex
            });
        }

        var previousSignature = BuildEventSignature(eventModel);
        ReflectionUtils.InvokeMethod(eventSynchronizer, "ChooseLocalOption", optionIndex);
        var stableOption = await WaitUntilAsync(
            () =>
            {
                var screen = ActiveScreenContext.Instance.GetCurrentScreen();
                if (screen is not NEventRoom)
                {
                    return true;
                }

                var currentEvent = ReflectionUtils.InvokeMethod(eventSynchronizer, "GetLocalEvent");
                return currentEvent != null && BuildEventSignature(currentEvent) != previousSignature;
            },
            TimeSpan.FromSeconds(10));

        return BuildResult(ActionIds.ChooseEventOption, stableOption);
    }

    private static async Task<BridgeActionResult> ExecuteOpenChestAsync()
    {
        EnsureActionAvailable(ActionIds.OpenChest);

        if (ActiveScreenContext.Instance.GetCurrentScreen() is not NTreasureRoom treasureRoom)
        {
            throw InvalidAction(ActionIds.OpenChest);
        }

        var chestButton = treasureRoom.GetNodeOrNull<Node>("%Chest") ?? throw StateUnavailable(ActionIds.OpenChest, "Chest button is unavailable.");
        ClickControl(chestButton);

        var stable = await WaitUntilAsync(
            () => GameUiAccess.GetTreasureRelicCollection(ActiveScreenContext.Instance.GetCurrentScreen()) != null,
            TimeSpan.FromSeconds(10));

        return BuildResult(ActionIds.OpenChest, stable);
    }

    private static async Task<BridgeActionResult> ExecuteChooseTreasureRelicAsync(BridgeActionRequest request)
    {
        EnsureActionAvailable(ActionIds.ChooseTreasureRelic);

        var synchronizer = ReflectionUtils.GetMemberValue(RunManager.Instance, "TreasureRoomRelicSynchronizer");
        var relics = ReflectionUtils.Enumerate(ReflectionUtils.GetMemberValue(synchronizer, "CurrentRelics")).ToList();
        var optionIndex = RequireOptionIndex(ActionIds.ChooseTreasureRelic, request.OptionIndex, relics.Count);

        ReflectionUtils.InvokeMethod(synchronizer, "PickRelicLocally", optionIndex);
        var stable = await WaitUntilAsync(
            () => GameUiAccess.GetProceedButton(ActiveScreenContext.Instance.GetCurrentScreen()) != null,
            TimeSpan.FromSeconds(10));

        return BuildResult(ActionIds.ChooseTreasureRelic, stable);
    }

    private static async Task<BridgeActionResult> ExecuteChooseRestOptionAsync(BridgeActionRequest request)
    {
        EnsureActionAvailable(ActionIds.ChooseRestOption);

        var synchronizer = ReflectionUtils.GetMemberValue(RunManager.Instance, "RestSiteSynchronizer");
        var options = ReflectionUtils.Enumerate(ReflectionUtils.InvokeMethod(synchronizer, "GetLocalOptions")).ToList();
        var optionIndex = RequireOptionIndex(ActionIds.ChooseRestOption, request.OptionIndex, options.Count);

        if (ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(options[optionIndex], "IsEnabled")) != true)
        {
            throw InvalidTarget(ActionIds.ChooseRestOption, optionIndex, options.Count, "The selected rest option is not enabled.");
        }

        var beforeSignature = BuildOptionSignature(options);
        ReflectionUtils.InvokeMethod(synchronizer, "ChooseLocalOption", optionIndex);

        var stable = await WaitUntilAsync(
            () =>
            {
                var currentOptions = ReflectionUtils.Enumerate(ReflectionUtils.InvokeMethod(synchronizer, "GetLocalOptions")).ToList();
                return BuildOptionSignature(currentOptions) != beforeSignature || ScreenClassifier.Classify(ActiveScreenContext.Instance.GetCurrentScreen()) != ScreenIds.Rest;
            },
            TimeSpan.FromSeconds(10));

        return BuildResult(ActionIds.ChooseRestOption, stable);
    }

    private static async Task<BridgeActionResult> ExecuteOpenShopInventoryAsync()
    {
        EnsureActionAvailable(ActionIds.OpenShopInventory);

        if (ActiveScreenContext.Instance.GetCurrentScreen() is not NMerchantRoom merchantRoom)
        {
            throw InvalidAction(ActionIds.OpenShopInventory);
        }

        merchantRoom.OpenInventory();
        var stable = await WaitUntilAsync(
            () => GameUiAccess.GetMerchantInventoryScreen(ActiveScreenContext.Instance.GetCurrentScreen())?.IsOpen == true,
            TimeSpan.FromSeconds(10));

        return BuildResult(ActionIds.OpenShopInventory, stable);
    }

    private static async Task<BridgeActionResult> ExecuteCloseShopInventoryAsync()
    {
        EnsureActionAvailable(ActionIds.CloseShopInventory);

        if (ActiveScreenContext.Instance.GetCurrentScreen() is not NMerchantInventory inventoryScreen)
        {
            throw InvalidAction(ActionIds.CloseShopInventory);
        }

        var backButton = inventoryScreen.GetNodeOrNull<Node>("%BackButton") ?? throw StateUnavailable(ActionIds.CloseShopInventory, "Shop back button is unavailable.");
        ClickControl(backButton);
        var stable = await WaitUntilAsync(
            () => ScreenClassifier.Classify(ActiveScreenContext.Instance.GetCurrentScreen()) == ScreenIds.Shop &&
                  GameUiAccess.GetMerchantInventoryScreen(ActiveScreenContext.Instance.GetCurrentScreen())?.IsOpen != true,
            TimeSpan.FromSeconds(10));

        return BuildResult(ActionIds.CloseShopInventory, stable);
    }

    private static async Task<BridgeActionResult> ExecuteBuyCardAsync(BridgeActionRequest request)
    {
        EnsureActionAvailable(ActionIds.BuyCard);

        var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        var inventory = GameUiAccess.GetMerchantInventory(currentScreen) ?? throw StateUnavailable(ActionIds.BuyCard, "Shop inventory is unavailable.");
        var cards = GameUiAccess.GetMerchantCardEntries(currentScreen);
        var optionIndex = RequireOptionIndex(ActionIds.BuyCard, request.OptionIndex, cards.Count);
        var entry = cards[optionIndex];

        if (!entry.IsStocked)
        {
            throw InvalidTarget(ActionIds.BuyCard, optionIndex, cards.Count, "The selected card is out of stock.");
        }

        var previousGold = inventory.Player.Gold;
        var previousCardId = entry.CreationResult?.Card.Id.Entry;
        var success = await entry.OnTryPurchaseWrapper(inventory);
        if (!success)
        {
            throw new BridgeApiException(409, "invalid_action", "Card purchase failed in the current state.", new
            {
                action = ActionIds.BuyCard,
                option_index = optionIndex,
                screen = ScreenClassifier.Classify(currentScreen),
                gold = inventory.Player.Gold,
                enough_gold = entry.EnoughGold,
                is_stocked = entry.IsStocked
            });
        }

        var stable = await WaitForMerchantCardPurchaseAsync(
            inventory.Player,
            entry,
            previousGold,
            previousCardId,
            TimeSpan.FromSeconds(10));

        return BuildResult(ActionIds.BuyCard, stable);
    }

    private static async Task<BridgeActionResult> ExecuteBuyRelicAsync(BridgeActionRequest request)
    {
        EnsureActionAvailable(ActionIds.BuyRelic);

        var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        var inventory = GameUiAccess.GetMerchantInventory(currentScreen) ?? throw StateUnavailable(ActionIds.BuyRelic, "Shop inventory is unavailable.");
        var relics = GameUiAccess.GetMerchantRelicEntries(currentScreen);
        var optionIndex = RequireOptionIndex(ActionIds.BuyRelic, request.OptionIndex, relics.Count);
        var entry = relics[optionIndex];

        if (!entry.IsStocked)
        {
            throw InvalidTarget(ActionIds.BuyRelic, optionIndex, relics.Count, "The selected relic is out of stock.");
        }

        var previousGold = inventory.Player.Gold;
        var previousRelicId = entry.Model?.Id.Entry;
        var success = await entry.OnTryPurchaseWrapper(inventory);
        if (!success)
        {
            throw new BridgeApiException(409, "invalid_action", "Relic purchase failed in the current state.", new
            {
                action = ActionIds.BuyRelic,
                option_index = optionIndex,
                screen = ScreenClassifier.Classify(currentScreen),
                gold = inventory.Player.Gold,
                enough_gold = entry.EnoughGold,
                is_stocked = entry.IsStocked
            });
        }

        var stable = await WaitForMerchantRelicPurchaseAsync(
            inventory.Player,
            entry,
            previousGold,
            previousRelicId,
            TimeSpan.FromSeconds(10));

        return BuildResult(ActionIds.BuyRelic, stable);
    }

    private static async Task<BridgeActionResult> ExecuteBuyPotionAsync(BridgeActionRequest request)
    {
        EnsureActionAvailable(ActionIds.BuyPotion);

        var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        var inventory = GameUiAccess.GetMerchantInventory(currentScreen) ?? throw StateUnavailable(ActionIds.BuyPotion, "Shop inventory is unavailable.");
        var potions = GameUiAccess.GetMerchantPotionEntries(currentScreen);
        var optionIndex = RequireOptionIndex(ActionIds.BuyPotion, request.OptionIndex, potions.Count);
        var entry = potions[optionIndex];

        if (!entry.IsStocked)
        {
            throw InvalidTarget(ActionIds.BuyPotion, optionIndex, potions.Count, "The selected potion is out of stock.");
        }

        var previousGold = inventory.Player.Gold;
        var previousPotionId = entry.Model?.Id.Entry;
        var success = await entry.OnTryPurchaseWrapper(inventory);
        if (!success)
        {
            throw new BridgeApiException(409, "invalid_action", "Potion purchase failed in the current state.", new
            {
                action = ActionIds.BuyPotion,
                option_index = optionIndex,
                screen = ScreenClassifier.Classify(currentScreen),
                gold = inventory.Player.Gold,
                enough_gold = entry.EnoughGold,
                is_stocked = entry.IsStocked
            });
        }

        var stable = await WaitForMerchantPotionPurchaseAsync(
            inventory.Player,
            entry,
            previousGold,
            previousPotionId,
            TimeSpan.FromSeconds(10));

        return BuildResult(ActionIds.BuyPotion, stable);
    }

    private static async Task<BridgeActionResult> ExecuteRemoveCardAtShopAsync()
    {
        EnsureActionAvailable(ActionIds.RemoveCardAtShop);

        var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        var inventory = GameUiAccess.GetMerchantInventory(currentScreen) ?? throw StateUnavailable(ActionIds.RemoveCardAtShop, "Shop inventory is unavailable.");
        var entry = GameUiAccess.GetMerchantCardRemovalEntry(currentScreen) ?? throw StateUnavailable(ActionIds.RemoveCardAtShop, "Card removal entry is unavailable.");
        _ = ObserveBackgroundResultAsync(entry.OnTryPurchaseWrapper(inventory), ActionIds.RemoveCardAtShop);
        var stable = await WaitForShopCardRemovalTransitionAsync(TimeSpan.FromSeconds(10));

        return BuildResult(ActionIds.RemoveCardAtShop, stable);
    }

    private static async Task<bool> WaitForMerchantCardPurchaseAsync(
        Player player,
        MerchantCardEntry entry,
        int previousGold,
        string? previousCardId,
        TimeSpan timeout)
    {
        var deadline = DateTime.UtcNow + timeout;
        while (DateTime.UtcNow < deadline)
        {
            await WaitForNextFrameAsync();

            var currentGold = player.Gold;
            var currentCardId = entry.CreationResult?.Card.Id.Entry;
            if (currentGold != previousGold || currentCardId != previousCardId || !entry.IsStocked)
            {
                return true;
            }
        }

        return player.Gold != previousGold || entry.CreationResult?.Card.Id.Entry != previousCardId || !entry.IsStocked;
    }

    private static async Task<bool> WaitForMerchantRelicPurchaseAsync(
        Player player,
        MerchantRelicEntry entry,
        int previousGold,
        string? previousRelicId,
        TimeSpan timeout)
    {
        var deadline = DateTime.UtcNow + timeout;
        while (DateTime.UtcNow < deadline)
        {
            await WaitForNextFrameAsync();

            var currentGold = player.Gold;
            var currentRelicId = entry.Model?.Id.Entry;
            if (currentGold != previousGold || currentRelicId != previousRelicId || !entry.IsStocked)
            {
                return true;
            }
        }

        return player.Gold != previousGold || entry.Model?.Id.Entry != previousRelicId || !entry.IsStocked;
    }

    private static async Task<bool> WaitForMerchantPotionPurchaseAsync(
        Player player,
        MerchantPotionEntry entry,
        int previousGold,
        string? previousPotionId,
        TimeSpan timeout)
    {
        var deadline = DateTime.UtcNow + timeout;
        while (DateTime.UtcNow < deadline)
        {
            await WaitForNextFrameAsync();

            var currentGold = player.Gold;
            var currentPotionId = entry.Model?.Id.Entry;
            if (currentGold != previousGold || currentPotionId != previousPotionId || !entry.IsStocked)
            {
                return true;
            }
        }

        return player.Gold != previousGold || entry.Model?.Id.Entry != previousPotionId || !entry.IsStocked;
    }

    private static async Task<bool> WaitForShopCardRemovalTransitionAsync(TimeSpan timeout)
    {
        return await WaitForStableSnapshotAsync(
            snapshot =>
                snapshot.Screen == ScreenIds.CardSelection &&
                snapshot.Selection != null &&
                snapshot.AvailableActions.Contains(ActionIds.SelectDeckCard, StringComparer.Ordinal),
            timeout);
    }

    private static async Task ObserveBackgroundResultAsync(Task<bool> task, string actionName)
    {
        try
        {
            await task.ConfigureAwait(true);
        }
        catch (Exception ex)
        {
            Log.Warn($"[{Entry.ModId}] Background action {actionName} failed: {ex}");
        }
    }

    private static async Task<bool> ConfirmDeckSelectionAsync(SelectionContextRef selectionContext, SelectionProgressSnapshot initialProgress, int selectedOptionIndex)
    {
        if (selectionContext.IsCombatEmbedded)
        {
            var initialSelectedCount = selectionContext.SelectedCount ?? 0;
            return await WaitForStableSnapshotAsync(
                snapshot =>
                {
                    if (snapshot.Screen != ScreenIds.CardSelection || snapshot.Selection == null)
                    {
                        return IsSnapshotActionableOrSettled(snapshot);
                    }

                    if (selectionContext.RequiresConfirmation == true)
                    {
                        var selectedCount = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(snapshot.Selection, "SelectedCount", "selectedCount"));
                        return selectedCount > initialSelectedCount;
                    }

                    return false;
                },
                TimeSpan.FromSeconds(10));
        }

        switch (selectionContext.ScreenContext)
        {
            case NChooseACardSelectionScreen:
                return await WaitUntilAsync(
                    () => ActiveScreenContext.Instance.GetCurrentScreen() is not NChooseACardSelectionScreen,
                    TimeSpan.FromSeconds(10));

            case NCardGridSelectionScreen gridSelectionScreen:
            {
                var deadline = DateTime.UtcNow + TimeSpan.FromSeconds(10);
                while (DateTime.UtcNow < deadline)
                {
                    await WaitForNextFrameAsync();

                    if (!GodotObject.IsInstanceValid(gridSelectionScreen) ||
                        ActiveScreenContext.Instance.GetCurrentScreen() is not NCardGridSelectionScreen activeGridSelection)
                    {
                        return true;
                    }

                    if (GameUiAccess.TryGetDeckSelectionConfirmButton(activeGridSelection, out var confirmButton))
                    {
                        ClickControl(confirmButton);
                    }
                }

                return ActiveScreenContext.Instance.GetCurrentScreen() is not NCardGridSelectionScreen;
            }

            default:
                return await WaitUntilAsync(
                    () =>
                    {
                        var screen = ActiveScreenContext.Instance.GetCurrentScreen();
                        var currentContext = GameUiAccess.ResolveSelectionContext(screen);
                        if (currentContext == null)
                        {
                            return true;
                        }

                        return HasSelectionProgressAdvanced(currentContext, selectedOptionIndex, initialProgress);
                    },
                    TimeSpan.FromSeconds(10));
        }
    }

    private static async Task<bool> TryApplyDeckSelectionChoiceAsync(SelectionContextRef selectionContext, NCardHolder selected, int optionIndex, SelectionProgressSnapshot initialProgress)
    {
        async Task<bool> TryAndObserveAsync(Action attempt)
        {
            attempt();
            if (TryInvokeSelectionCompletion(selectionContext))
            {
                // completion hook fired
            }

            return await WaitUntilAsync(
                () =>
                {
                    var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
                    var currentContext = GameUiAccess.ResolveSelectionContext(currentScreen);
                    if (currentContext == null)
                    {
                        return true;
                    }

                    return HasSelectionProgressAdvanced(currentContext, optionIndex, initialProgress);
                },
                TimeSpan.FromMilliseconds(350));
        }

        foreach (var candidate in new object?[] { selectionContext.Root, selectionContext.ScreenContext }.Where(candidate => candidate != null))
        {
            if (await TryAndObserveAsync(() => ReflectionUtils.TryInvokeMethod(candidate, "SelectCard", selected)))
            {
                return true;
            }
            if (await TryAndObserveAsync(() => ReflectionUtils.TryInvokeMethod(candidate, "SelectCard", optionIndex)))
            {
                return true;
            }
            if (await TryAndObserveAsync(() => ReflectionUtils.TryInvokeMethod(candidate, "OnCardPressed", selected)))
            {
                return true;
            }
            if (await TryAndObserveAsync(() => ReflectionUtils.TryInvokeMethod(candidate, "OnCardHolderPressed", selected)))
            {
                return true;
            }
            if (await TryAndObserveAsync(() => ReflectionUtils.TryInvokeMethod(candidate, "HandleCardSelected", selected)))
            {
                return true;
            }
            if (await TryAndObserveAsync(() => ReflectionUtils.TryInvokeMethod(candidate, "SelectCardInSimpleMode", selected)))
            {
                return true;
            }
        }

        if (await TryAndObserveAsync(() => ReflectionUtils.TryInvokeMethod(selected, "ForceClick")))
        {
            return true;
        }
        if (await TryAndObserveAsync(() => selected.EmitSignal(NCardHolder.SignalName.Pressed, selected)))
        {
            return true;
        }

        return false;
    }

    private static bool TryInvokeSelectionCompletion(SelectionContextRef selectionContext)
    {
        if (!ShouldInvokeSelectionCompletion(selectionContext))
        {
            return false;
        }

        return ReflectionUtils.TryInvokeMethod(selectionContext.Root, "CheckIfSelectionComplete") ||
            ReflectionUtils.TryInvokeMethod(selectionContext.ScreenContext, "CheckIfSelectionComplete");
    }

    private static bool ShouldInvokeSelectionCompletion(SelectionContextRef selectionContext)
    {
        if (selectionContext.ScreenContext is NSimpleCardSelectScreen)
        {
            return false;
        }

        if (selectionContext.IsCombatEmbedded)
        {
            return true;
        }

        if (selectionContext.RequiresConfirmation == true || selectionContext.CanConfirm == true)
        {
            return true;
        }

        return true;
    }

    private static SelectionProgressSnapshot CaptureSelectionProgress(SelectionContextRef selectionContext, int optionIndex)
    {
        return new SelectionProgressSnapshot
        {
            SelectedCount = selectionContext.SelectedCount ?? 0,
            CanConfirm = selectionContext.CanConfirm == true,
            OptionSelected = IsSelectionOptionSelected(selectionContext, optionIndex),
            AvailableActionsSignature = string.Join("|", StateSnapshotBuilder.Build().AvailableActions)
        };
    }

    private static bool HasSelectionProgressAdvanced(SelectionContextRef selectionContext, int optionIndex, SelectionProgressSnapshot initialProgress)
    {
        if ((selectionContext.SelectedCount ?? 0) > initialProgress.SelectedCount)
        {
            return true;
        }

        if (selectionContext.CanConfirm == true && !initialProgress.CanConfirm)
        {
            return true;
        }

        if (IsSelectionOptionSelected(selectionContext, optionIndex) && !initialProgress.OptionSelected)
        {
            return true;
        }

        var availableActionsSignature = string.Join("|", StateSnapshotBuilder.Build().AvailableActions);
        return !string.Equals(availableActionsSignature, initialProgress.AvailableActionsSignature, StringComparison.Ordinal);
    }

    private static bool IsSelectionOptionSelected(SelectionContextRef selectionContext, int optionIndex)
    {
        var options = GameUiAccess.GetDeckSelectionOptions(selectionContext.Root);
        if (optionIndex < 0 || optionIndex >= options.Count)
        {
            return false;
        }

        return ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(options[optionIndex], "IsSelected", "_isSelected", "Selected", "_selected")) == true;
    }

    private sealed class SelectionProgressSnapshot
    {
        public int SelectedCount { get; init; }
        public bool CanConfirm { get; init; }
        public bool OptionSelected { get; init; }
        public string AvailableActionsSignature { get; init; } = string.Empty;
    }
}
