using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Game;

internal static class AgentViewBuilder
{
    public static AgentViewSummary Build(
        string screen,
        RunSummary? run,
        CombatSummary? combat,
        int? turn,
        IReadOnlyList<string> availableActions)
    {
        return new AgentViewSummary
        {
            Headline = BuildHeadline(screen, combat, availableActions),
            Floor = run?.Floor,
            Turn = turn,
            AvailableActionCount = availableActions.Count,
            HandCount = combat?.Hand.Count,
            EnemyCount = combat?.Enemies.Count
        };
    }

    private static string BuildHeadline(string screen, CombatSummary? combat, IReadOnlyList<string> availableActions)
    {
        if (combat != null)
        {
            return $"{screen}: {combat.Enemies.Count} enemies, {combat.Hand.Count} cards in hand";
        }

        return $"{screen}: {availableActions.Count} available actions";
    }
}
