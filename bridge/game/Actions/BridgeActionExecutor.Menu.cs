using Godot;
using System.Reflection;
using MegaCrit.Sts2.Core.Nodes.Screens.CharacterSelect;
using MegaCrit.Sts2.Core.Nodes.Screens.MainMenu;
using MegaCrit.Sts2.Core.Nodes.Screens.ScreenContext;
using Spire2Mind.Bridge.Models;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.State;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Ui;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.State;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Util;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.State;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Ui;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.State;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.State.Builders;

namespace Spire2Mind.Bridge.Game.Actions;

internal static partial class BridgeActionExecutor
{
    private static async Task<BridgeActionResult> ExecuteModalButtonAsync(string actionName, Func<Node?> resolver)
    {
        var previousModal = GameUiAccess.GetOpenModal();
        var button = resolver();
        if (previousModal == null || !CanPerformModalAction(previousModal, actionName, button))
        {
            throw InvalidAction(actionName);
        }

        var previousSignature = BuildModalSignature(previousModal);
        if (!TryInvokeModalAction(previousModal, actionName, button))
        {
            throw InvalidAction(actionName);
        }

        var stable = await WaitUntilAsync(
            () =>
            {
                var currentModal = GameUiAccess.GetOpenModal();
                if (currentModal == null || !ReferenceEquals(currentModal, previousModal))
                {
                    return true;
                }

                return !string.Equals(BuildModalSignature(currentModal), previousSignature, StringComparison.Ordinal);
            },
            TimeSpan.FromSeconds(10));

        return BuildResult(actionName, stable);
    }

    private static async Task<BridgeActionResult> ExecuteContinueRunAsync()
    {
        EnsureActionAvailable(ActionIds.ContinueRun);

        var mainMenu = RequireMainMenu(ActionIds.ContinueRun);
        var continueButton = GameUiAccess.GetMainMenuContinueButton(mainMenu) ?? throw StateUnavailable(ActionIds.ContinueRun, "Continue button is unavailable.");
        continueButton.ForceClick();

        var stable = await WaitForStableSnapshotAsync(
            snapshot => snapshot.Screen != ScreenIds.MainMenu && IsSnapshotActionableOrSettled(snapshot),
            TimeSpan.FromSeconds(15));

        return BuildResult(ActionIds.ContinueRun, stable);
    }

    private static async Task<BridgeActionResult> ExecuteAbandonRunAsync()
    {
        EnsureActionAvailable(ActionIds.AbandonRun);

        var mainMenu = RequireMainMenu(ActionIds.AbandonRun);
        var button = GameUiAccess.GetMainMenuAbandonRunButton(mainMenu) ?? throw StateUnavailable(ActionIds.AbandonRun, "Abandon button is unavailable.");
        button.ForceClick();

        var stable = await WaitUntilAsync(
            () => GameUiAccess.GetOpenModal() != null || ScreenClassifier.Classify(ActiveScreenContext.Instance.GetCurrentScreen()) != ScreenIds.MainMenu,
            TimeSpan.FromSeconds(10));

        return BuildResult(ActionIds.AbandonRun, stable);
    }

    private static async Task<BridgeActionResult> ExecuteOpenCharacterSelectAsync()
    {
        EnsureActionAvailable(ActionIds.OpenCharacterSelect);

        var mainMenu = RequireMainMenu(ActionIds.OpenCharacterSelect);
        var button = GameUiAccess.GetMainMenuSingleplayerButton(mainMenu);
        if (UiControlHelper.IsAvailable(button))
        {
            button!.ForceClick();
        }
        else
        {
            var characterSelectScreen = mainMenu.SubmenuStack?.GetSubmenuType<NCharacterSelectScreen>()
                ?? throw StateUnavailable(ActionIds.OpenCharacterSelect, "Character select submenu is unavailable.");

            characterSelectScreen.InitializeSingleplayer();
            mainMenu.SubmenuStack!.Push(characterSelectScreen);
        }

        var stable = await WaitUntilAsync(
            () => ScreenClassifier.Classify(ActiveScreenContext.Instance.GetCurrentScreen()) == ScreenIds.CharacterSelect,
            TimeSpan.FromSeconds(10));

        return BuildResult(ActionIds.OpenCharacterSelect, stable);
    }

    private static async Task<BridgeActionResult> ExecuteSelectCharacterAsync(BridgeActionRequest request)
    {
        EnsureActionAvailable(ActionIds.SelectCharacter);

        var buttons = GameUiAccess.GetCharacterSelectButtons();
        var optionIndex = RequireOptionIndex(ActionIds.SelectCharacter, request.OptionIndex, buttons.Count);
        var button = buttons[optionIndex];

        if (button.IsLocked)
        {
            throw new BridgeApiException(409, "invalid_target", "The selected character is locked.", new
            {
                action = ActionIds.SelectCharacter,
                option_index = optionIndex,
                character_id = button.Character?.Id?.Entry
            });
        }

        var targetCharacterId = button.Character?.Id?.Entry;
        button.Select();
        var stable = await WaitUntilAsync(
            () => string.Equals(GetSelectedCharacterId(), targetCharacterId, StringComparison.Ordinal),
            TimeSpan.FromSeconds(5));

        return BuildResult(ActionIds.SelectCharacter, stable);
    }

    private static async Task<BridgeActionResult> ExecuteEmbarkAsync()
    {
        EnsureActionAvailable(ActionIds.Embark);

        var embarkButton = GameUiAccess.GetCharacterEmbarkButton() ?? throw StateUnavailable(ActionIds.Embark, "Embark button is unavailable.");
        embarkButton.ForceClick();

        var stable = await WaitForStableSnapshotAsync(
            snapshot => snapshot.Screen != ScreenIds.CharacterSelect && IsSnapshotActionableOrSettled(snapshot),
            TimeSpan.FromSeconds(15));

        return BuildResult(ActionIds.Embark, stable);
    }

    private static NMainMenu RequireMainMenu(string actionName)
    {
        var mainMenu = GameUiAccess.GetMainMenu();
        if (mainMenu == null)
        {
            throw StateUnavailable(actionName, "Main menu is unavailable.");
        }

        return mainMenu;
    }

    private static string? GetSelectedCharacterId()
    {
        var screen = GameUiAccess.GetCharacterSelectScreen();
        return ReflectionUtils.ModelId(
            ReflectionUtils.GetMemberValue(
                ReflectionUtils.GetMemberValue(
                    ReflectionUtils.GetMemberValue(screen, "Lobby"),
                "LocalPlayer"),
                "character",
                "Character"));
    }

    private static bool TryInvokeModalAction(object modal, string actionName, Node? button)
    {
        if (modal.GetType().Name.EndsWith("Ftue", StringComparison.Ordinal))
        {
            if (actionName == ActionIds.ConfirmModal)
            {
                if (TryInvokeMethod(modal, "ToggleRight", new object?[] { null }))
                {
                    return true;
                }

                if (TryInvokeMethod(modal, "CloseFtue", new object?[] { null }) ||
                    TryInvokeMethod(modal, "CloseFtue"))
                {
                    return true;
                }
            }
            else if (actionName == ActionIds.DismissModal &&
                     TryInvokeMethod(modal, "ToggleLeft", new object?[] { null }))
            {
                return true;
            }
        }

        if (button == null)
        {
            return false;
        }

        ClickControl(button);
        return true;
    }

    private static bool CanPerformModalAction(object modal, string actionName, Node? button)
    {
        if (button != null && ReflectionUtils.IsAvailable(button))
        {
            return true;
        }

        if (!modal.GetType().Name.EndsWith("Ftue", StringComparison.Ordinal))
        {
            return false;
        }

        if (actionName == ActionIds.ConfirmModal)
        {
            return HasMethod(modal, "ToggleRight", 1) || HasMethod(modal, "CloseFtue", 0) || HasMethod(modal, "CloseFtue", 1);
        }

        var currentPage = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(modal, "_currentPage", "CurrentPage"));
        return actionName == ActionIds.DismissModal && currentPage > 1 && HasMethod(modal, "ToggleLeft", 1);
    }

    private static bool HasMethod(object instance, string methodName, int parameterCount)
    {
        const BindingFlags flags = BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic;
        return instance.GetType()
            .GetMethods(flags)
            .Any(candidate => candidate.Name == methodName && candidate.GetParameters().Length == parameterCount);
    }

    private static bool TryInvokeMethod(object instance, string methodName, params object?[] args)
    {
        args ??= Array.Empty<object?>();

        const BindingFlags flags = BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic;
        var method = instance.GetType()
            .GetMethods(flags)
            .FirstOrDefault(candidate => candidate.Name == methodName && candidate.GetParameters().Length == args.Length);
        if (method == null)
        {
            return false;
        }

        method.Invoke(instance, args);
        return true;
    }

    private static string BuildModalSignature(object modal)
    {
        return string.Join(
            "|",
            modal.GetType().Name,
            ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(modal, "_currentPage", "CurrentPage"))?.ToString() ?? "-",
            ReflectionUtils.IsAvailable(GameUiAccess.GetModalConfirmButton()).ToString(),
            ReflectionUtils.IsAvailable(GameUiAccess.GetModalCancelButton()).ToString(),
            ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(modal, "Title", "Header", "Name")) ?? "-");
    }
}
