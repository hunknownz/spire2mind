using MegaCrit.Sts2.Core.Runs;
using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Game;

internal static class RunSectionBuilder
{
    public static RunSummary? Build(RunState? runState)
    {
        if (runState == null)
        {
            return null;
        }

        var localPlayer = GameUiAccess.GetLocalPlayer(runState);
        var creature = ReflectionUtils.GetMemberValue(localPlayer, "Creature");
        var deck = ReflectionUtils.GetMemberValue(localPlayer, "Deck");
        var cards = ReflectionUtils.Enumerate(ReflectionUtils.GetMemberValue(deck, "Cards")).ToList();

        return new RunSummary
        {
            Character = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(localPlayer, "Character")),
            Floor = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(runState, "TotalFloor", "ActFloor")),
            CurrentHp = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(creature, "CurrentHp")),
            MaxHp = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(creature, "MaxHp")),
            Gold = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(localPlayer, "Gold")),
            MaxEnergy = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(localPlayer, "MaxEnergy")),
            DeckCount = cards.Count,
            RelicCount = ReflectionUtils.Enumerate(ReflectionUtils.GetMemberValue(localPlayer, "Relics")).Count()
        };
    }
}
