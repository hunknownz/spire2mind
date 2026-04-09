namespace Spire2Mind.Bridge.Models;

internal sealed class ShopSummary
{
    public bool? IsOpen { get; init; }

    public bool? CanOpen { get; init; }

    public bool? CanClose { get; init; }

    public IReadOnlyList<ShopCardSummary> Cards { get; init; } = Array.Empty<ShopCardSummary>();

    public IReadOnlyList<ShopRelicSummary> Relics { get; init; } = Array.Empty<ShopRelicSummary>();

    public IReadOnlyList<ShopPotionSummary> Potions { get; init; } = Array.Empty<ShopPotionSummary>();

    public ShopCardRemovalSummary? CardRemoval { get; init; }
}

internal sealed class ShopCardSummary
{
    public int Index { get; init; }

    public string? Category { get; init; }

    public string? CardId { get; init; }

    public string? Name { get; init; }

    public int? Price { get; init; }

    public bool? OnSale { get; init; }

    public bool? IsStocked { get; init; }

    public bool? EnoughGold { get; init; }
}

internal sealed class ShopRelicSummary
{
    public int Index { get; init; }

    public string? RelicId { get; init; }

    public string? Name { get; init; }

    public string? Rarity { get; init; }

    public int? Price { get; init; }

    public bool? IsStocked { get; init; }

    public bool? EnoughGold { get; init; }
}

internal sealed class ShopPotionSummary
{
    public int Index { get; init; }

    public string? PotionId { get; init; }

    public string? Name { get; init; }

    public string? Rarity { get; init; }

    public int? Price { get; init; }

    public bool? IsStocked { get; init; }

    public bool? EnoughGold { get; init; }
}

internal sealed class ShopCardRemovalSummary
{
    public int? Price { get; init; }

    public bool? Available { get; init; }

    public bool? Used { get; init; }

    public bool? EnoughGold { get; init; }
}
