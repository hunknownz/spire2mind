using Godot;
using MegaCrit.Sts2.Core.Nodes;
using MegaCrit.Sts2.Core.Nodes.CommonUi;
using MegaCrit.Sts2.Core.Nodes.Rooms;
using MegaCrit.Sts2.Core.Nodes.Screens.CharacterSelect;
using MegaCrit.Sts2.Core.Nodes.Screens.GameOverScreen;
using MegaCrit.Sts2.Core.Nodes.Screens.MainMenu;
using MegaCrit.Sts2.Core.Nodes.Screens.ScreenContext;
using MegaCrit.Sts2.Core.Runs;

namespace Spire2Mind.Bridge.Game.Ui;

internal static partial class GameUiAccess
{
    public static NMainMenuTextButton? GetMainMenuContinueButton(NMainMenu mainMenu)
    {
        return mainMenu.GetNodeOrNull<NMainMenuTextButton>("MainMenuTextButtons/ContinueButton");
    }

    public static NMainMenuTextButton? GetMainMenuAbandonRunButton(NMainMenu mainMenu)
    {
        return mainMenu.GetNodeOrNull<NMainMenuTextButton>("MainMenuTextButtons/AbandonRunButton");
    }

    public static NMainMenuTextButton? GetMainMenuSingleplayerButton(NMainMenu mainMenu)
    {
        return mainMenu.GetNodeOrNull<NMainMenuTextButton>("MainMenuTextButtons/SingleplayerButton");
    }

    public static NMainMenuTextButton? GetMainMenuMultiplayerButton(NMainMenu mainMenu)
    {
        return mainMenu.GetNodeOrNull<NMainMenuTextButton>("MainMenuTextButtons/MultiplayerButton");
    }

    public static NMainMenuTextButton? GetMainMenuTimelineButton(NMainMenu mainMenu)
    {
        return mainMenu.GetNodeOrNull<NMainMenuTextButton>("MainMenuTextButtons/TimelineButton");
    }

    public static NMainMenuTextButton? GetMainMenuSettingsButton(NMainMenu mainMenu)
    {
        return mainMenu.GetNodeOrNull<NMainMenuTextButton>("MainMenuTextButtons/SettingsButton");
    }

    public static NMainMenuTextButton? GetMainMenuProfileButton(NMainMenu mainMenu)
    {
        return mainMenu.GetNodeOrNull<NMainMenuTextButton>("MainMenuTextButtons/ProfileButton");
    }

    public static NMainMenuTextButton? GetMainMenuPatchNotesButton(NMainMenu mainMenu)
    {
        return mainMenu.GetNodeOrNull<NMainMenuTextButton>("MainMenuTextButtons/PatchNotesButton");
    }

    public static NMainMenuTextButton? GetMainMenuQuitButton(NMainMenu mainMenu)
    {
        return mainMenu.GetNodeOrNull<NMainMenuTextButton>("MainMenuTextButtons/QuitButton");
    }

    public static NCharacterSelectScreen? GetCharacterSelectScreen()
    {
        var mainMenu = GetMainMenu();
        if (mainMenu == null)
        {
            return null;
        }

        return FindDescendants<NCharacterSelectScreen>(mainMenu)
            .FirstOrDefault(node => GodotObject.IsInstanceValid(node) && node.IsVisibleInTree());
    }

    public static IReadOnlyList<NCharacterSelectButton> GetCharacterSelectButtons()
    {
        var screen = GetCharacterSelectScreen();
        if (screen == null)
        {
            return Array.Empty<NCharacterSelectButton>();
        }

        return FindDescendants<NCharacterSelectButton>(screen)
            .Where(node => GodotObject.IsInstanceValid(node))
            .OrderBy(node => node.GlobalPosition.Y)
            .ThenBy(node => node.GlobalPosition.X)
            .ToArray();
    }

    public static NConfirmButton? GetCharacterEmbarkButton()
    {
        return GetCharacterSelectScreen()?.GetNodeOrNull<NConfirmButton>("ConfirmButton");
    }

    public static Node? GetGameOverContinueButton(IScreenContext? currentScreen)
    {
        return currentScreen is NGameOverScreen screen
            ? screen.GetNodeOrNull<Node>("%ContinueButton")
            : null;
    }

    public static Node? GetGameOverMainMenuButton(IScreenContext? currentScreen)
    {
        return currentScreen is NGameOverScreen screen
            ? screen.GetNodeOrNull<Node>("%MainMenuButton")
            : null;
    }

    public static Node? GetProceedButton(IScreenContext? currentScreen)
    {
        if (currentScreen == null || currentScreen.GetType().Name == "NCardRewardSelectionScreen")
        {
            return null;
        }

        if (currentScreen.GetType().Name == "NRewardsScreen")
        {
            var rewardProceedButton = GetRewardProceedButton(currentScreen);
            return IsProceedButtonUsable(rewardProceedButton) ? rewardProceedButton : null;
        }

        TryShowRestProceedButtonIfComplete(currentScreen);
        TryShowTreasureProceedButtonIfComplete(currentScreen);

        if (currentScreen is IRoomWithProceedButton roomWithProceedButton &&
            IsProceedButtonUsable(roomWithProceedButton.ProceedButton))
        {
            return roomWithProceedButton.ProceedButton;
        }

        var directProceedButton = ReflectionUtils.GetMemberValue(currentScreen, "ProceedButton", "_proceedButton");
        if (IsProceedButtonUsable(directProceedButton as Node))
        {
            return directProceedButton as Node;
        }

        if (currentScreen is not Node rootNode)
        {
            return null;
        }

        return ReflectionUtils.DescendantsByTypeName(rootNode, "NProceedButton").FirstOrDefault(IsProceedButtonUsable);
    }

    public static Node? GetModalConfirmButton()
    {
        return FindModalButton(
            ConfirmKeywords,
            "VerticalPopup/YesButton",
            "ConfirmButton",
            "RightArrow",
            "%ConfirmButton",
            "%Confirm",
            "%AcknowledgeButton",
            "%RightArrow",
            "%OkButton");
    }

    public static Node? GetModalCancelButton()
    {
        return FindModalButton(
            DismissKeywords,
            "VerticalPopup/NoButton",
            "CancelButton",
            "LeftArrow",
            "%CancelButton",
            "%BackButton",
            "%LeftArrow",
            "%CloseButton");
    }

    private static Node? FindModalButton(string[] keywords, params string[] paths)
    {
        if (GetOpenModal() is not Node modalNode)
        {
            return null;
        }

        foreach (var path in paths)
        {
            var button = modalNode.GetNodeOrNull<Node>(path);
            if (button != null && GodotObject.IsInstanceValid(button) && ReflectionUtils.IsAvailable(button))
            {
                return button;
            }
        }

        var directControl = keywords == ConfirmKeywords
            ? ReflectionUtils.GetMemberValue(modalNode, "_nextButton", "NextButton", "_confirmButton", "ConfirmButton", "_yesButton", "YesButton")
            : ReflectionUtils.GetMemberValue(modalNode, "_prevButton", "PrevButton", "_dismissButton", "DismissButton", "_cancelButton", "CancelButton", "_noButton", "NoButton");
        if (directControl is Node directNode &&
            GodotObject.IsInstanceValid(directNode) &&
            ReflectionUtils.IsAvailable(directNode))
        {
            return directNode;
        }

        var candidates = ReflectionUtils.Descendants(modalNode)
            .Where(candidate =>
                GodotObject.IsInstanceValid(candidate) &&
                ReflectionUtils.IsAvailable(candidate) &&
                IsButtonLike(candidate))
            .ToArray();

        var matched = candidates
            .Where(candidate => MatchesKeywords(candidate, keywords))
            .OrderBy(candidate => candidate.GetPath().ToString().Length)
            .FirstOrDefault();

        if (matched != null)
        {
            return matched;
        }

        if (keywords == ConfirmKeywords && candidates.Length == 1)
        {
            return candidates[0];
        }

        return null;
    }

    private static bool IsButtonLike(Node candidate)
    {
        return candidate is BaseButton || candidate.GetType().Name.Contains("Button", StringComparison.OrdinalIgnoreCase);
    }

    private static bool MatchesKeywords(Node candidate, IEnumerable<string> keywords)
    {
        var haystacks = new[]
        {
            candidate.Name.ToString(),
            candidate.GetType().Name,
            ReflectionUtils.LocalizedText(candidate),
            ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(candidate, "Text", "Label", "Title"))
        };

        return haystacks
            .Where(value => !string.IsNullOrWhiteSpace(value))
            .Select(value => value!.ToLowerInvariant())
            .Any(value => keywords.Any(value.Contains));
    }

    private static bool IsProceedButtonUsable(Node? button)
    {
        return button != null &&
               GodotObject.IsInstanceValid(button) &&
               ReflectionUtils.IsAvailable(button);
    }

    private static void TryShowRestProceedButtonIfComplete(IScreenContext? currentScreen)
    {
        if (currentScreen is not NRestSiteRoom restSiteRoom)
        {
            return;
        }

        if (IsProceedButtonUsable(restSiteRoom.ProceedButton))
        {
            return;
        }

        try
        {
            var synchronizer = ReflectionUtils.GetMemberValue(RunManager.Instance, "RestSiteSynchronizer");
            var options = ReflectionUtils.Enumerate(ReflectionUtils.InvokeMethod(synchronizer, "GetLocalOptions")).ToList();
            if (options.Count != 0)
            {
                return;
            }

            restSiteRoom.Call(NRestSiteRoom.MethodName.ShowProceedButton);
            ActiveScreenContext.Instance.Update();
        }
        catch
        {
            // Best-effort UI sync only; fall back to existing proceed button discovery.
        }
    }

    private static void TryShowTreasureProceedButtonIfComplete(IScreenContext? currentScreen)
    {
        if (currentScreen is not NTreasureRoom treasureRoom)
        {
            return;
        }

        if (currentScreen is IRoomWithProceedButton roomWithProceedButton &&
            IsProceedButtonUsable(roomWithProceedButton.ProceedButton))
        {
            return;
        }

        var chestButton = treasureRoom.GetNodeOrNull<Node>("%Chest");
        var isOpened = chestButton == null || !GodotObject.IsInstanceValid(chestButton) || ReflectionUtils.IsEnabled(chestButton) == false;
        if (!isOpened)
        {
            return;
        }

        if (GetTreasureRelicCollection(currentScreen) != null)
        {
            return;
        }

        try
        {
            var synchronizer = ReflectionUtils.GetMemberValue(RunManager.Instance, "TreasureRoomRelicSynchronizer");
            var currentRelics = ReflectionUtils.Enumerate(ReflectionUtils.GetMemberValue(synchronizer, "CurrentRelics")).ToList();
            if (currentRelics.Count > 0)
            {
                return;
            }

            ReflectionUtils.InvokeMethod(treasureRoom, "ShowProceedButton");
            treasureRoom.Call("ShowProceedButton");
            ActiveScreenContext.Instance.Update();
        }
        catch
        {
            // Best-effort UI sync only; fall back to existing proceed button discovery.
        }
    }
}
