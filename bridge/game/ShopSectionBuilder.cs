using MegaCrit.Sts2.Core.Entities.Merchant;
using MegaCrit.Sts2.Core.Entities.Players;
using MegaCrit.Sts2.Core.Nodes.Screens.ScreenContext;
using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Game;

internal static class ShopSectionBuilder
{
    public static ShopSummary? Build(IScreenContext? currentScreen)
    {
        var merchantRoom = GameUiAccess.GetMerchantRoom(currentScreen);
        var inventoryScreen = GameUiAccess.GetMerchantInventoryScreen(currentScreen);
        var inventory = GameUiAccess.GetMerchantInventory(currentScreen);

        if (merchantRoom == null && inventoryScreen == null)
        {
            return null;
        }

        if (inventory == null)
        {
            return new ShopSummary
            {
                IsOpen = inventoryScreen?.IsOpen,
                CanOpen = merchantRoom?.Inventory != null && inventoryScreen?.IsOpen != true,
                CanClose = inventoryScreen?.IsOpen == true
            };
        }

        var characterCards = inventory.CharacterCardEntries
            .Select((entry, index) => BuildCard(entry, index, "character"));
        var colorlessCards = inventory.ColorlessCardEntries
            .Select((entry, index) => BuildCard(entry, inventory.CharacterCardEntries.Count + index, "colorless"));

        return new ShopSummary
        {
            IsOpen = inventoryScreen?.IsOpen,
            CanOpen = merchantRoom?.Inventory != null && inventoryScreen?.IsOpen != true,
            CanClose = inventoryScreen?.IsOpen == true,
            Cards = characterCards.Concat(colorlessCards).ToList(),
            Relics = inventory.RelicEntries.Select((entry, index) => new ShopRelicSummary
            {
                Index = index,
                RelicId = ReflectionUtils.ModelId(ReflectionUtils.GetMemberValue(entry, "Model")),
                Name = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(ReflectionUtils.GetMemberValue(entry, "Model"), "Name", "Title")),
                Rarity = ReflectionUtils.GetMemberValue(ReflectionUtils.GetMemberValue(entry, "Model"), "Rarity")?.ToString(),
                Price = entry.IsStocked ? entry.Cost : 0,
                IsStocked = entry.IsStocked,
                EnoughGold = entry.IsStocked && entry.EnoughGold
            }).ToList(),
            Potions = inventory.PotionEntries.Select((entry, index) => new ShopPotionSummary
            {
                Index = index,
                PotionId = ReflectionUtils.ModelId(ReflectionUtils.GetMemberValue(entry, "Model")),
                Name = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(ReflectionUtils.GetMemberValue(entry, "Model"), "Name", "Title")),
                Rarity = ReflectionUtils.GetMemberValue(ReflectionUtils.GetMemberValue(entry, "Model"), "Rarity")?.ToString(),
                Price = entry.IsStocked ? entry.Cost : 0,
                IsStocked = entry.IsStocked,
                EnoughGold = CanPurchasePotion(inventory.Player, entry)
            }).ToList(),
            CardRemoval = BuildCardRemoval(GameUiAccess.GetMerchantCardRemovalEntry(currentScreen))
        };
    }

    private static ShopCardSummary BuildCard(MerchantCardEntry entry, int index, string category)
    {
        var card = entry.CreationResult?.Card;
        return new ShopCardSummary
        {
            Index = index,
            Category = category,
            CardId = ReflectionUtils.ModelId(card),
            Name = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(card, "TitleLocString", "Title")),
            Price = entry.IsStocked ? entry.Cost : 0,
            OnSale = entry.IsOnSale,
            IsStocked = entry.IsStocked,
            EnoughGold = entry.IsStocked && entry.EnoughGold
        };
    }

    private static bool CanPurchasePotion(Player? player, MerchantPotionEntry entry)
    {
        return entry.IsStocked &&
            entry.EnoughGold &&
            player?.PotionSlots.Any(slot => slot == null) == true;
    }

    private static ShopCardRemovalSummary? BuildCardRemoval(object? entry)
    {
        if (entry == null)
        {
            return null;
        }

        return new ShopCardRemovalSummary
        {
            Price = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(entry, "Cost")),
            Available = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(entry, "IsStocked", "Available")),
            Used = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(entry, "Used")),
            EnoughGold = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(entry, "IsStocked")) == true &&
                ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(entry, "EnoughGold")) == true
        };
    }
}
