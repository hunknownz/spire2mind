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

internal static class SessionSectionBuilder
{
    public static SessionSummary Build(string screen, RunState? runState)
    {
        if (screen == ScreenIds.CharacterSelect)
        {
            return new SessionSummary
            {
                Mode = "singleplayer",
                Phase = "character_select",
                ControlScope = "local_player"
            };
        }

        if (runState != null)
        {
            return new SessionSummary
            {
                Mode = "singleplayer",
                Phase = "run",
                ControlScope = "local_player"
            };
        }

        return new SessionSummary
        {
            Mode = "singleplayer",
            Phase = "menu",
            ControlScope = "local_player"
        };
    }
}
