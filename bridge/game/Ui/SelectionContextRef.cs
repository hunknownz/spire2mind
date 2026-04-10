using Godot;
using MegaCrit.Sts2.Core.Nodes.Screens.ScreenContext;

namespace Spire2Mind.Bridge.Game.Ui;

internal sealed class SelectionContextRef
{
    public string Kind { get; init; } = string.Empty;

    public Node Root { get; init; } = null!;

    public IScreenContext? ScreenContext { get; init; }

    public bool IsCombatEmbedded { get; init; }

    public string? Mode { get; init; }

    public string? Prompt { get; init; }

    public int? MinSelect { get; init; }

    public int? MaxSelect { get; init; }

    public int? SelectedCount { get; init; }

    public bool? RequiresConfirmation { get; init; }

    public bool? CanConfirm { get; init; }
}
