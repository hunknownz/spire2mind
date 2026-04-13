using Godot;
using System.Reflection;
using MegaCrit.Sts2.Core.Nodes;
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
            BridgeDefaults.CombatActionTimeout);

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
            BridgeDefaults.TransitionTimeout);

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
            BridgeDefaults.CombatActionTimeout);

        return BuildResult(ActionIds.AbandonRun, stable);
    }

    private static async Task<BridgeActionResult> ExecuteOpenCharacterSelectAsync()
    {
        EnsureActionAvailable(ActionIds.OpenCharacterSelect);

        var mainMenu = RequireMainMenu(ActionIds.OpenCharacterSelect);

        // Follow STS2-Agent's approach: directly push CharacterSelectScreen via SubmenuStack
        // This bypasses the need to click the SingleplayerButton and handles mode selection automatically
        var characterSelectScreen = mainMenu.SubmenuStack?.GetSubmenuType<NCharacterSelectScreen>();
        if (characterSelectScreen == null)
        {
            throw StateUnavailable(ActionIds.OpenCharacterSelect, "CharacterSelectScreen not found in SubmenuStack.");
        }

        characterSelectScreen.InitializeSingleplayer();
        mainMenu.SubmenuStack!.Push(characterSelectScreen);

        // Wait for CharacterSelect screen to become active
        var deadline = DateTime.UtcNow + BridgeDefaults.TransitionTimeout;
        while (DateTime.UtcNow < deadline)
        {
            await WaitForNextFrameAsync();

            var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
            if (currentScreen is NCharacterSelectScreen)
            {
                return BuildResult(ActionIds.OpenCharacterSelect, true);
            }

            // Also check via classifier for the CHARACTER_SELECT screen ID
            var screenId = ScreenClassifier.Classify(currentScreen);
            if (screenId == ScreenIds.CharacterSelect)
            {
                return BuildResult(ActionIds.OpenCharacterSelect, true);
            }

            // If we see UNKNOWN or any other screen, try to proceed anyway
            // (the screen might be transitioning or in a mode-selection state)
        }

        // Final check
        var finalScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        if (finalScreen is NCharacterSelectScreen || ScreenClassifier.Classify(finalScreen) == ScreenIds.CharacterSelect)
        {
            return BuildResult(ActionIds.OpenCharacterSelect, true);
        }

        throw StateUnavailable(ActionIds.OpenCharacterSelect, $"CharacterSelect screen did not appear. Current screen: {ScreenClassifier.Classify(ActiveScreenContext.Instance.GetCurrentScreen())}");
    }

    /// <summary>
    /// Find a button by its displayed text (case-insensitive partial match).
    /// </summary>
    private static Node? FindButtonByText(Node parent, params string[] keywords)
    {
        var candidates = ReflectionUtils.Descendants(parent)
            .Where(node =>
                GodotObject.IsInstanceValid(node) &&
                IsButtonLike(node))
            .ToList();

        foreach (var candidate in candidates)
        {
            var text = ReflectionUtils.LocalizedText(candidate) ?? "";
            var name = candidate.Name.ToString();
            var combined = $"{text} {name}".ToLowerInvariant();
            if (keywords.Any(k => combined.Contains(k.ToLowerInvariant())))
            {
                return candidate;
            }
        }

        return null;
    }

    /// <summary>
    /// Finds and clicks a button on the mode selection screen (Standard / Daily / Custom).
    /// Returns true if a button was clicked, false otherwise.
    /// Skips main menu navigation buttons to avoid re-clicking SingleplayerButton.
    /// </summary>
    private static bool TryClickModeSelectionButton(object? currentScreen, NMainMenu? mainMenu)
    {
        if (currentScreen == null)
        {
            return false;
        }

        if (currentScreen is not Node root)
        {
            return false;
        }

        // Get the main menu to identify its navigation buttons
        var singleplayerButton = mainMenu != null ? GameUiAccess.GetMainMenuSingleplayerButton(mainMenu) : null;
        var continueButton = mainMenu != null ? GameUiAccess.GetMainMenuContinueButton(mainMenu) : null;
        var multiplayerButton = mainMenu != null ? GameUiAccess.GetMainMenuMultiplayerButton(mainMenu) : null;

        var candidates = ReflectionUtils.Descendants(root)
            .Where(candidate =>
                GodotObject.IsInstanceValid(candidate) &&
                ReflectionUtils.IsAvailable(candidate) &&
                IsButtonLike(candidate) &&
                // Skip main menu navigation buttons
                candidate != singleplayerButton &&
                candidate != continueButton &&
                candidate != multiplayerButton)
            .OrderBy(candidate => candidate.GetPath().ToString().Length)
            .ToList();

        if (candidates.Count == 0)
        {
            return false;
        }

        ClickControl(candidates[0]);
        return true;
    }

    private static bool IsButtonLike(object candidate)
    {
        return candidate is Godot.BaseButton || candidate.GetType().Name.Contains("Button", StringComparison.OrdinalIgnoreCase);
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
            BridgeDefaults.TransitionTimeout);

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
