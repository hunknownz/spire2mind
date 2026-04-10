using MegaCrit.Sts2.Core.Context;
using MegaCrit.Sts2.Core.Entities.Players;
using MegaCrit.Sts2.Core.Runs;
using Spire2Mind.Bridge.Models;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.Ui;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.Util;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.Ui;
using Spire2Mind.Bridge.Game.Threading;

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

        return new RunSummary
        {
            Character = ReflectionUtils.LocalizedText(localPlayer.Character),
            Floor = runState.TotalFloor,
            CurrentHp = (int?)creature?.CurrentHp,
            MaxHp = (int?)creature?.MaxHp,
            Gold = localPlayer.Gold,
            MaxEnergy = localPlayer.MaxEnergy,
            DeckCount = deck?.Cards?.Count ?? 0,
            RelicCount = localPlayer.Relics?.Count ?? 0
        };
    }
}
