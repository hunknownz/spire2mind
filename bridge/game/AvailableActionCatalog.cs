using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Game;

internal static class AvailableActionCatalog
{
    public static IReadOnlyList<AvailableActionDescriptor> Describe(IEnumerable<string> actions)
    {
        var descriptors = new List<AvailableActionDescriptor>();

        foreach (var action in actions.Distinct(StringComparer.Ordinal))
        {
            descriptors.Add(action switch
            {
                "open_character_select" => Descriptor(action, "Open the single-player character selection flow from the main menu."),
                "play_card" => Descriptor(action, "Play a card from hand.", new[] { "card_index" }, new[] { "target_index" }),
                "end_turn" => Descriptor(action, "End the current combat turn."),
                "continue_run" => Descriptor(action, "Continue the current save from the main menu."),
                "abandon_run" => Descriptor(action, "Abandon the current save from the main menu."),
                "open_multiplayer" => Descriptor(action, "Open the multiplayer submenu from the main menu."),
                "open_compendium" => Descriptor(action, "Open the compendium from the main menu."),
                "open_timeline" => Descriptor(action, "Open the timeline screen from the main menu."),
                "open_settings" => Descriptor(action, "Open the settings menu from the main menu."),
                "open_profile" => Descriptor(action, "Open the profile screen from the main menu."),
                "view_patch_notes" => Descriptor(action, "Open patch notes from the main menu."),
                "quit_game" => Descriptor(action, "Quit the game from the main menu."),
                "continue_after_game_over" => Descriptor(action, "Advance past the current game over screen when continue is available."),
                "return_to_main_menu" => Descriptor(action, "Return to the main menu from the game over screen."),
                "select_character" => Descriptor(action, "Choose a character on the character select screen.", new[] { "option_index" }),
                "embark" => Descriptor(action, "Start the run after character selection."),
                "choose_map_node" => Descriptor(action, "Travel to a reachable map node.", new[] { "option_index" }),
                "claim_reward" => Descriptor(action, "Claim a reward button from the rewards screen.", new[] { "option_index" }),
                "choose_reward_card" => Descriptor(action, "Choose a card reward option.", new[] { "option_index" }),
                "skip_reward_cards" => Descriptor(action, "Skip the current card reward selection."),
                "select_deck_card" => Descriptor(action, "Select a card in a generic deck-selection screen such as upgrade, transform, or enchant.", new[] { "option_index" }),
                "confirm_selection" => Descriptor(action, "Confirm the current deck or hand selection when the UI requires confirmation."),
                "proceed" => Descriptor(action, "Press the current proceed/continue button."),
                "choose_event_option" => Descriptor(action, "Choose an event option.", new[] { "option_index" }),
                "open_chest" => Descriptor(action, "Open the current treasure chest."),
                "choose_treasure_relic" => Descriptor(action, "Pick a relic from the current treasure chest.", new[] { "option_index" }),
                "choose_rest_option" => Descriptor(action, "Choose an option at the current rest site.", new[] { "option_index" }),
                "open_shop_inventory" => Descriptor(action, "Open the merchant inventory panel."),
                "close_shop_inventory" => Descriptor(action, "Close the merchant inventory panel."),
                "buy_card" => Descriptor(action, "Buy a card from the merchant inventory.", new[] { "option_index" }),
                "buy_relic" => Descriptor(action, "Buy a relic from the merchant inventory.", new[] { "option_index" }),
                "buy_potion" => Descriptor(action, "Buy a potion from the merchant inventory.", new[] { "option_index" }),
                "remove_card_at_shop" => Descriptor(action, "Use the merchant's card removal service."),
                "confirm_modal" => Descriptor(action, "Confirm the active modal dialog."),
                "dismiss_modal" => Descriptor(action, "Dismiss the active modal dialog."),
                _ => Descriptor(action, "Detected as available in the current UI state.")
            });
        }

        return descriptors;
    }

    private static AvailableActionDescriptor Descriptor(
        string action,
        string description,
        IReadOnlyList<string>? requiredParameters = null,
        IReadOnlyList<string>? optionalParameters = null)
    {
        return new AvailableActionDescriptor
        {
            Action = action,
            Description = description,
            RequiredParameters = requiredParameters ?? Array.Empty<string>(),
            OptionalParameters = optionalParameters ?? Array.Empty<string>()
        };
    }
}
