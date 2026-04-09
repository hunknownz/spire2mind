using Godot;
using MegaCrit.Sts2.Core.Nodes;
using MegaCrit.Sts2.Core.Combat;
using MegaCrit.Sts2.Core.Nodes.Screens.ScreenContext;
using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Game;

internal static partial class BridgeActionExecutor
{
    public static Task<BridgeActionResult> ExecuteAsync(BridgeActionRequest request)
    {
        var actionName = request.Action?.Trim().ToLowerInvariant();
        if (string.IsNullOrWhiteSpace(actionName))
        {
            throw new BridgeApiException(400, "invalid_request", "Request body must include a non-empty action.");
        }

        return actionName switch
        {
            ActionIds.ConfirmModal => ExecuteModalButtonAsync(actionName, GameUiAccess.GetModalConfirmButton),
            ActionIds.DismissModal => ExecuteModalButtonAsync(actionName, GameUiAccess.GetModalCancelButton),
            ActionIds.ContinueRun => ExecuteContinueRunAsync(),
            ActionIds.AbandonRun => ExecuteAbandonRunAsync(),
            ActionIds.OpenCharacterSelect => ExecuteOpenCharacterSelectAsync(),
            ActionIds.ContinueAfterGameOver => ExecuteContinueAfterGameOverAsync(),
            ActionIds.ReturnToMainMenu => ExecuteReturnToMainMenuAsync(),
            ActionIds.SelectCharacter => ExecuteSelectCharacterAsync(request),
            ActionIds.Embark => ExecuteEmbarkAsync(),
            ActionIds.ChooseMapNode => ExecuteChooseMapNodeAsync(request),
            ActionIds.ClaimReward => ExecuteClaimRewardAsync(request),
            ActionIds.ChooseRewardCard => ExecuteChooseRewardCardAsync(request),
            ActionIds.SkipRewardCards => ExecuteSkipRewardCardsAsync(),
            ActionIds.SelectDeckCard => ExecuteSelectDeckCardAsync(request),
            ActionIds.ConfirmSelection => ExecuteConfirmSelectionAsync(),
            ActionIds.Proceed => ExecuteProceedAsync(),
            ActionIds.ChooseEventOption => ExecuteChooseEventOptionAsync(request),
            ActionIds.OpenChest => ExecuteOpenChestAsync(),
            ActionIds.ChooseTreasureRelic => ExecuteChooseTreasureRelicAsync(request),
            ActionIds.ChooseRestOption => ExecuteChooseRestOptionAsync(request),
            ActionIds.OpenShopInventory => ExecuteOpenShopInventoryAsync(),
            ActionIds.CloseShopInventory => ExecuteCloseShopInventoryAsync(),
            ActionIds.BuyCard => ExecuteBuyCardAsync(request),
            ActionIds.BuyRelic => ExecuteBuyRelicAsync(request),
            ActionIds.BuyPotion => ExecuteBuyPotionAsync(request),
            ActionIds.RemoveCardAtShop => ExecuteRemoveCardAtShopAsync(),
            ActionIds.PlayCard => ExecutePlayCardAsync(request),
            ActionIds.EndTurn => ExecuteEndTurnAsync(),
            _ => throw new BridgeApiException(
                409,
                "invalid_action",
                "Action is not supported yet.",
                new { action = request.Action })
        };
    }

    private static void EnsureActionAvailable(string actionName)
    {
        var snapshot = StateSnapshotBuilder.Build();
        if (!snapshot.AvailableActions.Contains(actionName, StringComparer.Ordinal))
        {
            throw InvalidAction(actionName, snapshot);
        }
    }

    private static int RequireOptionIndex(string actionName, int? optionIndex, int count, string parameterName = "option_index")
    {
        if (optionIndex == null)
        {
            throw new BridgeApiException(400, "invalid_request", $"{actionName} requires {parameterName}.", new
            {
                action = actionName
            });
        }

        if (optionIndex < 0 || optionIndex >= count)
        {
            throw InvalidTarget(actionName, optionIndex.Value, count);
        }

        return optionIndex.Value;
    }

    private static BridgeApiException InvalidAction(string actionName, BridgeStateSnapshot? snapshot = null, string? screen = null)
    {
        snapshot ??= StateSnapshotBuilder.Build();
        var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        var combatDiagnostic = string.Equals(snapshot.Screen, ScreenIds.Combat, StringComparison.Ordinal)
            ? CombatActionAvailability.CaptureDiagnostic(currentScreen, CombatManager.Instance.DebugOnlyGetState())
            : null;

        return new BridgeApiException(409, "invalid_action", "Action is not available in the current state.", new
        {
            action = actionName,
            screen = screen ?? snapshot.Screen ?? ScreenClassifier.Classify(currentScreen),
            run_id = snapshot.RunId,
            turn = snapshot.Turn,
            available_actions = snapshot.AvailableActions,
            headline = snapshot.AgentView?.Headline,
            combat_diagnostic = combatDiagnostic
        });
    }

    private static BridgeApiException InvalidTarget(string actionName, int index, int count, string? message = null)
    {
        return new BridgeApiException(409, "invalid_target", message ?? "option_index is out of range.", new
        {
            action = actionName,
            option_index = index,
            option_count = count
        });
    }

    private static BridgeApiException StateUnavailable(string actionName, string message)
    {
        return new BridgeApiException(503, "state_unavailable", message, new
        {
            action = actionName,
            screen = ScreenClassifier.Classify(ActiveScreenContext.Instance.GetCurrentScreen())
        }, retryable: true);
    }

    private static BridgeActionResult BuildResult(string actionName, bool stable)
    {
        return new BridgeActionResult
        {
            Action = actionName,
            Status = stable ? "completed" : "pending",
            Stable = stable,
            Message = stable ? "Action completed." : "Action queued but state is still transitioning.",
            State = StateSnapshotBuilder.Build()
        };
    }

    private static async Task<bool> WaitForStableSnapshotAsync(Func<BridgeStateSnapshot, bool> predicate, TimeSpan timeout)
    {
        var deadline = DateTime.UtcNow + timeout;
        while (DateTime.UtcNow < deadline)
        {
            var snapshot = StateSnapshotBuilder.Build();
            if (predicate(snapshot))
            {
                return true;
            }

            await WaitForNextFrameAsync();
        }

        return predicate(StateSnapshotBuilder.Build());
    }

    private static bool IsSnapshotActionableOrSettled(BridgeStateSnapshot snapshot)
    {
        if (snapshot.Screen == ScreenIds.Unknown)
        {
            return false;
        }

        if (snapshot.Modal != null || snapshot.Screen == ScreenIds.Modal)
        {
            return true;
        }

        if (snapshot.Map?.IsTraveling == true)
        {
            return false;
        }

        return snapshot.Screen switch
        {
            ScreenIds.Map => snapshot.Map?.IsTraveling != true && snapshot.AvailableActions.Count > 0,
            ScreenIds.Combat => snapshot.AvailableActions.Count > 0,
            ScreenIds.MainMenu => snapshot.AvailableActions.Count > 0,
            ScreenIds.CharacterSelect => snapshot.CharacterSelect != null && snapshot.AvailableActions.Count > 0,
            ScreenIds.Reward => snapshot.Reward != null && snapshot.AvailableActions.Count > 0,
            ScreenIds.Event => snapshot.Event != null && snapshot.AvailableActions.Count > 0,
            ScreenIds.CardSelection => snapshot.Selection != null && snapshot.AvailableActions.Count > 0,
            ScreenIds.Shop => snapshot.Shop != null && snapshot.AvailableActions.Count > 0,
            ScreenIds.Rest => snapshot.Rest != null && snapshot.AvailableActions.Count > 0,
            ScreenIds.Chest => snapshot.Chest != null && snapshot.AvailableActions.Count > 0,
            ScreenIds.GameOver => snapshot.AvailableActions.Count > 0,
            _ => snapshot.AvailableActions.Count > 0
        };
    }

    private static async Task<bool> WaitUntilAsync(Func<bool> predicate, TimeSpan timeout)
    {
        var deadline = DateTime.UtcNow + timeout;
        while (DateTime.UtcNow < deadline)
        {
            if (predicate())
            {
                return true;
            }

            await WaitForNextFrameAsync();
        }

        return predicate();
    }

    private static async Task WaitForNextFrameAsync()
    {
        var game = NGame.Instance;
        if (game == null || !GodotObject.IsInstanceValid(game))
        {
            await Task.Delay(16).ConfigureAwait(true);
            return;
        }

        var tree = game.GetTree();
        if (tree == null || !GodotObject.IsInstanceValid(tree))
        {
            await Task.Delay(16).ConfigureAwait(true);
            return;
        }

        await game.ToSignal(tree, SceneTree.SignalName.ProcessFrame);
    }

    private static async Task<bool> AwaitBoolAsync(object? value)
    {
        if (value is not Task task)
        {
            return false;
        }

        await task.ConfigureAwait(true);
        var resultProperty = task.GetType().GetProperty("Result");
        return ReflectionUtils.ToNullableBool(resultProperty?.GetValue(task)) == true;
    }

    private static void ClickControl(object? control)
    {
        if (control == null)
        {
            return;
        }

        if (ReflectionUtils.TryInvokeMethod(control, "ForceClick"))
        {
            return;
        }

        if (control is BaseButton baseButton)
        {
            baseButton.EmitSignal(BaseButton.SignalName.Pressed);
        }
    }

    private static string BuildOptionSignature(IEnumerable<object> options)
    {
        return string.Join(
            "|",
            options.Select(option =>
                $"{ReflectionUtils.GetMemberValue<string>(option, "OptionId")}:{ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(option, "IsEnabled"))}:{ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(option, "Title"))}"));
    }

    private static string BuildEventSignature(object eventModel)
    {
        var options = ReflectionUtils.Enumerate(ReflectionUtils.GetMemberValue(eventModel, "CurrentOptions"));
        return string.Join(
            "|",
            options.Select(option =>
                $"{ReflectionUtils.GetMemberValue<string>(option, "TextKey", "OptionId")}:{ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(option, "IsLocked"))}:{ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(option, "IsProceed"))}:{ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(option, "Title"))}"));
    }
}
