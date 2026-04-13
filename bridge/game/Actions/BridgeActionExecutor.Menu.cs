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
        var button = GameUiAccess.GetMainMenuSingleplayerButton(mainMenu);

        // Debug: log button state
        MegaCrit.Sts2.Core.Logging.Log.Info($"[Spire2Mind] SingleplayerButton: found={button != null}, available={UiControlHelper.IsAvailable(button)}");

        if (UiControlHelper.IsAvailable(button))
        {
            button!.ForceClick();
            MegaCrit.Sts2.Core.Logging.Log.Info("[Spire2Mind] Clicked SingleplayerButton via ForceClick");
        }
        else
        {
            // Try fallback: find any button that looks like "singleplayer" by text
            var fallbackButton = FindButtonByText(mainMenu, "singleplayer", "single", "solo", "单人");
            if (fallbackButton != null && UiControlHelper.IsAvailable(fallbackButton))
            {
                ClickControl(fallbackButton);
                MegaCrit.Sts2.Core.Logging.Log.Info("[Spire2Mind] Clicked fallback Singleplayer button");
            }
            else
            {
                var characterSelectScreen = mainMenu.SubmenuStack?.GetSubmenuType<NCharacterSelectScreen>();
                if (characterSelectScreen != null)
                {
                    characterSelectScreen.InitializeSingleplayer();
                    mainMenu.SubmenuStack!.Push(characterSelectScreen);
                    MegaCrit.Sts2.Core.Logging.Log.Info("[Spire2Mind] Pushed CharacterSelectScreen via submenu");
                }
                else
                {
                    MegaCrit.Sts2.Core.Logging.Log.Warn($"[Spire2Mind] No way to open character select. Button={button != null}, Submenu={mainMenu.SubmenuStack != null}");
                    throw StateUnavailable(ActionIds.OpenCharacterSelect, "Cannot open character select: button unavailable and no submenu.");
                }
            }
        }

        // StS 2 shows a run-mode selection screen (Standard / Daily / Custom) after
        // clicking SingleplayerButton. The mode selection appears as a submenu within
        // MAIN_MENU, not as a separate screen. We need to detect and auto-click Standard.
        var deadline = DateTime.UtcNow + BridgeDefaults.CombatActionTimeout;
        var modeSelectionHandled = false;

        while (DateTime.UtcNow < deadline)
        {
            await WaitForNextFrameAsync();

            var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
            var screenId = ScreenClassifier.Classify(currentScreen);

            MegaCrit.Sts2.Core.Logging.Log.Info($"[Spire2Mind] Polling: screen={screenId}, modeHandled={modeSelectionHandled}");

            if (screenId == ScreenIds.CharacterSelect)
            {
                MegaCrit.Sts2.Core.Logging.Log.Info("[Spire2Mind] CharacterSelect reached!");
                return BuildResult(ActionIds.OpenCharacterSelect, true);
            }

            // Try to click mode selection button
            if (!modeSelectionHandled)
            {
                // Debug: log what buttons we can see
                if (currentScreen is Node screenNode)
                {
                    var allButtons = ReflectionUtils.Descendants(screenNode)
                        .Where(n => GodotObject.IsInstanceValid(n) && IsButtonLike(n))
                        .Take(10)
                        .Select(n => new { name = n.Name.ToString(), available = ReflectionUtils.IsAvailable(n) })
                        .ToList();
                    MegaCrit.Sts2.Core.Logging.Log.Info($"[Spire2Mind] Found {allButtons.Count} buttons on {screenId}: {string.Join(", ", allButtons.Select(b => $"{b.name}(av={b.available})"))}");
                }

                if (TryClickModeSelectionButton(currentScreen, mainMenu))
                {
                    modeSelectionHandled = true;
                    MegaCrit.Sts2.Core.Logging.Log.Info("[Spire2Mind] Clicked mode selection button");
                }
                else
                {
                    // If TryClickModeSelectionButton failed, try clicking ANY available button
                    // This is a fallback for the mode selection screen
                    if (screenId == ScreenIds.Unknown && currentScreen is Node unknownScreen)
                    {
                        var anyButton = ReflectionUtils.Descendants(unknownScreen)
                            .FirstOrDefault(n => GodotObject.IsInstanceValid(n) && ReflectionUtils.IsAvailable(n) && IsButtonLike(n));
                        if (anyButton != null)
                        {
                            ClickControl(anyButton);
                            modeSelectionHandled = true;
                            MegaCrit.Sts2.Core.Logging.Log.Info($"[Spire2Mind] Clicked fallback button on UNKNOWN: {anyButton.Name}");
                        }
                    }
                }
            }
        }

        MegaCrit.Sts2.Core.Logging.Log.Warn($"[Spire2Mind] Timeout: CharacterSelect never appeared. Final screen={ScreenClassifier.Classify(ActiveScreenContext.Instance.GetCurrentScreen())}");
        throw StateUnavailable(ActionIds.OpenCharacterSelect, "CharacterSelect screen did not appear after opening character select.");
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
