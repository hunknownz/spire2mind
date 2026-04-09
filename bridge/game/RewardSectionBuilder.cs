using Godot;
using MegaCrit.Sts2.Core.Runs;
using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Game;

internal static class RewardSectionBuilder
{
    public static RewardSummary? Build(object? currentScreen, RunState? runState)
    {
        var typeName = currentScreen?.GetType().Name;
        if (typeName != "NRewardsScreen" && typeName != "NCardRewardSelectionScreen")
        {
            return null;
        }

        var rewardItems = new List<RewardItemSummary>();
        var cardOptions = new List<CardSummary>();
        var alternatives = new List<AlternativeSummary>();
        bool? canProceed = null;
        var hasEmptyPotionSlot = GameUiAccess.HasEmptyPotionSlot(runState);

        if (currentScreen is Node rewardNode && typeName == "NRewardsScreen")
        {
            rewardItems = ReflectionUtils.DescendantsByTypeName(rewardNode, "NRewardButton")
                .Select((button, index) => BuildRewardItem(button, index, hasEmptyPotionSlot))
                .ToList();

            canProceed = UiControlHelper.HasAvailableControl(currentScreen, "_proceedButton");
        }

        if (typeName == "NCardRewardSelectionScreen")
        {
            var options = ReflectionUtils.Enumerate(ReflectionUtils.GetMemberValue(currentScreen, "_options"));
            cardOptions = options
                .Select((option, index) => CardSummaryBuilder.Build(ReflectionUtils.GetMemberValue(option, "Card"), index))
                .ToList();

            alternatives = ReflectionUtils.Enumerate(ReflectionUtils.GetMemberValue(currentScreen, "_extraOptions"))
                .Select((option, index) => new AlternativeSummary
                {
                    Index = index,
                    OptionId = ReflectionUtils.GetMemberValue<string>(option, "OptionId"),
                    Title = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(option, "Title"))
                })
                .ToList();
        }

        return new RewardSummary
        {
            Phase = ResolveRewardPhase(typeName, rewardItems, cardOptions, canProceed),
            SourceScreen = GameUiAccess.ResolveRewardSourceScreen(currentScreen, runState),
            SourceHint = GameUiAccess.ResolveRewardSourceHint(currentScreen, runState),
            ScreenType = typeName,
            PendingCardChoice = typeName == "NCardRewardSelectionScreen",
            CanProceed = canProceed,
            RewardCount = rewardItems.Count,
            ClaimableRewardCount = rewardItems.Count(item => item.Claimable != false),
            AlternativeCount = alternatives.Count,
            Rewards = rewardItems,
            CardOptions = cardOptions,
            Alternatives = alternatives
        };
    }

    private static string ResolveRewardPhase(
        string? screenTypeName,
        IReadOnlyCollection<RewardItemSummary> rewardItems,
        IReadOnlyCollection<CardSummary> cardOptions,
        bool? canProceed)
    {
        if (string.Equals(screenTypeName, "NCardRewardSelectionScreen", StringComparison.Ordinal))
        {
            return "card_choice";
        }

        if (rewardItems.Any(item => item.Claimable != false))
        {
            return "claim";
        }

        if (canProceed == true)
        {
            return "proceed";
        }

        if (cardOptions.Count > 0)
        {
            return "card_choice";
        }

        return "settling";
    }

    private static RewardItemSummary BuildRewardItem(Node rewardButtonNode, int index, bool hasEmptyPotionSlot)
    {
        var reward = ReflectionUtils.GetMemberValue(rewardButtonNode, "Reward");
        var rewardType = reward?.GetType().Name;
        var claimable = ReflectionUtils.IsAvailable(rewardButtonNode);
        string? blockedReason = null;

        if (claimable && IsPotionReward(rewardType) && !hasEmptyPotionSlot)
        {
            claimable = false;
            blockedReason = "full_potion_belt";
        }

        return new RewardItemSummary
        {
            Index = index,
            RewardType = rewardType,
            Description = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(reward, "Description")),
            Claimable = claimable,
            BlockedReason = blockedReason
        };
    }

    private static bool IsPotionReward(string? rewardType)
    {
        return !string.IsNullOrWhiteSpace(rewardType) &&
               rewardType.Contains("Potion", StringComparison.OrdinalIgnoreCase);
    }
}
