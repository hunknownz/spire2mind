using MegaCrit.Sts2.Core.Combat;
using MegaCrit.Sts2.Core.Context;
using MegaCrit.Sts2.Core.Entities.Cards;
using MegaCrit.Sts2.Core.Entities.Creatures;
using MegaCrit.Sts2.Core.Entities.Players;
using MegaCrit.Sts2.Core.MonsterMoves.Intents;
using Spire2Mind.Bridge.Models;
using Spire2Mind.Bridge.Game.Ui;
using Spire2Mind.Bridge.Game.Threading;

namespace Spire2Mind.Bridge.Game.State.Builders;

internal static class CombatSectionBuilder
{
    public static CombatSummary? Build(object? currentScreen, CombatState? combatState)
    {
        if (combatState == null)
        {
            return null;
        }

        var localPlayer = LocalContext.GetMe(combatState);
        var playerCreature = localPlayer?.Creature;
        var playerCombatState = localPlayer?.PlayerCombatState;

        var enemies = combatState.HittableEnemies
            .Select((enemy, index) =>
            {
                var monster = enemy.Monster;
                return new EnemySummary
                {
                    Index = index,
                    EnemyId = ReflectionUtils.ModelId(monster),
                    Name = ReflectionUtils.LocalizedText(monster) ?? ReflectionUtils.ModelId(monster),
                    CurrentHp = (int?)enemy.CurrentHp,
                    MaxHp = (int?)enemy.MaxHp,
                    Block = (int?)enemy.Block,
                    IsAlive = enemy.IsAlive,
                    IsHittable = true,
                    Powers = BuildPowers(enemy),
                    MoveId = ReflectionUtils.ModelId(monster?.NextMove),
                    Intents = BuildEnemyIntents(enemy, combatState)
                };
            })
            .ToList();

        var hand = playerCombatState?.Hand?.Cards
            .Select((card, index) => CardSummaryBuilder.Build(card, index, combatState))
            .ToList() ?? new List<CardSummary>();

        // Pile contents
        var drawPile = BuildPileCards(localPlayer, PileType.Draw);
        var discardPile = BuildPileCards(localPlayer, PileType.Discard);
        var exhaustPile = BuildPileCards(localPlayer, PileType.Exhaust);

        var diagnostic = CombatActionAvailability.CaptureDiagnostic(currentScreen, combatState);

        return new CombatSummary
        {
            ActionWindowOpen = diagnostic.IsCombatRoom &&
                               diagnostic.HasCombatState &&
                               diagnostic.IsInProgress &&
                               !diagnostic.IsOverOrEnding &&
                               diagnostic.IsPlayPhase &&
                               !diagnostic.PlayerActionsDisabled &&
                               diagnostic.HandVisible &&
                               !diagnostic.InCardPlay &&
                               !diagnostic.IsInCardSelection &&
                               diagnostic.LocalPlayerAlive,
            IsInCardPlay = diagnostic.InCardPlay,
            IsInCardSelection = diagnostic.IsInCardSelection,
            PlayerActionsDisabled = diagnostic.PlayerActionsDisabled,
            IsOverOrEnding = diagnostic.IsOverOrEnding,
            Player = new PlayerSummary
            {
                CurrentHp = (int?)playerCreature?.CurrentHp,
                MaxHp = (int?)playerCreature?.MaxHp,
                Block = (int?)playerCreature?.Block,
                Energy = playerCombatState?.Energy,
                Stars = playerCombatState?.Stars,
                Powers = BuildPowers(playerCreature)
            },
            Enemies = enemies,
            Hand = hand,
            DrawPileCount = drawPile.Count,
            DiscardPileCount = discardPile.Count,
            ExhaustPileCount = exhaustPile.Count,
            DrawPile = drawPile,
            DiscardPile = discardPile,
            ExhaustPile = exhaustPile
        };
    }

    public static int? ResolveTurn(CombatState? combatState)
    {
        return combatState?.RoundNumber;
    }

    private static IReadOnlyList<EnemyIntentSummary> BuildEnemyIntents(Creature enemy, CombatState combatState)
    {
        var monster = enemy.Monster;
        var nextMove = monster?.NextMove;
        if (nextMove?.Intents == null)
        {
            return Array.Empty<EnemyIntentSummary>();
        }

        var targets = combatState.Players
            .Select(p => p.Creature)
            .Where(c => c != null)
            .ToArray()!;

        return ReflectionUtils.Enumerate(nextMove.Intents)
            .Select((intent, index) =>
            {
                var attackIntent = intent as AttackIntent;
                var multiAttack = intent as MultiAttackIntent;
                return new EnemyIntentSummary
                {
                    Index = index,
                    IntentType = ReflectionUtils.GetMemberValue(intent, "IntentType")?.ToString(),
                    Label = SafeLocalizedIntentLabel(intent, targets, enemy),
                    Damage = SafeNullableInt(() => attackIntent != null ? attackIntent.GetSingleDamage(targets, enemy) : null),
                    Hits = SafeNullableInt(() => multiAttack == null ? null : Math.Max(1, multiAttack.Repeats)),
                    TotalDamage = SafeNullableInt(() => attackIntent != null ? attackIntent.GetTotalDamage(targets, enemy) : null),
                    StatusCardCount = SafeNullableInt(() => ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(intent, "CardCount")))
                };
            })
            .ToList();
    }

    private static string? SafeLocalizedIntentLabel(object intent, Creature[] targets, Creature owner)
    {
        try
        {
            return ReflectionUtils.LocalizedText(ReflectionUtils.InvokeMethod(intent, "GetIntentLabel", targets, owner));
        }
        catch
        {
            return null;
        }
    }

    private static int? SafeNullableInt(Func<object?> valueFactory)
    {
        try
        {
            return ReflectionUtils.ToNullableInt(valueFactory());
        }
        catch
        {
            return null;
        }
    }

    private static int? SafeNullableInt(Func<int?> valueFactory)
    {
        try
        {
            return valueFactory();
        }
        catch
        {
            return null;
        }
    }

    private static IReadOnlyList<PowerSummary> BuildPowers(Creature? creature)
    {
        if (creature?.Powers == null)
        {
            return Array.Empty<PowerSummary>();
        }

        return creature.Powers
            .Select(power => new PowerSummary
            {
                PowerId = ReflectionUtils.ModelId(power),
                Name = ReflectionUtils.LocalizedText(power),
                Amount = (int?)power.Amount,
                PowerType = power.Type.ToString()
            })
            .ToList();
    }

    private static IReadOnlyList<CardSummary> BuildPileCards(Player? player, PileType pileType)
    {
        if (player == null)
        {
            return Array.Empty<CardSummary>();
        }

        var pile = pileType.GetPile(player);
        if (pile?.Cards == null)
        {
            return Array.Empty<CardSummary>();
        }

        return pile.Cards
            .Select((card, index) => CardSummaryBuilder.Build(card, index))
            .ToList();
    }
}
