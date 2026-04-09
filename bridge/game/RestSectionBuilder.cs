using MegaCrit.Sts2.Core.Nodes.Rooms;
using MegaCrit.Sts2.Core.Runs;
using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Game;

internal static class RestSectionBuilder
{
    public static RestSummary? Build(object? currentScreen)
    {
        if (currentScreen is not NRestSiteRoom)
        {
            return null;
        }

        try
        {
            var synchronizer = ReflectionUtils.GetMemberValue(RunManager.Instance, "RestSiteSynchronizer");
            var options = ReflectionUtils.Enumerate(ReflectionUtils.InvokeMethod(synchronizer, "GetLocalOptions")).ToList();

            return new RestSummary
            {
                Options = options.Select((option, index) => new RestOptionSummary
                {
                    Index = index,
                    OptionId = ReflectionUtils.GetMemberValue<string>(option, "OptionId"),
                    Title = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(option, "Title")),
                    Description = ReflectionUtils.LocalizedText(ReflectionUtils.GetMemberValue(option, "Description")),
                    IsEnabled = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(option, "IsEnabled"))
                }).ToList()
            };
        }
        catch
        {
            return null;
        }
    }
}
