using MegaCrit.Sts2.Core.Runs;
using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Game;

internal static class SelectionSectionBuilder
{
    public static SelectionSummary? Build(object? currentScreen, RunState? runState)
    {
        var selectionContext = GameUiAccess.ResolveSelectionContext(currentScreen);
        if (selectionContext == null)
        {
            return null;
        }

        var options = GameUiAccess.GetDeckSelectionOptions(currentScreen);
        if (options.Count == 0)
        {
            return null;
        }

        var cards = options
            .Select((holder, index) => CardSummaryBuilder.Build(holder.CardModel, index, sourceNode: holder))
            .ToList();

        var prompt = selectionContext.Prompt ?? GameUiAccess.GetDeckSelectionPrompt(currentScreen);
        if (string.IsNullOrWhiteSpace(prompt) && selectionContext.IsCombatEmbedded)
        {
            prompt = "Choose a card to continue combat.";
        }

        var requiresConfirmation = selectionContext.RequiresConfirmation ?? GameUiAccess.DeckSelectionRequiresConfirmation(currentScreen);
        var canConfirm = selectionContext.CanConfirm ?? GameUiAccess.TryGetDeckSelectionConfirmButton(currentScreen, out _);
        if (string.Equals(selectionContext.Kind, "NSimpleCardSelectScreen", StringComparison.Ordinal))
        {
            requiresConfirmation = false;
            canConfirm = false;
        }
        else if (!requiresConfirmation)
        {
            // Keep the serialized contract conservative: expose confirm only when
            // the selection flow truly requires an explicit confirmation step.
            canConfirm = false;
        }

        return new SelectionSummary
        {
            Kind = selectionContext.Kind,
            SourceScreen = GameUiAccess.ResolveSelectionSourceScreen(selectionContext, runState),
            SourceHint = GameUiAccess.ResolveSelectionSourceHint(selectionContext, runState),
            Mode = selectionContext.Mode,
            IsCombatEmbedded = selectionContext.IsCombatEmbedded,
            Prompt = prompt,
            MinSelect = selectionContext.MinSelect ?? 1,
            MaxSelect = selectionContext.MaxSelect ?? 1,
            SelectedCount = selectionContext.SelectedCount ?? 0,
            RequiresConfirmation = requiresConfirmation,
            CanConfirm = canConfirm,
            Cards = cards
        };
    }
}
