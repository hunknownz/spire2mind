using MegaCrit.Sts2.Core.Models;

namespace Spire2Mind.Bridge.Game.State;

/// <summary>
/// Provides read-only access to the game's model database (cards, relics, powers, potions).
/// Data is queried from ModelDb at runtime and includes both base game and mod content.
/// </summary>
internal static class ModelDbAccess
{
    public static object GetCards()
    {
        return ModelDb.AllCards
            .Select(card => new
            {
                id = ReflectionUtils.ModelId(card),
                name = ReflectionUtils.LocalizedText(card),
                type = card.Type.ToString(),
                rarity = card.Rarity.ToString(),
                cost = ResolveCost(card.EnergyCost),
                targetType = card.TargetType.ToString()
            })
            .ToList();
    }

    public static object GetRelics()
    {
        return ModelDb.AllRelics
            .Select(relic => new
            {
                id = ReflectionUtils.ModelId(relic),
                name = ReflectionUtils.LocalizedText(relic),
                rarity = relic.Rarity.ToString()
            })
            .ToList();
    }

    public static object GetPowers()
    {
        return ModelDb.AllPowers
            .Select(power => new
            {
                id = ReflectionUtils.ModelId(power),
                name = ReflectionUtils.LocalizedText(power),
                type = power.Type.ToString()
            })
            .ToList();
    }

    private static int? ResolveCost(object? energyCost)
    {
        if (energyCost == null) return null;
        // Try known property/field names across game versions
        return DynamicAccessor.ToNullableInt(
            DynamicAccessor.GetMemberValue(energyCost, "BaseValue", "BaseCost", "Value", "_baseValue", "_value")
            ?? DynamicAccessor.InvokeMethod(energyCost, "GetBaseValue")
            ?? DynamicAccessor.InvokeMethod(energyCost, "GetWithModifiers",
                   ResolveEnumValue("MegaCrit.Sts2.Core.Entities.Cards.CostModifiers", "None")));
    }

    private static object? ResolveEnumValue(string fullName, string valueName)
    {
        var enumType = Type.GetType($"{fullName}, sts2");
        return enumType == null ? null : Enum.Parse(enumType, valueName);
    }

    public static object GetPotions()
    {
        return ModelDb.AllPotions
            .Select(potion => new
            {
                id = ReflectionUtils.ModelId(potion),
                name = ReflectionUtils.LocalizedText(potion),
                rarity = potion.Rarity.ToString()
            })
            .ToList();
    }
}
