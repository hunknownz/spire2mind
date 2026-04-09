using MegaCrit.Sts2.Core.Nodes;
using MegaCrit.Sts2.Core.Runs;
using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Game;

internal static class CharacterSelectSectionBuilder
{
    public static CharacterSelectSummary? Build()
    {
        if (RunManager.Instance.DebugOnlyGetState() != null)
        {
            return null;
        }

        var screen = GameUiAccess.GetCharacterSelectScreen();
        var buttons = GameUiAccess.GetCharacterSelectButtons();

        if (buttons.Count == 0)
        {
            return null;
        }
        var selectedCharacterId = ReflectionUtils.ModelId(
            ReflectionUtils.GetMemberValue(
                ReflectionUtils.GetMemberValue(
                    ReflectionUtils.GetMemberValue(screen, "Lobby"),
                    "LocalPlayer"),
                "character",
                "Character"));

        return new CharacterSelectSummary
        {
            SelectedCharacterId = selectedCharacterId,
            Characters = buttons.Select((button, index) => new CharacterOptionSummary
            {
                Index = index,
                CharacterId = ReflectionUtils.ModelId(ReflectionUtils.GetMemberValue(button, "Character")),
                Name = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(button, "Character")),
                IsLocked = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(button, "IsLocked")),
                IsSelected = string.Equals(
                    ReflectionUtils.ModelId(ReflectionUtils.GetMemberValue(button, "Character")),
                    selectedCharacterId,
                    StringComparison.Ordinal),
                IsRandom = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(button, "IsRandom"))
            }).ToList(),
            CanEmbark = UiControlHelper.IsAvailable(GameUiAccess.GetCharacterEmbarkButton())
        };
    }
}
