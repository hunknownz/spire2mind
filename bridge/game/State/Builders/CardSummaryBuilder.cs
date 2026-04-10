using MegaCrit.Sts2.Core.Combat;
using MegaCrit.Sts2.Core.Context;
using MegaCrit.Sts2.Core.Entities.Cards;
using MegaCrit.Sts2.Core.Models;
using Spire2Mind.Bridge.Models;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.Ui;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.Util;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.Ui;
using Spire2Mind.Bridge.Game.Threading;

namespace Spire2Mind.Bridge.Game.State.Builders;

internal static class CardSummaryBuilder
{
    public static CardSummary Build(object? card, int index, CombatState? combatState = null, object? sourceNode = null)
    {
        if (card is CardModel cardModel)
        {
            return BuildFromCardModel(cardModel, index, combatState, sourceNode);
        }

        // Fallback for untyped objects (e.g., selection screen cards not yet typed)
        return BuildFromReflection(card, index, combatState, sourceNode);
    }

    private static CardSummary BuildFromCardModel(CardModel card, int index, CombatState? combatState, object? sourceNode)
    {
        var targetType = card.TargetType.ToString();
        var energyCost = card.EnergyCost;
        int? energyValue = null;
        try
        {
            energyValue = energyCost?.GetWithModifiers(CostModifiers.All);
        }
        catch
        {
            // EnergyCost may not be available in all contexts
        }

        return new CardSummary
        {
            Index = index,
            CardId = ReflectionUtils.ModelId(card),
            Name = ReflectionUtils.LocalizedText(card.TitleLocString),
            EnergyCost = energyValue,
            StarCost = ReflectionUtils.ToNullableInt(card.GetStarCostWithModifiers()),
            CostsX = energyCost?.CostsX,
            StarCostsX = card.HasStarCostX,
            TargetType = targetType,
            RequiresTarget = RequiresIndexedTarget(targetType),
            Playable = card.CanPlay(),
            IsSelected = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(sourceNode ?? (object)card, "IsSelected", "_isSelected", "Selected", "_selected")),
            ValidTargetIndices = ResolveValidTargetIndices(targetType, combatState)
        };
    }

    private static CardSummary BuildFromReflection(object? card, int index, CombatState? combatState, object? sourceNode)
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
            return combatState.HittableEnemies
                .Select((_, index) => index)
                .ToList();
        }

        if (string.Equals(targetType, "AnyAlly", StringComparison.Ordinal))
        {
            var localPlayer = LocalContext.GetMe(combatState);

            return combatState.Players
                .Where(player => player.Creature is { IsAlive: true } && player != localPlayer)
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
