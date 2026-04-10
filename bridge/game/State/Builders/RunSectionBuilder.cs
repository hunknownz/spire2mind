using MegaCrit.Sts2.Core.Context;
using MegaCrit.Sts2.Core.Entities.Players;
using MegaCrit.Sts2.Core.Runs;
using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Game.State.Builders;

internal static class RunSectionBuilder
{
    public static RunSummary? Build(RunState? runState)
    {
        if (runState == null)
        {
            return null;
        }

        var localPlayer = LocalContext.GetMe(runState) as Player;
        if (localPlayer == null)
        {
            return null;
        }

        var creature = localPlayer.Creature;
        var deck = localPlayer.Deck;
        var relics = localPlayer.Relics;
        var potions = localPlayer.Potions;

        return new RunSummary
        {
            Character = ReflectionUtils.LocalizedText(localPlayer.Character),
            Floor = runState.TotalFloor,
            CurrentHp = (int?)creature?.CurrentHp,
            MaxHp = (int?)creature?.MaxHp,
            Gold = localPlayer.Gold,
            MaxEnergy = localPlayer.MaxEnergy,
            DeckCount = deck?.Cards?.Count ?? 0,
            RelicCount = relics?.Count ?? 0,
            PotionCount = potions?.Count(p => p != null) ?? 0,
            Deck = deck?.Cards?
                .Select(card => new RunCardSummary
                {
                    CardId = ReflectionUtils.ModelId(card),
                    Name = ReflectionUtils.LocalizedText(card)
                })
                .ToList() ?? new List<RunCardSummary>(),
            Relics = relics?
                .Select(relic => new RunRelicSummary
                {
                    RelicId = ReflectionUtils.ModelId(relic),
                    Name = ReflectionUtils.LocalizedText(relic)
                })
                .ToList() ?? new List<RunRelicSummary>(),
            Potions = potions?
                .Select(potion => new RunPotionSummary
                {
                    PotionId = potion != null ? ReflectionUtils.ModelId(potion) : null,
                    Name = potion != null ? ReflectionUtils.LocalizedText(potion) : null,
                    IsEmpty = potion == null
                })
                .ToList() ?? new List<RunPotionSummary>()
        };
    }
}
