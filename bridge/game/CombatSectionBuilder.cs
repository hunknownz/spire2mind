using MegaCrit.Sts2.Core.Combat;
using MegaCrit.Sts2.Core.Context;
using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Game;

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
        var playerCombatState = ReflectionUtils.GetMemberValue(localPlayer, "PlayerCombatState");
        var intentTargets = BuildIntentTargets(
            ReflectionUtils.Enumerate(ReflectionUtils.GetMemberValue(combatState, "Players"))
                .Select(player => ReflectionUtils.GetMemberValue(player, "Creature"))
                .OfType<object>());

        var enemies = ReflectionUtils.Enumerate(ReflectionUtils.GetMemberValue(combatState, "Enemies"))
            .OfType<object>()
            .Where(enemy =>
            {
                var isAlive = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(enemy, "IsAlive"));
                var isHittable = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(enemy, "IsHittable"));
                return isAlive == true && isHittable == true;
            })
            .Select((enemy, index) => new EnemySummary
            {
                Index = index,
                EnemyId = ReflectionUtils.ModelId(ReflectionUtils.GetMemberValue(enemy, "Monster")),
                Name = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(enemy, "Monster", "Name", "Title"))
                    ?? ReflectionUtils.ModelId(ReflectionUtils.GetMemberValue(enemy, "Monster")),
                CurrentHp = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(enemy, "CurrentHp")),
                MaxHp = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(enemy, "MaxHp")),
                Block = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(enemy, "Block")),
                IsAlive = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(enemy, "IsAlive")),
                IsHittable = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(enemy, "IsHittable")),
                MoveId = ReflectionUtils.ModelId(ReflectionUtils.GetMemberValue(ReflectionUtils.GetMemberValue(enemy, "Monster"), "NextMove")),
                Intents = BuildEnemyIntents(enemy, intentTargets)
            })
            .ToList();

        var hand = ReflectionUtils.Enumerate(ReflectionUtils.GetMemberValue(ReflectionUtils.GetMemberValue(playerCombatState, "Hand"), "Cards"))
            .Select((card, index) => CardSummaryBuilder.Build(card, index, combatState))
            .ToList();
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
                CurrentHp = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(playerCreature, "CurrentHp")),
                MaxHp = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(playerCreature, "MaxHp")),
                Block = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(playerCreature, "Block")),
                Energy = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(playerCombatState, "Energy")),
                Stars = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(playerCombatState, "Stars"))
            },
            Enemies = enemies,
            Hand = hand
        };
    }

    public static int? ResolveTurn(CombatState? combatState)
    {
        return ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(combatState, "RoundNumber"));
    }

    private static IReadOnlyList<EnemyIntentSummary> BuildEnemyIntents(object enemy, Array targets)
    {
        var nextMove = ReflectionUtils.GetMemberValue(ReflectionUtils.GetMemberValue(enemy, "Monster"), "NextMove");
        var intents = ReflectionUtils.Enumerate(ReflectionUtils.GetMemberValue(nextMove, "Intents"));

        return intents
            .Select((intent, index) => new EnemyIntentSummary
            {
                Index = index,
                IntentType = ReflectionUtils.GetMemberValue(intent, "IntentType")?.ToString(),
                Label = SafeLocalizedIntentLabel(intent, targets, enemy),
                Damage = SafeNullableInt(() => ReflectionUtils.InvokeMethod(intent, "GetSingleDamage", targets, enemy)),
                Hits = SafeNullableInt(() =>
                {
                    var repeats = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(intent, "Repeats"));
                    return repeats == null ? null : Math.Max(1, repeats.Value);
                }),
                TotalDamage = SafeNullableInt(() => ReflectionUtils.InvokeMethod(intent, "GetTotalDamage", targets, enemy)),
                StatusCardCount = SafeNullableInt(() => ReflectionUtils.GetMemberValue(intent, "CardCount"))
            })
            .ToList();
    }

    private static string? SafeLocalizedIntentLabel(object intent, Array targets, object enemy)
    {
        try
        {
            return ReflectionUtils.LocalizedText(ReflectionUtils.InvokeMethod(intent, "GetIntentLabel", targets, enemy));
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

    private static Array BuildIntentTargets(IEnumerable<object> targets)
    {
        var targetList = targets.Where(target => target != null).ToList();
        if (targetList.Count == 0)
        {
            return Array.Empty<object>();
        }

        var elementType = targetList[0].GetType();
        var array = Array.CreateInstance(elementType, targetList.Count);
        for (var index = 0; index < targetList.Count; index++)
        {
            array.SetValue(targetList[index], index);
        }

        return array;
    }
}
