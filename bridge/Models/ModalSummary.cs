namespace Spire2Mind.Bridge.Models;

internal sealed class ModalSummary
{
    public string? ModalType { get; init; }

    public string? UnderlyingScreen { get; init; }

    public string? Title { get; init; }

    public string? Description { get; init; }

    public bool CanConfirm { get; init; } = true;

    public bool CanDismiss { get; init; } = true;

    public string? ConfirmLabel { get; init; }

    public string? DismissLabel { get; init; }
}
