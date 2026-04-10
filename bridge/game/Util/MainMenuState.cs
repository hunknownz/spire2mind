namespace Spire2Mind.Bridge.Game.Util;

internal sealed class MainMenuState
{
    public bool CanContinueRun { get; init; }

    public bool CanAbandonRun { get; init; }

    public bool CanOpenCharacterSelect { get; init; }

    public bool CanOpenMultiplayer { get; init; }

    public bool CanOpenCompendium { get; init; }

    public bool CanOpenTimeline { get; init; }

    public bool CanOpenSettings { get; init; }

    public bool CanOpenProfile { get; init; }

    public bool CanViewPatchNotes { get; init; }

    public bool CanQuitGame { get; init; }
}
