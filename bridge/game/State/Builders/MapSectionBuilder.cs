using MegaCrit.Sts2.Core.Nodes.Screens.Map;
using MegaCrit.Sts2.Core.Nodes.Screens.ScreenContext;
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

internal static class MapSectionBuilder
{
    public static MapSummary? Build(RunState? runState)
    {
        if (runState == null)
        {
            return null;
        }

        var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();
        if (!GameUiAccess.TryGetMapScreen(currentScreen, runState, out var mapScreen))
        {
            return new MapSummary
            {
                CurrentNode = BuildCoordNode(ReflectionUtils.GetMemberValue(runState, "CurrentMapCoord")),
                IsTravelEnabled = NMapScreen.Instance?.IsTravelEnabled,
                IsTraveling = NMapScreen.Instance?.IsTraveling,
                AvailableNodes = Array.Empty<MapNodeSummary>()
            };
        }

        var children = GameUiAccess.GetAvailableMapNodes(currentScreen, runState)
            .Select((node, index) => BuildNode(node.Point, index))
            .ToList();

        return new MapSummary
        {
            CurrentNode = BuildCoordNode(ReflectionUtils.GetMemberValue(runState, "CurrentMapCoord")),
            IsTravelEnabled = mapScreen!.IsTravelEnabled,
            IsTraveling = mapScreen.IsTraveling,
            AvailableNodes = children
        };
    }

    private static MapNodeSummary BuildNode(object point, int index)
    {
        var coord = ReflectionUtils.GetMemberValue(point, "coord");
        return new MapNodeSummary
        {
            Index = index,
            Row = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(coord, "row", "Row")),
            Col = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(coord, "col", "Col")),
            NodeType = ResolveNodeType(point)
        };
    }

    private static MapNodeSummary? BuildCoordNode(object? coord)
    {
        if (coord == null)
        {
            return null;
        }

        return new MapNodeSummary
        {
            Index = 0,
            Row = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(coord, "row", "Row")),
            Col = ReflectionUtils.ToNullableInt(ReflectionUtils.GetMemberValue(coord, "col", "Col"))
        };
    }

    private static string? ResolveNodeType(object point)
    {
        var directType = NormalizeNodeType(
            ReflectionUtils.GetMemberValue(point, "PointType", "RoomType", "Type", "NodeType")?.ToString());
        if (!string.IsNullOrWhiteSpace(directType))
        {
            return directType;
        }

        var room = ReflectionUtils.GetMemberValue(point, "Room", "RoomModel", "MapRoom", "Destination", "Encounter");
        var roomType = NormalizeNodeType(
            ReflectionUtils.GetMemberValue(room, "PointType", "RoomType", "Type", "NodeType")?.ToString()
            ?? room?.GetType().Name);
        if (!string.IsNullOrWhiteSpace(roomType))
        {
            return roomType;
        }

        var rewards = ReflectionUtils.ToNullableBool(ReflectionUtils.GetMemberValue(point, "HasRewards"));
        if (rewards == true)
        {
            return "Reward";
        }

        return null;
    }

    private static string? NormalizeNodeType(string? rawValue)
    {
        if (string.IsNullOrWhiteSpace(rawValue))
        {
            return null;
        }

        var value = rawValue.Trim();
        if (value.StartsWith("N", StringComparison.Ordinal) && value.EndsWith("Room", StringComparison.Ordinal))
        {
            value = value[1..^4];
        }

        return value switch
        {
            "Merchant" or "Shop" or "MerchantRoom" => "Shop",
            "RestSite" or "Rest" or "RestSiteRoom" => "Rest",
            "Treasure" or "TreasureRoom" or "Chest" => "Chest",
            "Event" or "Question" or "QuestionMark" or "EventRoom" => "Event",
            "Combat" or "CombatRoom" => "Combat",
            "EliteCombat" or "Elite" => "Elite",
            "BossCombat" or "Boss" => "Boss",
            "Unknown" => null,
            _ => value
        };
    }
}
