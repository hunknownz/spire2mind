package analyst

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RunReview is the LLM deep analysis of a completed run.
type RunReview struct {
	RunID        string    `json:"run_id"`
	ReviewedAt   time.Time `json:"reviewed_at"`
	Floor        int       `json:"floor"`
	Outcome      string    `json:"outcome"` // defeat / victory
	Character    string    `json:"character"`
	DeckArchetype string   `json:"deck_archetype"` // what direction the deck was going
	RootCause    string    `json:"root_cause"`     // why the run ended
	KeyDecisions []KeyDecision `json:"key_decisions"`
	ActionableLessons []string `json:"actionable_lessons"` // concrete, specific
	NextRunPlan  string    `json:"next_run_plan"`       // specific rules for next run
	DeckAssessment string  `json:"deck_assessment"`     // was the deck direction correct?
}

// KeyDecision captures a single pivotal choice in the run.
type KeyDecision struct {
	Floor    int    `json:"floor"`
	Decision string `json:"decision"`
	Impact   string `json:"impact"` // critical_positive / major_positive / neutral / major_negative / critical_negative
	Lesson   string `json:"lesson"`
}

// runEvent is a minimal parsed event from events.jsonl.
type runEvent struct {
	Time    string          `json:"time"`
	Kind    string          `json:"kind"`
	Cycle   int             `json:"cycle"`
	Message string          `json:"message"`
	Screen  string          `json:"screen"`
	RunID   string          `json:"run_id"`
	Action  string          `json:"action"`
	Floor   int             `json:"floor"`
	State   json.RawMessage `json:"state"`
	Data    json.RawMessage `json:"data"`
}

// runSummary is the structured context extracted from a run directory.
type runSummary struct {
	RunID       string
	Floor       int
	Outcome     string
	Character   string
	FinalDeck   []string
	FinalRelics []string
	Gold        int
	MaxHP       int
	FinalHP     int
	HPTrajectory []hpPoint    // floor → HP
	CardPicks   []cardPick   // what cards were chosen at each floor
	ShopActions []shopAction // what was bought/skipped at shop
	EliteResults []eliteResult
	ReflectionLessons []string
}

type hpPoint struct {
	Floor int
	HP    int
	MaxHP int
}

type cardPick struct {
	Floor    int
	Chosen   string
	Options  []string
}

type shopAction struct {
	Floor  int
	Bought []string
	Gold   int
}

type eliteResult struct {
	Floor    int
	Enemy    string
	HPBefore int
	HPAfter  int
}

// ReviewRun performs a deep LLM retrospective of a completed run.
func (a *Analyst) ReviewRun(ctx context.Context, runDir string) (*RunReview, error) {
	// Step 1: find run directory
	dir, err := a.resolveRunDir(runDir)
	if err != nil {
		return nil, fmt.Errorf("resolve run dir: %w", err)
	}
	fmt.Printf("Reviewing run: %s\n", dir)

	// Step 2: extract structured summary from events
	summary, err := extractRunSummary(dir)
	if err != nil {
		return nil, fmt.Errorf("extract run summary: %w", err)
	}
	fmt.Printf("Run %s: floor %d, %s, deck %d cards\n",
		summary.RunID, summary.Floor, summary.Outcome, len(summary.FinalDeck))

	// Step 3: check if already reviewed
	lessonsPath := filepath.Join(a.knowledgeDir, "run_lessons", "history.json")
	existing := loadExistingReviews(lessonsPath)
	if _, ok := existing[summary.RunID]; ok {
		fmt.Printf("Run %s already reviewed\n", summary.RunID)
		r := existing[summary.RunID]
		return &r, nil
	}

	// Step 4: call LLM for deep analysis
	review, err := a.analyzeRun(ctx, summary)
	if err != nil {
		return nil, fmt.Errorf("LLM analysis: %w", err)
	}

	// Step 5: save to history
	existing[review.RunID] = *review
	if err := writeJSON(lessonsPath, existing); err != nil {
		return nil, fmt.Errorf("save review: %w", err)
	}

	printReview(review)
	return review, nil
}

// resolveRunDir finds the run directory by run_id or returns latest.
func (a *Analyst) resolveRunDir(hint string) (string, error) {
	runsRoot := filepath.Join(a.cfg.RepoRoot, "scratch", "agent-runs")

	if hint == "" || strings.ToLower(hint) == "latest" {
		return latestRunDir(runsRoot)
	}

	// Try as absolute or relative path
	if _, err := os.Stat(hint); err == nil {
		return hint, nil
	}

	// Try as run ID suffix match under agent-runs
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		return "", fmt.Errorf("read runs dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() && strings.Contains(e.Name(), hint) {
			return filepath.Join(runsRoot, e.Name()), nil
		}
	}

	return "", fmt.Errorf("run not found: %q", hint)
}

func latestRunDir(runsRoot string) (string, error) {
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		return "", fmt.Errorf("read runs dir: %w", err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(runsRoot, e.Name()))
		}
	}
	if len(dirs) == 0 {
		return "", fmt.Errorf("no run directories found in %s", runsRoot)
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i] > dirs[j] })
	return dirs[0], nil
}

// extractRunSummary parses events.jsonl and reflection JSON into a structured summary.
func extractRunSummary(dir string) (*runSummary, error) {
	summary := &runSummary{}

	// Load reflection JSON for basic outcome data
	reflections, _ := filepath.Glob(filepath.Join(dir, "attempt-*-reflection.json"))
	if len(reflections) > 0 {
		sort.Strings(reflections)
		data, err := os.ReadFile(reflections[len(reflections)-1])
		if err == nil {
			var ref struct {
				RunID     string   `json:"run_id"`
				Floor     int      `json:"floor"`
				Outcome   string   `json:"outcome"`
				Character string   `json:"character_id"`
				Lessons   []string `json:"lessons"`
			}
			if json.Unmarshal(data, &ref) == nil {
				summary.RunID = ref.RunID
				summary.Floor = ref.Floor
				summary.Outcome = ref.Outcome
				summary.Character = ref.Character
				summary.ReflectionLessons = ref.Lessons
			}
		}
	}

	// Parse events.jsonl for decision timeline
	eventsPath := filepath.Join(dir, "events.jsonl")
	data, err := os.ReadFile(eventsPath)
	if err != nil {
		return nil, fmt.Errorf("read events.jsonl: %w", err)
	}

	var lastState struct {
		Run *struct {
			Character string `json:"character"`
			Floor     int    `json:"floor"`
			CurrentHP int    `json:"currentHp"`
			MaxHP     int    `json:"maxHp"`
			Gold      int    `json:"gold"`
			Deck      []struct {
				CardID string `json:"cardId"`
				Name   string `json:"name"`
			} `json:"deck"`
			Relics []struct {
				RelicID string `json:"relicId"`
				Name    string `json:"name"`
			} `json:"relics"`
		} `json:"run"`
		AgentView *struct {
			Floor int `json:"floor"`
		} `json:"agentView"`
	}

	prevFloor := 0
	prevHP := 0

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var ev runEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}

		// Track state
		if len(ev.State) > 2 {
			json.Unmarshal(ev.State, &lastState)
		}

		// Track HP trajectory at floor transitions
		if lastState.Run != nil && lastState.AgentView != nil {
			floor := lastState.AgentView.Floor
			hp := lastState.Run.CurrentHP
			if floor != prevFloor && floor > 0 {
				summary.HPTrajectory = append(summary.HPTrajectory, hpPoint{
					Floor: floor,
					HP:    hp,
					MaxHP: lastState.Run.MaxHP,
				})
				prevFloor = floor
				prevHP = hp
			}
			_ = prevHP
		}

		// Track card picks from data field
		if ev.Action == "choose_reward_card" && ev.Kind == "tool" {
			var d struct {
				CardIndex int `json:"card_index"`
			}
			json.Unmarshal(ev.Data, &d)
			floor := 0
			if lastState.AgentView != nil {
				floor = lastState.AgentView.Floor
			}
			// Get chosen card name from current deck snapshot
			pick := cardPick{Floor: floor}
			if lastState.Run != nil && d.CardIndex >= 0 && d.CardIndex < len(lastState.Run.Deck) {
				pick.Chosen = lastState.Run.Deck[d.CardIndex].Name
			}
			summary.CardPicks = append(summary.CardPicks, pick)
		}

		// Track shop purchases
		if ev.Screen == "SHOP" && ev.Action == "buy_card" && ev.Kind == "tool" {
			floor := 0
			if lastState.AgentView != nil {
				floor = lastState.AgentView.Floor
			}
			gold := 0
			if lastState.Run != nil {
				gold = lastState.Run.Gold
			}
			if len(summary.ShopActions) == 0 || summary.ShopActions[len(summary.ShopActions)-1].Floor != floor {
				summary.ShopActions = append(summary.ShopActions, shopAction{Floor: floor, Gold: gold})
			}
		}
	}

	// Final state
	if lastState.Run != nil {
		if summary.Character == "" {
			summary.Character = lastState.Run.Character
		}
		if summary.Floor == 0 {
			summary.Floor = lastState.Run.Floor
		}
		summary.FinalHP = lastState.Run.CurrentHP
		summary.MaxHP = lastState.Run.MaxHP
		summary.Gold = lastState.Run.Gold
		for _, c := range lastState.Run.Deck {
			summary.FinalDeck = append(summary.FinalDeck, c.Name)
		}
		for _, r := range lastState.Run.Relics {
			summary.FinalRelics = append(summary.FinalRelics, r.Name)
		}
	}

	return summary, nil
}

// analyzeRun calls LLM with a structured run summary for deep analysis.
func (a *Analyst) analyzeRun(ctx context.Context, s *runSummary) (*RunReview, error) {
	systemPrompt := `你是杀戮尖塔 2（Slay the Spire 2）的复盘教练，专注于铁甲战士（Ironclad）。
你的任务是对一局游戏进行深度复盘，找出关键错误和可操作的改进建议。
只返回 JSON，不要输出其他文本。`

	// Build HP trajectory string
	var hpLines []string
	for _, h := range s.HPTrajectory {
		pct := 0
		if h.MaxHP > 0 {
			pct = h.HP * 100 / h.MaxHP
		}
		hpLines = append(hpLines, fmt.Sprintf("  Floor %d: %d/%d (%d%%)", h.Floor, h.HP, h.MaxHP, pct))
	}

	// Build deck summary
	deckCounts := make(map[string]int)
	for _, c := range s.FinalDeck {
		deckCounts[c]++
	}
	var deckLines []string
	for name, cnt := range deckCounts {
		if cnt > 1 {
			deckLines = append(deckLines, fmt.Sprintf("%s ×%d", name, cnt))
		} else {
			deckLines = append(deckLines, name)
		}
	}
	sort.Strings(deckLines)

	userPrompt := fmt.Sprintf(`请对以下这局杀戮尖塔 2 游戏进行深度复盘：

## 基本信息
- 角色: 铁甲战士
- 结局: %s（第 %d 层）
- 最终血量: %d/%d
- 最终金币: %d

## 最终牌组（%d 张）
%s

## 遗物
%s

## 血量变化轨迹
%s

## 选牌记录（各楼层奖励卡选择）
%s

## 商店操作
%s

## 系统生成的教训（供参考，但请给出更深入分析）
%s

请进行深度复盘，返回以下 JSON 格式：
{
  "run_id": "%s",
  "floor": %d,
  "outcome": "%s",
  "character": "IRONCLAD",
  "deck_archetype": "当前牌组方向（力量流/格挡流/清除流/混合流/无方向）",
  "root_cause": "这局失败的根本原因（1-2句话，要具体）",
  "key_decisions": [
    {
      "floor": 楼层数,
      "decision": "具体做了什么决策",
      "impact": "critical_negative/major_negative/neutral/major_positive/critical_positive",
      "lesson": "这个决策的具体教训"
    }
  ],
  "actionable_lessons": [
    "具体可操作的教训（不要写'低血量要防御'这种废话，要写'血量低于40%%时每回合必须先打防御牌再打攻击牌'）"
  ],
  "next_run_plan": "下一局的具体行动计划（3-5条规则）",
  "deck_assessment": "对牌组构筑的评价：方向是否正确？关键选牌对不对？"
}`,
		s.Outcome, s.Floor,
		s.FinalHP, s.MaxHP,
		s.Gold,
		len(s.FinalDeck), strings.Join(deckLines, "\n"),
		strings.Join(s.FinalRelics, ", "),
		strings.Join(hpLines, "\n"),
		formatCardPicks(s.CardPicks),
		formatShopActions(s.ShopActions),
		strings.Join(s.ReflectionLessons, "\n"),
		s.RunID, s.Floor, s.Outcome,
	)

	response, err := a.llm.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	jsonStr := extractJSON(response)
	var review RunReview
	if err := json.Unmarshal([]byte(jsonStr), &review); err != nil {
		return nil, fmt.Errorf("parse review JSON: %w (response: %.300s)", err, response)
	}

	review.ReviewedAt = time.Now()
	if review.RunID == "" {
		review.RunID = s.RunID
	}

	return &review, nil
}

func formatCardPicks(picks []cardPick) string {
	if len(picks) == 0 {
		return "  (无记录)"
	}
	var lines []string
	for _, p := range picks {
		if p.Chosen != "" {
			lines = append(lines, fmt.Sprintf("  Floor %d: 选了 %s", p.Floor, p.Chosen))
		}
	}
	if len(lines) == 0 {
		return "  (无记录)"
	}
	return strings.Join(lines, "\n")
}

func formatShopActions(actions []shopAction) string {
	if len(actions) == 0 {
		return "  (未进入商店或未购买)"
	}
	var lines []string
	for _, a := range actions {
		if len(a.Bought) > 0 {
			lines = append(lines, fmt.Sprintf("  Floor %d: 购买了 %s（剩余金币 %d）",
				a.Floor, strings.Join(a.Bought, ", "), a.Gold))
		} else {
			lines = append(lines, fmt.Sprintf("  Floor %d: 进入商店未购买（金币 %d）", a.Floor, a.Gold))
		}
	}
	return strings.Join(lines, "\n")
}

func printReview(r *RunReview) {
	fmt.Printf("\n=== Run Review: %s ===\n", r.RunID)
	fmt.Printf("Floor: %d | Outcome: %s | Archetype: %s\n", r.Floor, r.Outcome, r.DeckArchetype)
	fmt.Printf("Root cause: %s\n", r.RootCause)
	fmt.Printf("Deck: %s\n", r.DeckAssessment)
	fmt.Println("Key decisions:")
	for _, d := range r.KeyDecisions {
		fmt.Printf("  Floor %d [%s]: %s → %s\n", d.Floor, d.Impact, d.Decision, d.Lesson)
	}
	fmt.Println("Actionable lessons:")
	for i, l := range r.ActionableLessons {
		fmt.Printf("  %d. %s\n", i+1, l)
	}
	fmt.Printf("Next run plan: %s\n", r.NextRunPlan)
}

func loadExistingReviews(path string) map[string]RunReview {
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]RunReview)
	}
	var existing map[string]RunReview
	if err := json.Unmarshal(data, &existing); err != nil {
		return make(map[string]RunReview)
	}
	return existing
}
