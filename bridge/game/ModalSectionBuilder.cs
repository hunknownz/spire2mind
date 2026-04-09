using Godot;
using System.Reflection;
using MegaCrit.Sts2.Core.Nodes;
using MegaCrit.Sts2.Core.Nodes.CommonUi;
using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Game;

internal static class ModalSectionBuilder
{
    public static ModalSummary? Build()
    {
        var modal = NModalContainer.Instance?.OpenModal;
        if (modal == null)
        {
            return null;
        }

        var confirmControl = GameUiAccess.GetModalConfirmButton() ?? ResolveConfirmControl(modal);
        var dismissControl = GameUiAccess.GetModalCancelButton() ?? ResolveDismissControl(modal);
        var canConfirm = confirmControl != null && ReflectionUtils.IsAvailable(confirmControl);
        var canDismiss = dismissControl != null && ReflectionUtils.IsAvailable(dismissControl);

        if (modal.GetType().Name.EndsWith("Ftue", StringComparison.Ordinal))
        {
            canConfirm |= HasMethod(modal, "ToggleRight", 1) || HasMethod(modal, "CloseFtue", 0) || HasMethod(modal, "CloseFtue", 1);

            var currentPage = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(modal, "_currentPage", "CurrentPage"));
            canDismiss |= currentPage > 1 && HasMethod(modal, "ToggleLeft", 1);
        }

        return new ModalSummary
        {
            ModalType = modal.GetType().Name,
            UnderlyingScreen = ResolveUnderlyingScreen(modal),
            Title = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(modal, "Title", "Header", "Name")),
            Description = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(modal, "Description", "Message", "Text")),
            CanConfirm = canConfirm,
            CanDismiss = canDismiss,
            ConfirmLabel = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(confirmControl, "Text", "Label", "Title")),
            DismissLabel = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(dismissControl, "Text", "Label", "Title"))
        };
    }

    private static object? ResolveConfirmControl(object modal)
    {
        var directControl = ReflectionUtils.GetMemberValue(
            modal,
            "_confirmButton",
            "ConfirmButton",
            "_nextButton",
            "NextButton",
            "_yesButton",
            "YesButton");

        if (directControl != null)
        {
            return directControl;
        }

        var verticalPopup = ReflectionUtils.GetMemberValue(modal, "_verticalPopup", "VerticalPopup");
        return ReflectionUtils.GetMemberValue(verticalPopup, "YesButton");
    }

    private static object? ResolveDismissControl(object modal)
    {
        var directControl = ReflectionUtils.GetMemberValue(
            modal,
            "_dismissButton",
            "DismissButton",
            "_cancelButton",
            "CancelButton",
            "_prevButton",
            "PrevButton",
            "_backButton",
            "BackButton",
            "_noButton",
            "NoButton");

        if (directControl != null)
        {
            return directControl;
        }

        var verticalPopup = ReflectionUtils.GetMemberValue(modal, "_verticalPopup", "VerticalPopup");
        return ReflectionUtils.GetMemberValue(verticalPopup, "NoButton");
    }

    private static string? ResolveUnderlyingScreen(object modal)
    {
        if (modal is not Node modalNode)
        {
            return null;
        }

        var parent = modalNode.GetParent();
        while (parent != null)
        {
            if (parent == NGame.Instance?.MainMenu)
            {
                return ScreenIds.MainMenu;
            }

            parent = parent.GetParent();
        }

        return null;
    }

    private static bool HasMethod(object instance, string methodName, int parameterCount)
    {
        return instance.GetType().GetMethods(BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic)
            .Any(method => method.Name == methodName && method.GetParameters().Length == parameterCount);
    }
}
