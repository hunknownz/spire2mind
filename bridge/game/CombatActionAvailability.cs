using MegaCrit.Sts2.Core.Combat;
using MegaCrit.Sts2.Core.Context;
using MegaCrit.Sts2.Core.Nodes.Rooms;

namespace Spire2Mind.Bridge.Game;

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

        var handCards = ReflectionUtils.Enumerate(
                ReflectionUtils.GetMemberValue(
                    ReflectionUtils.GetMemberValue(localPlayer, "PlayerCombatState"),
                    "Hand") is { } hand
                    ? ReflectionUtils.GetMemberValue(hand, "Cards")
                    : null)
            .ToList();

        return handCards.Any(card => ReflectionUtils.ToNullableBool(ReflectionUtils.InvokeMethod(card, "CanPlay")) == true);
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

        var hand = ReflectionUtils.GetMemberValue(ReflectionUtils.GetMemberValue(room, "Ui"), "Hand");
        var handCards = ReflectionUtils.Enumerate(
                ReflectionUtils.GetMemberValue(
                    ReflectionUtils.GetMemberValue(LocalContext.GetMe(combatState), "PlayerCombatState"),
                    "Hand") is { } combatHand
                    ? ReflectionUtils.GetMemberValue(combatHand, "Cards")
                    : null)
            .ToList();

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
            InCardPlay = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(hand, "InCardPlay")) == true,
            IsInCardSelection = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(hand, "IsInCardSelection")) == true,
            LocalPlayerAlive = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(ReflectionUtils.GetMemberValue(LocalContext.GetMe(combatState), "Creature"), "IsAlive")) == true,
            HandCount = handCards.Count,
            PlayableCardCount = handCards.Count(card => ReflectionUtils.ToNullableBool(ReflectionUtils.InvokeMethod(card, "CanPlay")) == true)
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

        var hand = ReflectionUtils.GetMemberValue(ReflectionUtils.GetMemberValue(room, "Ui"), "Hand");
        if (hand == null ||
            ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(hand, "InCardPlay")) == true ||
            ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(hand, "IsInCardSelection")) == true)
        {
            return false;
        }

        localPlayer = LocalContext.GetMe(combatState);
        var creature = ReflectionUtils.GetMemberValue(localPlayer, "Creature");
        return localPlayer != null && ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(creature, "IsAlive")) == true;
    }
}
