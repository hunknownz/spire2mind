using MegaCrit.Sts2.Core.Combat;
using MegaCrit.Sts2.Core.Context;
using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Game;

internal static class CardSummaryBuilder
{
    public static CardSummary Build(object? card, int index, CombatState? combatState = null, object? sourceNode = null)
    {
        var targetType = ReflectionUtils.GetMemberValue(card, "TargetType")?.ToString();
        var energyCost = ReflectionUtils.GetMemberValue(card, "EnergyCost");
        var energyValue = ReflectionUtils.ToNullableInt(
            ReflectionUtils.InvokeMethod(
                energyCost,
                "GetWithModifiers",
                ResolveEnumValue("MegaCrit.Sts2.Core.Entities.Cards.CostModifiers", "All")));

        return new CardSummary
        {
            Index = index,
            CardId = ReflectionUtils.ModelId(card),
            Name = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(card, "TitleLocString", "Title")),
            EnergyCost = energyValue,
            StarCost = ReflectionUtils.ToNullableInt(ReflectionUtils.InvokeMethod(card, "GetStarCostWithModifiers")),
            CostsX = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(energyCost, "CostsX")),
            StarCostsX = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(card, "HasStarCostX")),
            TargetType = targetType,
            RequiresTarget = RequiresIndexedTarget(targetType),
            Playable = ReflectionUtils.ToNullableBool(ReflectionUtils.InvokeMethod(card, "CanPlay")),
            IsSelected = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(sourceNode ?? card, "IsSelected", "_isSelected", "Selected", "_selected")),
            ValidTargetIndices = ResolveValidTargetIndices(targetType, combatState)
        };
    }

    internal static bool? RequiresIndexedTarget(string? targetType)
    {
        if (string.IsNullOrWhiteSpace(targetType))
        {
            return null;
        }

        return targetType switch
        {
            "AnyEnemy" => true,
            "AnyAlly" => true,
            "None" => false,
            "Self" => false,
            "AllEnemies" => false,
            "RandomEnemy" => false,
            "AllAllies" => false,
            _ => false
        };
    }

    private static IReadOnlyList<int> ResolveValidTargetIndices(string? targetType, CombatState? combatState)
    {
        if (combatState == null || RequiresIndexedTarget(targetType) != true)
        {
            return Array.Empty<int>();
        }

        if (string.Equals(targetType, "AnyEnemy", StringComparison.Ordinal))
        {
            return ReflectionUtils.Enumerate(ReflectionUtils.GetMemberValue(combatState, "Enemies"))
                .OfType<object>()
                .Where(enemy =>
                    ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(enemy, "IsAlive")) == true &&
                    ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(enemy, "IsHittable")) == true)
                .Select((_, index) => index)
                .ToList();
        }

        if (string.Equals(targetType, "AnyAlly", StringComparison.Ordinal))
        {
            var localPlayer = LocalContext.GetMe(combatState);
            var localNetId = ReflectionUtils.GetMemberValue(localPlayer, "NetId");

            return ReflectionUtils.Enumerate(ReflectionUtils.GetMemberValue(combatState, "Players"))
                .OfType<object>()
                .Where(player =>
                {
                    var creature = ReflectionUtils.GetMemberValue(player, "Creature");
                    return creature != null &&
                           ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(creature, "IsAlive")) == true &&
                           !Equals(ReflectionUtils.GetMemberValue(player, "NetId"), localNetId);
                })
                .Select((_, index) => index)
                .ToList();
        }

        return Array.Empty<int>();
    }

    private static object? ResolveEnumValue(string fullName, string valueName)
    {
        var enumType = Type.GetType($"{fullName}, sts2");
        return enumType == null ? null : Enum.Parse(enumType, valueName);
    }
}
