using MegaCrit.Sts2.Core.Combat;
using MegaCrit.Sts2.Core.Context;
using MegaCrit.Sts2.Core.Entities.Players;
using MegaCrit.Sts2.Core.Models;
using MegaCrit.Sts2.Core.Nodes.Rooms;

namespace Spire2Mind.Bridge.Game.State.Builders;

internal static class CombatActionAvailability
{
    internal sealed class CombatAvailabilityDiagnostic
    {
        public string? ScreenType { get; init; }

        public string? RoomMode { get; init; }

        public bool IsCombatRoom { get; init; }

        public bool HasCombatState { get; init; }

        public bool IsInProgress { get; init; }

        public bool IsOverOrEnding { get; init; }

        public bool IsPlayPhase { get; init; }

        public bool PlayerActionsDisabled { get; init; }

        public bool HandVisible { get; init; }

        public bool InCardPlay { get; init; }

        public bool IsInCardSelection { get; init; }

        public bool LocalPlayerAlive { get; init; }

        public int HandCount { get; init; }

        public int PlayableCardCount { get; init; }
    }

    public static bool CanPlayAnyCard(object? currentScreen, CombatState? combatState)
    {
        if (!CanUseCombatActions(currentScreen, combatState, out var localPlayer, out _))
        {
            return false;
        }

        var player = localPlayer as Player;
        var hand = player?.PlayerCombatState?.Hand;
        if (hand == null)
        {
            return false;
        }

        return hand.Cards.Any(card => card.CanPlay());
    }

    public static bool CanEndTurn(object? currentScreen, CombatState? combatState)
    {
        return CanUseCombatActions(currentScreen, combatState, out _, out _);
    }

    public static CombatAvailabilityDiagnostic CaptureDiagnostic(object? currentScreen, CombatState? combatState)
    {
        var diagnostic = new CombatAvailabilityDiagnostic
        {
            ScreenType = currentScreen?.GetType().Name,
            IsCombatRoom = currentScreen is NCombatRoom,
            HasCombatState = combatState != null,
            IsInProgress = CombatManager.Instance.IsInProgress,
            IsOverOrEnding = CombatManager.Instance.IsOverOrEnding,
            IsPlayPhase = CombatManager.Instance.IsPlayPhase,
            PlayerActionsDisabled = CombatManager.Instance.PlayerActionsDisabled
        };

        if (currentScreen is not NCombatRoom room)
        {
            return diagnostic;
        }

        var hand = room.Ui?.Hand;
        var player = LocalContext.GetMe(combatState) as Player;
        var handCards = player?.PlayerCombatState?.Hand?.Cards;

        return new CombatAvailabilityDiagnostic
        {
            ScreenType = diagnostic.ScreenType,
            RoomMode = room.Mode.ToString(),
            IsCombatRoom = true,
            HasCombatState = diagnostic.HasCombatState,
            IsInProgress = diagnostic.IsInProgress,
            IsOverOrEnding = diagnostic.IsOverOrEnding,
            IsPlayPhase = diagnostic.IsPlayPhase,
            PlayerActionsDisabled = diagnostic.PlayerActionsDisabled,
            HandVisible = hand != null,
            InCardPlay = hand?.InCardPlay == true,
            IsInCardSelection = hand?.IsInCardSelection == true,
            LocalPlayerAlive = player?.Creature?.IsAlive == true,
            HandCount = handCards?.Count ?? 0,
            PlayableCardCount = handCards?.Count(card => card.CanPlay()) ?? 0
        };
    }

    private static bool CanUseCombatActions(object? currentScreen, CombatState? combatState, out object? localPlayer, out NCombatRoom? combatRoom)
    {
        localPlayer = null;
        combatRoom = null;

        if (combatState == null || currentScreen is not NCombatRoom room)
        {
            return false;
        }

        combatRoom = room;

        if (!CombatManager.Instance.IsInProgress ||
            CombatManager.Instance.IsOverOrEnding ||
            !CombatManager.Instance.IsPlayPhase ||
            CombatManager.Instance.PlayerActionsDisabled)
        {
            return false;
        }

        if (!string.Equals(room.Mode.ToString(), "ActiveCombat", StringComparison.Ordinal))
        {
            return false;
        }

        var hand = room.Ui?.Hand;
        if (hand == null || hand.InCardPlay || hand.IsInCardSelection)
        {
            return false;
        }

        var player = LocalContext.GetMe(combatState) as Player;
        localPlayer = player;
        return player?.Creature?.IsAlive == true;
    }
}
