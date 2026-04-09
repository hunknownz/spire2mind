using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Game;

internal static class AvailableActionBuilder
{
    public static IReadOnlyList<string> Build(
        string screen,
        MegaCrit.Sts2.Core.Nodes.Screens.ScreenContext.IScreenContext? currentScreen,
        MainMenuState? mainMenu,
        CombatSummary? combat,
        bool canPlayCombatCard,
        bool canEndTurn,
        MapSummary? map,
        SelectionSummary? selection,
        ChestSummary? chest,
        ShopSummary? shop,
        RestSummary? rest,
        RewardSummary? reward,
        EventSummary? eventSummary,
        CharacterSelectSummary? characterSelect,
        GameOverSummary? gameOver,
        ModalSummary? modal)
    {
        var actions = new List<string>();

        if (modal != null)
        {
            if (modal.CanConfirm)
            {
                actions.Add(ActionIds.ConfirmModal);
            }

            if (modal.CanDismiss)
            {
                actions.Add(ActionIds.DismissModal);
            }

            return actions;
        }

        // During map travel, stale room nodes can remain visible for a few frames.
        // Suppress actions until the next room settles so the agent does not act on ghosts.
        if (map?.IsTraveling == true)
        {
            return actions;
        }

        switch (screen)
        {
            case ScreenIds.MainMenu:
                AddMainMenuActions(mainMenu, actions);
                break;

            case ScreenIds.CharacterSelect:
                if (characterSelect?.Characters.Count > 0)
                {
                    actions.Add(ActionIds.SelectCharacter);
                }
                if (characterSelect?.CanEmbark == true)
                {
                    actions.Add(ActionIds.Embark);
                }
                break;

            case ScreenIds.Map:
                if (map?.IsTravelEnabled == true && map.AvailableNodes.Count > 0)
                {
                    actions.Add(ActionIds.ChooseMapNode);
                }
                break;

            case ScreenIds.Chest:
                if (chest?.IsOpened == false)
                {
                    actions.Add(ActionIds.OpenChest);
                }
                if (chest?.RelicOptions.Count > 0 && chest.HasRelicBeenClaimed != true)
                {
                    actions.Add(ActionIds.ChooseTreasureRelic);
                }
                if (GameUiAccess.GetProceedButton(currentScreen) != null)
                {
                    actions.Add(ActionIds.Proceed);
                }
                break;

            case ScreenIds.Reward:
            case ScreenIds.CardSelection:
                var hasClaimableRewards = reward?.Rewards.Any(item => item.Claimable != false) == true;
                var hasRewardCardChoice = reward?.CardOptions.Count > 0;
                var hasGenericDeckSelection = selection?.Cards.Count > 0 && !hasRewardCardChoice;

                if (hasClaimableRewards)
                {
                    actions.Add(ActionIds.ClaimReward);
                }
                if (hasRewardCardChoice)
                {
                    actions.Add(ActionIds.ChooseRewardCard);
                    actions.Add(ActionIds.SkipRewardCards);
                }
                if (hasGenericDeckSelection)
                {
                    actions.Add(ActionIds.SelectDeckCard);
                }
                if (selection?.RequiresConfirmation == true && selection.CanConfirm == true)
                {
                    actions.Add(ActionIds.ConfirmSelection);
                }
                if (!hasClaimableRewards &&
                    !hasRewardCardChoice &&
                    !hasGenericDeckSelection &&
                    reward?.CanProceed == true)
                {
                    actions.Add(ActionIds.Proceed);
                }
                break;

            case ScreenIds.Event:
                if (eventSummary?.Options.Count > 0 || eventSummary?.IsFinished == true)
                {
                    actions.Add(ActionIds.ChooseEventOption);
                }
                break;

            case ScreenIds.Rest:
                if (rest?.Options.Any(option => option.IsEnabled == true) == true)
                {
                    actions.Add(ActionIds.ChooseRestOption);
                }
                if (GameUiAccess.GetProceedButton(currentScreen) != null)
                {
                    actions.Add(ActionIds.Proceed);
                }
                break;

            case ScreenIds.Shop:
                if (shop?.CanOpen == true)
                {
                    actions.Add(ActionIds.OpenShopInventory);
                }
                if (shop?.CanClose == true)
                {
                    actions.Add(ActionIds.CloseShopInventory);
                }
                if (shop?.IsOpen == true &&
                    shop.Cards.Any(card => card.IsStocked == true && card.EnoughGold == true))
                {
                    actions.Add(ActionIds.BuyCard);
                }
                if (shop?.IsOpen == true &&
                    shop.Relics.Any(relic => relic.IsStocked == true && relic.EnoughGold == true))
                {
                    actions.Add(ActionIds.BuyRelic);
                }
                if (shop?.IsOpen == true &&
                    shop.Potions.Any(potion => potion.IsStocked == true && potion.EnoughGold == true))
                {
                    actions.Add(ActionIds.BuyPotion);
                }
                if (shop?.IsOpen == true &&
                    shop.CardRemoval?.Available == true &&
                    shop.CardRemoval.EnoughGold == true)
                {
                    actions.Add(ActionIds.RemoveCardAtShop);
                }
                if (GameUiAccess.GetProceedButton(currentScreen) != null)
                {
                    actions.Add(ActionIds.Proceed);
                }
                break;

            case ScreenIds.Combat:
                var actionWindowOpen = selection == null && combat?.ActionWindowOpen == true;
                if (actionWindowOpen && canPlayCombatCard && combat?.Hand.Count > 0)
                {
                    actions.Add(ActionIds.PlayCard);
                }
                if (actionWindowOpen && canEndTurn)
                {
                    actions.Add(ActionIds.EndTurn);
                }
                break;

            case ScreenIds.GameOver:
                if (gameOver?.CanContinue == true)
                {
                    actions.Add(ActionIds.ContinueAfterGameOver);
                }
                if (gameOver?.CanReturnToMainMenu == true)
                {
                    actions.Add(ActionIds.ReturnToMainMenu);
                }
                break;
        }

        return actions;
    }

    private static void AddMainMenuActions(MainMenuState? mainMenu, ICollection<string> actions)
    {
        if (mainMenu == null)
        {
            return;
        }

        if (mainMenu.CanContinueRun)
        {
            actions.Add(ActionIds.ContinueRun);
        }
        if (mainMenu.CanAbandonRun)
        {
            actions.Add(ActionIds.AbandonRun);
        }
        if (mainMenu.CanOpenCharacterSelect)
        {
            actions.Add(ActionIds.OpenCharacterSelect);
        }
    }
}
