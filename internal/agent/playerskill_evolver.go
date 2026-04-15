package agentruntime

import (
	"fmt"
	"strings"
)

// PlayerSkillAnalysis contains identified weaknesses in the current player skill document.
type PlayerSkillAnalysis struct {
	Weaknesses []PlayerSkillWeakness
	RunSummary RunSummary
}

// PlayerSkillWeakness describes a specific area the player skill document needs improvement.
type PlayerSkillWeakness struct {
	Section     string // "Playstyle Directives" | "Known Weaknesses" | "Signature Patterns" | "Performance History"
	Severity    string // "critical" | "major" | "minor"
	Description string
	Evidence    []string
	NewBullet   string // exact text to append to the section
}

// AnalyzePlayerSkill loads recent reflections and identifies weaknesses
// in the player skill document, returning proposed section-level edits.
func AnalyzePlayerSkill(artifactsRoot, playerSkillPath string, recentN int) (*PlayerSkillAnalysis, error) {
	analysis := &PlayerSkillAnalysis{}

	reflections, err := LoadRecentAttemptReflections(artifactsRoot, "", recentN)
	if err != nil {
		return nil, err
	}

	analysis.RunSummary = computeRunSummary(reflections)
	analysis.Weaknesses = identifyPlayerSkillWeaknesses(reflections, &analysis.RunSummary)

	return analysis, nil
}

// identifyPlayerSkillWeaknesses maps reflection patterns to player skill sections.
func identifyPlayerSkillWeaknesses(reflections []*AttemptReflection, summary *RunSummary) []PlayerSkillWeakness {
	var weaknesses []PlayerSkillWeakness

	hpDeaths := 0
	goldWastes := 0
	earlyDeaths := 0
	lowDefensives := 0
	noScalings := 0
	eliteRushes := 0

	// Track what worked
	goodDefense := 0
	goodRemoval := 0

	for _, r := range reflections {
		floor := 0
		if r.Floor != nil {
			floor = *r.Floor
		}

		for _, lesson := range r.Lessons {
			lower := strings.ToLower(lesson)
			switch {
			case strings.Contains(lower, "hp") || strings.Contains(lower, "health") || strings.Contains(lower, "critical"):
				if r.Outcome == "defeat" {
					hpDeaths++
				}
			case strings.Contains(lower, "gold") || strings.Contains(lower, "shop") || strings.Contains(lower, "unspent"):
				if r.Outcome == "defeat" {
					goldWastes++
				}
			case strings.Contains(lower, "defense") || strings.Contains(lower, "block") || strings.Contains(lower, "defensive"):
				if r.Outcome == "defeat" {
					lowDefensives++
				} else {
					goodDefense++
				}
			case strings.Contains(lower, "scale") || strings.Contains(lower, "power"):
				if r.Outcome == "defeat" {
					noScalings++
				}
			case strings.Contains(lower, "removal") || strings.Contains(lower, "remove") || strings.Contains(lower, "strike"):
				if r.Outcome != "defeat" {
					goodRemoval++
				}
			}
		}

		if r.Outcome == "defeat" && floor <= 7 {
			earlyDeaths++
		}

		for _, risk := range r.Risks {
			lower := strings.ToLower(risk)
			if strings.Contains(lower, "elite") && floor <= 5 {
				eliteRushes++
			}
		}
	}

	// HP deaths → strengthen Playstyle Directives
	if hpDeaths >= 2 {
		weaknesses = append(weaknesses, PlayerSkillWeakness{
			Section:     "Playstyle Directives",
			Severity:    "critical",
			Description: fmt.Sprintf("%d recent deaths from HP collapse — survival directive needs reinforcement", hpDeaths),
			Evidence:    []string{fmt.Sprintf("%d HP-related deaths in last %d runs", hpDeaths, len(reflections))},
			NewBullet:   "When HP < 30%, play only block cards unless you have lethal this turn; never trade HP for damage",
		})
	}

	// Gold waste → Known Weaknesses
	if goldWastes >= 2 {
		weaknesses = append(weaknesses, PlayerSkillWeakness{
			Section:     "Known Weaknesses",
			Severity:    "major",
			Description: fmt.Sprintf("%d runs ended with significant unspent gold", goldWastes),
			Evidence:    []string{fmt.Sprintf("%d gold-waste incidents in last %d runs", goldWastes, len(reflections))},
			NewBullet:   "Failing to spend gold at shops — treat unspent gold as a missed upgrade",
		})
	}

	// Early deaths → Playstyle Directives
	if earlyDeaths >= 2 {
		weaknesses = append(weaknesses, PlayerSkillWeakness{
			Section:     "Playstyle Directives",
			Severity:    "critical",
			Description: fmt.Sprintf("%d deaths before floor 7 — early game is too aggressive", earlyDeaths),
			Evidence:    []string{fmt.Sprintf("%d early deaths in last %d runs", earlyDeaths, len(reflections))},
			NewBullet:   "In floors 1-6, prioritize deck stability over power; avoid elites unless HP > 70%",
		})
	}

	// Low defense → Playstyle Directives
	if lowDefensives >= 2 {
		weaknesses = append(weaknesses, PlayerSkillWeakness{
			Section:     "Playstyle Directives",
			Severity:    "major",
			Description: "Defense consistently underprioritized in card picks",
			Evidence:    []string{fmt.Sprintf("%d low-defense incidents in last %d runs", lowDefensives, len(reflections))},
			NewBullet:   "At each reward, if deck has fewer than 3 block/defense cards, prioritize one over damage",
		})
	}

	// No scaling → Known Weaknesses
	if noScalings >= 2 {
		weaknesses = append(weaknesses, PlayerSkillWeakness{
			Section:     "Known Weaknesses",
			Severity:    "minor",
			Description: "Scaling cards not acquired early enough",
			Evidence:    []string{fmt.Sprintf("%d no-scaling incidents in last %d runs", noScalings, len(reflections))},
			NewBullet:   "Neglecting scaling cards in floors 8-12 — midgame power gap leads to Act 2 collapse",
		})
	}

	// Elite rush → Known Weaknesses
	if eliteRushes >= 2 {
		weaknesses = append(weaknesses, PlayerSkillWeakness{
			Section:     "Known Weaknesses",
			Severity:    "major",
			Description: "Rushing elites too early in the run",
			Evidence:    []string{fmt.Sprintf("%d early elite encounters in last %d runs", eliteRushes, len(reflections))},
			NewBullet:   "Taking elite nodes before floor 5 when HP < 80% — high variance, low reward",
		})
	}

	// What worked → Signature Patterns
	if goodDefense >= 2 {
		weaknesses = append(weaknesses, PlayerSkillWeakness{
			Section:     "Signature Patterns (What Works)",
			Severity:    "minor",
			Description: "Defense-first approach correlated with better outcomes",
			Evidence:    []string{fmt.Sprintf("%d runs where defense focus helped", goodDefense)},
			NewBullet:   "Prioritizing block cards in Act 1 correlates with surviving to Act 2",
		})
	}

	if goodRemoval >= 2 {
		weaknesses = append(weaknesses, PlayerSkillWeakness{
			Section:     "Signature Patterns (What Works)",
			Severity:    "minor",
			Description: "Card removal correlated with better outcomes",
			Evidence:    []string{fmt.Sprintf("%d runs where removal helped", goodRemoval)},
			NewBullet:   "Early Strike removal (floors 1-5) consistently improves draw quality",
		})
	}

	return weaknesses
}

// TopPlayerSkillWeakness returns the highest-priority weakness, or nil if none.
func (a *PlayerSkillAnalysis) TopWeakness() *PlayerSkillWeakness {
	severityOrder := map[string]int{"critical": 0, "major": 1, "minor": 2}
	var top *PlayerSkillWeakness
	for i := range a.Weaknesses {
		w := &a.Weaknesses[i]
		if top == nil || severityOrder[w.Severity] < severityOrder[top.Severity] {
			top = w
		}
	}
	return top
}
