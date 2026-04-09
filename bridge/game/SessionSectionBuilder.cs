using MegaCrit.Sts2.Core.Runs;
using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Game;

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
