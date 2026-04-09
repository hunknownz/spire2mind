using Godot;
using MegaCrit.Sts2.Core.Nodes.Rooms;
using MegaCrit.Sts2.Core.Nodes.Screens.ScreenContext;
using MegaCrit.Sts2.Core.Runs;
using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Game;

internal static class ChestSectionBuilder
{
    public static ChestSummary? Build(object? currentScreen)
    {
        var screenContext = currentScreen as IScreenContext;
        if (GameUiAccess.GetTreasureRelicCollection(screenContext) != null)
        {
            var relics = ReflectionUtils.Enumerate(
                    ReflectionUtils.GetMemberValue(
                        ReflectionUtils.GetMemberValue(RunManager.Instance, "TreasureRoomRelicSynchronizer"),
                        "CurrentRelics"))
                .ToList();

            return new ChestSummary
            {
                IsOpened = true,
                HasRelicBeenClaimed = GameUiAccess.GetProceedButton(screenContext) != null,
                RelicOptions = relics.Select((relic, index) => new ChestRelicSummary
                {
                    Index = index,
                    RelicId = ReflectionUtils.ModelId(relic),
                    Name = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(relic, "Name", "Title")),
                    Rarity = ReflectionUtils.GetMemberValue(relic, "Rarity")?.ToString()
                }).ToList()
            };
        }

        if (currentScreen is not NTreasureRoom treasureRoom)
        {
            return null;
        }

        var chestButton = treasureRoom.GetNodeOrNull<Node>("%Chest");
        return new ChestSummary
        {
            IsOpened = chestButton == null || !GodotObject.IsInstanceValid(chestButton) || ReflectionUtils.IsEnabled(chestButton) == false,
            HasRelicBeenClaimed = GameUiAccess.GetProceedButton(screenContext) != null,
            RelicOptions = Array.Empty<ChestRelicSummary>()
        };
    }
}
