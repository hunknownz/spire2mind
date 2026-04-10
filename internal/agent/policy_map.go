package agentruntime

import (
	"strings"

	"spire2mind/internal/game"
)

func preferredMapNodeIndex(state *game.StateSnapshot) *int {
	if state == nil || state.Map == nil {
		return nil
	}
	nodes := state.Map.AvailableNodes
	if len(nodes) == 0 {
		return nil
	}

	type rankedNode struct {
		index int
		score float64
	}

	ranked := make([]rankedNode, 0, len(nodes))
	for _, node := range nodes {
		nodeMap := mapNodeToMap(node)
		estimate := estimateMapNodeDepth(state, nodeMap)
		ranked = append(ranked, rankedNode{
			index: node.Index,
			score: estimate.Score,
		})
	}

	if len(ranked) == 0 {
		return nil
	}

	best := ranked[0]
	for _, candidate := range ranked[1:] {
		if candidate.score > best.score || (candidate.score == best.score && candidate.index < best.index) {
			best = candidate
		}
	}

	return &best.index
}

func mapNodePriority(nodeType string, hpRatio float64, gold int, floor int, currentHP int, maxHP int) int {
	normalized := strings.ToLower(strings.TrimSpace(nodeType))
	if hpRatio >= 0.70 && gold >= 120 && floor <= 10 {
		switch normalized {
		case "shop":
			return 0
		case "event":
			return 1
		case "combat":
			return 2
		case "rest":
			return 3
		case "elite":
			return 4
		case "chest":
			return 5
		case "boss":
			return 6
		}
	}
	if hpRatio < 0.55 {
		switch normalized {
		case "rest":
			return 0
		case "shop":
			return 1
		case "event":
			return 2
		case "combat":
			return 3
		case "chest":
			return 4
		case "elite":
			return 5
		case "boss":
			return 6
		case "":
			return 10
		default:
			return 9
		}
	}
	if floor <= 8 && hpRatio < 0.70 && currentHP > 0 && maxHP > 0 {
		switch normalized {
		case "event":
			return 0
		case "combat":
			return 1
		case "rest":
			return 2
		case "shop":
			return 3
		case "elite":
			return 6
		}
	}

	switch normalized {
	case "shop":
		return 0
	case "event":
		return 1
	case "combat":
		return 2
	case "chest":
		return 3
	case "rest":
		return 4
	case "elite":
		return 5
	case "boss":
		return 6
	case "":
		return 10
	default:
		return 9
	}
}
