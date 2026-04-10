using BaseLib.Abstracts;
using MegaCrit.Sts2.Core.Combat;
using MegaCrit.Sts2.Core.Context;
using MegaCrit.Sts2.Core.Entities.Creatures;
using MegaCrit.Sts2.Core.Entities.Players;
using MegaCrit.Sts2.Core.Entities.Cards;
using MegaCrit.Sts2.Core.GameActions.Multiplayer;
using MegaCrit.Sts2.Core.Models;
using MegaCrit.Sts2.Core.Rooms;
using MegaCrit.Sts2.Core.ValueProps;

namespace Spire2Mind.Bridge.Game.Hooks;

/// <summary>
/// Passive hook receiver that listens to combat and run events via BaseLib's
/// CustomSingletonModel. Each hook immediately notifies the EventService,
/// replacing the 120ms polling for combat events with zero-latency push.
/// </summary>
internal sealed class BridgeHooker : CustomSingletonModel
{
    /// <summary>
    /// Fired when any hook triggers. Listeners should rebuild state or publish events.
    /// </summary>
    internal static event Action<string, object?>? OnHookFired;

    private static int _subscribed;

    public BridgeHooker() : base(
        receiveCombatHooks: Volatile.Read(ref _subscribed) == 0,
        receiveRunHooks: Volatile.Read(ref _subscribed) == 0)
    {
        Interlocked.Exchange(ref _subscribed, 1);
    }

    public override bool ShouldReceiveCombatHooks => true;

    // ── Combat lifecycle ──────────────────────────────────────────

    public override Task AfterSideTurnStart(CombatSide side, CombatState combatState)
    {
        if (side == CombatSide.Player)
        {
            Fire("player_turn_started", new { round = combatState.RoundNumber });
        }

        return Task.CompletedTask;
    }

    public override Task AfterPlayerTurnStart(PlayerChoiceContext choiceContext, Player player)
    {
        Fire("player_action_window_opened", null);
        return Task.CompletedTask;
    }

    public override Task AfterTurnEnd(PlayerChoiceContext choiceContext, CombatSide side)
    {
        if (side == CombatSide.Player)
        {
            Fire("player_turn_ended", null);
        }

        return Task.CompletedTask;
    }

    public override Task AfterCombatEnd(CombatRoom room)
    {
        Fire("combat_ended", null);
        return Task.CompletedTask;
    }

    // ── Card events ───────────────────────────────────────────────

    public override Task AfterCardPlayed(PlayerChoiceContext context, CardPlay cardPlay)
    {
        Fire("card_played", new
        {
            cardId = ReflectionUtils.ModelId(cardPlay.Card),
            cardName = ReflectionUtils.LocalizedText(cardPlay.Card?.TitleLocString)
        });
        return Task.CompletedTask;
    }

    public override Task AfterCardDrawn(PlayerChoiceContext choiceContext, CardModel card, bool fromHandDraw)
    {
        Fire("card_drawn", new
        {
            cardId = ReflectionUtils.ModelId(card),
            fromHandDraw
        });
        return Task.CompletedTask;
    }

    // ── Resource events ───────────────────────────────────────────

    public override Task AfterEnergySpent(CardModel card, int amount)
    {
        Fire("energy_spent", new { amount });
        return Task.CompletedTask;
    }

    public override Task AfterCurrentHpChanged(Creature creature, decimal delta)
    {
        Fire("hp_changed", new
        {
            creatureName = creature?.ToString(),
            delta,
            isPlayer = creature != null && LocalContext.IsMe(creature)
        });
        return Task.CompletedTask;
    }

    public override Task AfterBlockGained(Creature creature, decimal amount, ValueProp props, CardModel? cardSource)
    {
        Fire("block_gained", new
        {
            amount,
            isPlayer = creature != null && LocalContext.IsMe(creature)
        });
        return Task.CompletedTask;
    }

    public override Task AfterStarsSpent(int amount, Player spender)
    {
        Fire("stars_spent", new { amount });
        return Task.CompletedTask;
    }

    // ── Internal ──────────────────────────────────────────────────

    private static void Fire(string hookName, object? data)
    {
        try
        {
            OnHookFired?.Invoke(hookName, data);
        }
        catch
        {
            // Never let a subscriber exception break the game's hook chain.
        }
    }
}
