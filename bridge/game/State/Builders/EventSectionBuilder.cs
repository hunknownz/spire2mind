using MegaCrit.Sts2.Core.Runs;
using Spire2Mind.Bridge.Models;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.Ui;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.Util;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.Ui;
using Spire2Mind.Bridge.Game.Threading;

namespace Spire2Mind.Bridge.Game.State.Builders;

internal static class EventSectionBuilder
{
    public static EventSummary? Build(RunState? runState)
    {
        var room = ReflectionUtils.GetMemberValue(runState, "CurrentRoom");
        if (room?.GetType().Name != "EventRoom")
        {
            return null;
        }

        var eventModel = ReflectionUtils.GetMemberValue(room, "LocalMutableEvent", "CanonicalEvent");
        if (eventModel == null)
        {
            return null;
        }

        var options = ReflectionUtils.Enumerate(ReflectionUtils.GetMemberValue(eventModel, "CurrentOptions"))
            .Select((option, index) => new EventOptionSummary
            {
                Index = index,
                Title = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(option, "Title")),
                Description = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(option, "Description")),
                IsLocked = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(option, "IsLocked")),
                IsProceed = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(option, "IsProceed"))
            })
            .ToList();

        return new EventSummary
        {
            EventId = ReflectionUtils.ModelId(eventModel),
            Title = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(eventModel, "Title")),
            Description = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(eventModel, "Description", "InitialDescription")),
            IsFinished = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(eventModel, "IsFinished")),
            Options = options
        };
    }
}
