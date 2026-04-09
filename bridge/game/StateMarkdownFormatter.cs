using System.Text;
using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Game;

internal static class StateMarkdownFormatter
{
    public static string FormatMarkdown(BridgeStateSnapshot snapshot)
    {
        var builder = new StringBuilder();

        builder.AppendLine("# Spire2Mind State");
        builder.AppendLine();
        builder.AppendLine($"- Screen: `{snapshot.Screen}`");
        builder.AppendLine($"- In combat: `{snapshot.InCombat}`");
        builder.AppendLine($"- Turn: `{snapshot.Turn?.ToString() ?? "-"}`");
        builder.AppendLine($"- Available actions: `{string.Join(", ", snapshot.AvailableActions)}`");

        if (snapshot.Run != null)
        {
            builder.AppendLine();
            builder.AppendLine("## Run");
            builder.AppendLine($"- Character: `{snapshot.Run.Character ?? "-"}`");
            builder.AppendLine($"- Floor: `{snapshot.Run.Floor?.ToString() ?? "-"}`");
            builder.AppendLine($"- HP: `{snapshot.Run.CurrentHp?.ToString() ?? "-"}` / `{snapshot.Run.MaxHp?.ToString() ?? "-"}`");
            builder.AppendLine($"- Gold: `{snapshot.Run.Gold?.ToString() ?? "-"}`");
        }

        if (snapshot.Combat != null)
        {
            builder.AppendLine();
            builder.AppendLine("## Combat");
            builder.AppendLine($"- Energy: `{snapshot.Combat.Player?.Energy?.ToString() ?? "-"}`");
            builder.AppendLine($"- Stars: `{snapshot.Combat.Player?.Stars?.ToString() ?? "-"}`");
            builder.AppendLine($"- Enemies: `{snapshot.Combat.Enemies.Count}`");
            builder.AppendLine($"- Hand: `{string.Join(", ", snapshot.Combat.Hand.Select(card => card.Name ?? card.CardId ?? $"card_{card.Index}"))}`");
        }

        if (snapshot.Map != null && snapshot.Map.AvailableNodes.Count > 0)
        {
            builder.AppendLine();
            builder.AppendLine("## Map");
            foreach (var node in snapshot.Map.AvailableNodes)
            {
                builder.AppendLine($"- [{node.Index}] `{node.NodeType}` at `({node.Row},{node.Col})`");
            }
        }

        if (snapshot.Reward != null)
        {
            builder.AppendLine();
            builder.AppendLine("## Reward");
            foreach (var reward in snapshot.Reward.Rewards)
            {
                var blockedSuffix = string.IsNullOrWhiteSpace(reward.BlockedReason)
                    ? string.Empty
                    : $" blocked=`{reward.BlockedReason}`";
                builder.AppendLine($"- [{reward.Index}] `{reward.RewardType}` {reward.Description} claimable=`{reward.Claimable}`{blockedSuffix}");
            }

            foreach (var card in snapshot.Reward.CardOptions)
            {
                builder.AppendLine($"- Card [{card.Index}] `{card.Name ?? card.CardId}`");
            }
        }

        if (snapshot.Event != null)
        {
            builder.AppendLine();
            builder.AppendLine("## Event");
            builder.AppendLine($"- Title: `{snapshot.Event.Title ?? snapshot.Event.EventId ?? "-"}`");
            foreach (var option in snapshot.Event.Options)
            {
                builder.AppendLine($"- [{option.Index}] `{option.Title ?? "-"}`");
            }
        }

        if (snapshot.CharacterSelect != null)
        {
            builder.AppendLine();
            builder.AppendLine("## Character Select");
            foreach (var option in snapshot.CharacterSelect.Characters)
            {
                builder.AppendLine($"- [{option.Index}] `{option.Name ?? option.CharacterId ?? "-"}` locked={option.IsLocked}");
            }
            builder.AppendLine($"- Can embark: `{snapshot.CharacterSelect.CanEmbark}`");
        }

        if (snapshot.Modal != null)
        {
            builder.AppendLine();
            builder.AppendLine("## Modal");
            builder.AppendLine($"- Type: `{snapshot.Modal.ModalType}`");
            builder.AppendLine($"- Title: `{snapshot.Modal.Title ?? "-"}`");
            builder.AppendLine($"- Description: `{snapshot.Modal.Description ?? "-"}`");
        }

        return builder.ToString().TrimEnd();
    }
}
