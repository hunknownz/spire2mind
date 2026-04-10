using MegaCrit.Sts2.Core.Combat;
using MegaCrit.Sts2.Core.Nodes.Rooms;
using MegaCrit.Sts2.Core.Nodes.Screens.ScreenContext;
using MegaCrit.Sts2.Core.Runs;
using Spire2Mind.Bridge.Models;
using Spire2Mind.Bridge.Game.Ui;
using Spire2Mind.Bridge.Game.Util;
using Spire2Mind.Bridge.Game.Ui;
using Spire2Mind.Bridge.Game.State.Builders;
using Spire2Mind.Bridge.Game.Ui;
using Spire2Mind.Bridge.Game.Util;
using Spire2Mind.Bridge.Game.Ui;

namespace Spire2Mind.Bridge.Game.State;

internal static class StateSnapshotBuilder
{
    public static BridgeStateSnapshot BuildBootstrap()
    {
        var availableActions = Array.Empty<string>();
        return new BridgeStateSnapshot
        {
            RunId = "run_unknown",
            Screen = ScreenIds.Unknown,
            InCombat = false,
            Turn = null,
            AvailableActions = availableActions,
            Session = new SessionSummary
            {
                Mode = "singleplayer",
                Phase = "starting",
                ControlScope = "local_player"
            },
            Run = null,
            Combat = null,
            Map = null,
            Selection = null,
            Reward = null,
            Event = null,
            CharacterSelect = null,
            Chest = null,
            Shop = null,
            Rest = null,
            Modal = null,
            GameOver = null,
            Multiplayer = null,
            MultiplayerLobby = null,
            Timeline = null,
            AgentView = AgentViewBuilder.Build(ScreenIds.Unknown, null, null, null, availableActions)
        };
    }

    public static BridgeStateSnapshot Build()
    {
        var runState = RunManager.Instance.DebugOnlyGetState();
        var combatState = CombatManager.Instance.DebugOnlyGetState();
        var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        var effectiveScreen = GameUiAccess.ResolveGameplayScreen(currentScreen, runState);
        var effectiveScreenContext = effectiveScreen as IScreenContext;
        var selection = TryBuild(() => SelectionSectionBuilder.Build(effectiveScreenContext ?? currentScreen, runState));
        var screen = ScreenClassifier.Classify(effectiveScreen);
        var isCombatScreen = string.Equals(screen, ScreenIds.Combat, StringComparison.Ordinal);
        var isCombatContext = combatState != null && effectiveScreen is NCombatRoom;

        var mainMenu = TryBuild(MainMenuSectionBuilder.Build);
        var run = TryBuild(() => RunSectionBuilder.Build(runState));
        var combat = isCombatContext ? TryBuild(() => CombatSectionBuilder.Build(effectiveScreen, combatState)) : null;
        var map = TryBuild(() => MapSectionBuilder.Build(runState));
        var reward = TryBuild(() => RewardSectionBuilder.Build(effectiveScreenContext ?? currentScreen, runState));
        var eventSummary = string.Equals(screen, ScreenIds.Event, StringComparison.Ordinal)
            ? TryBuild(() => EventSectionBuilder.Build(runState))
            : null;
        var characterSelect = TryBuild(CharacterSelectSectionBuilder.Build);
        var chest = TryBuild(() => ChestSectionBuilder.Build(effectiveScreenContext ?? effectiveScreen));
        var shop = TryBuild(() => ShopSectionBuilder.Build(effectiveScreenContext));
        var rest = TryBuild(() => RestSectionBuilder.Build(effectiveScreenContext ?? effectiveScreen));
        var modal = TryBuild(ModalSectionBuilder.Build);
        var gameOver = TryBuild(() => GameOverSectionBuilder.Build(effectiveScreenContext ?? effectiveScreen, runState));
        var turn = isCombatScreen ? CombatSectionBuilder.ResolveTurn(combatState) : null;
        // Combat action availability must evaluate the actual room node, not a nested
        // screen context, otherwise valid end-turn windows can disappear for a frame.
        var canPlayCombatCard = isCombatScreen && CombatActionAvailability.CanPlayAnyCard(effectiveScreen, combatState);
        var canEndTurn = isCombatScreen && CombatActionAvailability.CanEndTurn(effectiveScreen, combatState);
        var availableActions = AvailableActionBuilder.Build(
            screen,
            effectiveScreenContext,
            mainMenu,
            combat,
            canPlayCombatCard,
            canEndTurn,
            map,
            selection,
            chest,
            shop,
            rest,
            reward,
            eventSummary,
            characterSelect,
            gameOver,
            modal);

        return new BridgeStateSnapshot
        {
            RunId = ResolveRunId(runState),
            Screen = screen,
            InCombat = isCombatContext,
            Turn = turn,
            AvailableActions = availableActions,
            Session = SessionSectionBuilder.Build(screen, runState),
            Run = run,
            Combat = combat,
            Map = map,
            Selection = selection,
            Reward = reward,
            Event = eventSummary,
            CharacterSelect = characterSelect,
            Chest = chest,
            Shop = shop,
            Rest = rest,
            Modal = modal,
            GameOver = gameOver,
            Multiplayer = null,
            MultiplayerLobby = null,
            Timeline = null,
            AgentView = AgentViewBuilder.Build(screen, run, combat, turn, availableActions)
        };
    }

    private static string ResolveRunId(RunState? runState)
    {
        return ReflectionUtils.GetMemberValue<string>(
                   ReflectionUtils.GetMemberValue(runState, "Rng"),
                   "StringSeed")
               ?? "run_unknown";
    }

    private static T? TryBuild<T>(Func<T?> builder) where T : class
    {
        try
        {
            return builder();
        }
        catch
        {
            return null;
        }
    }
}
