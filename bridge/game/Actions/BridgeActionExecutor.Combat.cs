using MegaCrit.Sts2.Core.Combat;
using MegaCrit.Sts2.Core.Context;
using MegaCrit.Sts2.Core.Entities.Creatures;
using MegaCrit.Sts2.Core.GameActions;
using MegaCrit.Sts2.Core.Runs;
using MegaCrit.Sts2.Core.Nodes.Screens.ScreenContext;
using Spire2Mind.Bridge.Models;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.State;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Ui;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.State;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Util;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.State;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Ui;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.State;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.State.Builders;

namespace Spire2Mind.Bridge.Game.Actions;

internal static partial class BridgeActionExecutor
{
    private static async Task<BridgeActionResult> ExecutePlayCardAsync(BridgeActionRequest request)
    {
        EnsureActionAvailable(ActionIds.PlayCard);

        var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        var combatState = CombatManager.Instance.DebugOnlyGetState() ?? throw StateUnavailable(ActionIds.PlayCard, "Combat state is unavailable.");
        if (!CombatActionAvailability.CanPlayAnyCard(currentScreen, combatState))
        {
            throw InvalidAction(ActionIds.PlayCard, screen: ScreenClassifier.Classify(currentScreen));
        }

        var player = LocalContext.GetMe(combatState) ?? throw StateUnavailable(ActionIds.PlayCard, "Local player is unavailable.");
        var hand = player.PlayerCombatState?.Hand.Cards.ToList() ?? throw StateUnavailable(ActionIds.PlayCard, "Player hand is unavailable.");
        var cardIndex = RequireOptionIndex(ActionIds.PlayCard, request.CardIndex, hand.Count, "card_index");
        var card = hand[cardIndex];

        var target = ResolveCardTarget(request, combatState, card);
        var previousHandCount = hand.Count;
        var previousEnergy = player.PlayerCombatState?.Energy;

        if (ReflectionUtils.ToNullableBool(ReflectionUtils.InvokeMethod(card, "TryManualPlay", target)) != true)
        {
            throw InvalidAction(ActionIds.PlayCard);
        }

        var stable = await WaitUntilAsync(
            () =>
            {
                var currentCombat = CombatManager.Instance.DebugOnlyGetState();
                if (currentCombat == null)
                {
                    return true;
                }

                var currentPlayer = LocalContext.GetMe(currentCombat);
                var currentHand = currentPlayer?.PlayerCombatState?.Hand.Cards;
                return currentHand == null ||
                       currentHand.Count != previousHandCount ||
                       currentPlayer?.PlayerCombatState?.Energy != previousEnergy;
            },
            TimeSpan.FromSeconds(10));

        return BuildResult(ActionIds.PlayCard, stable);
    }

    private static async Task<BridgeActionResult> ExecuteEndTurnAsync()
    {
        EnsureActionAvailable(ActionIds.EndTurn);

        var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        var combatState = CombatManager.Instance.DebugOnlyGetState() ?? throw StateUnavailable(ActionIds.EndTurn, "Combat state is unavailable.");
        if (!CombatActionAvailability.CanEndTurn(currentScreen, combatState))
        {
            throw InvalidAction(ActionIds.EndTurn, screen: ScreenClassifier.Classify(currentScreen));
        }

        var player = LocalContext.GetMe(combatState) ?? throw StateUnavailable(ActionIds.EndTurn, "Local player is unavailable.");
        var roundNumber = combatState.RoundNumber;

        RunManager.Instance.ActionQueueSynchronizer.RequestEnqueue(new EndPlayerTurnAction(player, roundNumber));
        var stable = await WaitUntilAsync(
            () =>
            {
                var currentCombat = CombatManager.Instance.DebugOnlyGetState();
                return currentCombat == null ||
                       currentCombat.RoundNumber != roundNumber ||
                       currentCombat.CurrentSide != CombatSide.Player ||
                       !CombatManager.Instance.IsPlayPhase;
            },
            TimeSpan.FromSeconds(10));

        return BuildResult(ActionIds.EndTurn, stable);
    }

    private static Creature? ResolveCardTarget(BridgeActionRequest request, CombatState combatState, object card)
    {
        var targetType = ReflectionUtils.GetMemberValue(card, "TargetType")?.ToString();
        var requiresTarget = CardSummaryBuilder.RequiresIndexedTarget(targetType) == true;
        if (!requiresTarget)
        {
            return null;
        }

        if (request.TargetIndex == null)
        {
            throw new BridgeApiException(400, "invalid_request", "play_card requires target_index for targeted cards.", new
            {
                action = ActionIds.PlayCard,
                card_id = ReflectionUtils.ModelId(card)
            });
        }

        var targets = ResolveIndexedTargets(combatState, targetType);

        if (request.TargetIndex < 0 || request.TargetIndex >= targets.Count)
        {
            throw new BridgeApiException(409, "invalid_target", "target_index is out of range.", new
            {
                action = ActionIds.PlayCard,
                target_type = targetType,
                target_index = request.TargetIndex,
                target_count = targets.Count
            });
        }

        return targets[request.TargetIndex.Value];
    }

    private static IReadOnlyList<Creature> ResolveIndexedTargets(CombatState combatState, string? targetType)
    {
        if (string.Equals(targetType, "AnyEnemy", StringComparison.Ordinal))
        {
            return combatState.Enemies
                .Where(enemy => enemy.IsAlive && enemy.IsHittable)
                .Cast<Creature>()
                .ToList();
        }

        if (string.Equals(targetType, "AnyAlly", StringComparison.Ordinal))
        {
            var localPlayer = LocalContext.GetMe(combatState);
            var localNetId = ReflectionUtils.GetMemberValue(localPlayer, "NetId");
            return combatState.Players
                .Where(player => player.Creature != null && player.Creature.IsAlive && !Equals(player.NetId, localNetId))
                .Select(player => player.Creature)
                .Where(creature => creature != null)
                .Cast<Creature>()
                .ToList();
        }

        return Array.Empty<Creature>();
    }
}
