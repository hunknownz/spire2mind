package agentruntime

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// LLMEvolver uses LLM to propose edits for evolution.
// Inspired by autoresearch and darwin-skill: LLM directly edits the target file.
type LLMEvolver struct {
	provider CompleteEvolutioner
}

// NewLLMEvolver creates a new LLM-based evolver.
func NewLLMEvolver(provider CompleteEvolutioner) *LLMEvolver {
	return &LLMEvolver{provider: provider}
}

// ProposeGuidebookEdit generates an edit for the guidebook using LLM.
// The LLM receives the current content, analysis results, and generates a new version.
func (e *LLMEvolver) ProposeGuidebookEdit(ctx context.Context, targetFile string, analysis *GuidebookAnalysis) (EvolutionEdit, error) {
	// Read current content
	currentContent, err := os.ReadFile(targetFile)
	if err != nil {
		return EvolutionEdit{}, fmt.Errorf("read target file: %w", err)
	}

	// Build the prompt
	systemPrompt := `你是一个STS2（杀戮尖塔2）策略专家。你的任务是根据游戏数据分析结果，改进战斗策略文档。

约束规则：
1. 只添加或修改"## Evolution Rules"部分
2. 每次只添加一条新规则（bullet point格式）
3. 规则必须具体、可执行，不要模糊的建议
4. 规则用中文写
5. 不要删除已有的规则
6. 保持简洁，每条规则不超过50字

输出格式：
- 直接输出新增的规则内容（包括 "- " 前缀）
- 不要输出其他内容`

	userPrompt := fmt.Sprintf(`当前文件内容：
---
%s
---

分析结果：
- 发现的弱点：%s
- 弱点类别：%s
- 严重程度：%s

请根据这个弱点，生成一条新的进化规则来改进策略。直接输出规则内容：`,
		string(currentContent),
		analysis.Weaknesses[0].Description,
		analysis.Weaknesses[0].Category,
		analysis.Weaknesses[0].Severity,
	)

	// Call LLM
	response, err := e.provider.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return EvolutionEdit{}, fmt.Errorf("LLM call failed: %w", err)
	}

	// Parse response
	newRule := strings.TrimSpace(response)
	if !strings.HasPrefix(newRule, "- ") {
		newRule = "- " + newRule
	}

	// Build the edit
	var oldText, newText string
	if strings.Contains(string(currentContent), "## Evolution Rules") {
		// Append to existing section
		oldText = "## Evolution Rules\n"
		newText = fmt.Sprintf("## Evolution Rules\n\n%s\n", newRule)
	} else {
		// Create new section
		oldText = ""
		newText = fmt.Sprintf("\n\n## Evolution Rules\n\n%s\n", newRule)
	}

	return EvolutionEdit{
		File:     targetFile,
		OldText:  oldText,
		NewText:  newText,
		Summary:  fmt.Sprintf("[LLM/guidebook] %s", newRule),
		Category: analysis.Weaknesses[0].Category,
	}, nil
}

// ProposePlayerSkillEdit generates an edit for the player skill using LLM.
func (e *LLMEvolver) ProposePlayerSkillEdit(ctx context.Context, targetFile string, analysis *PlayerSkillAnalysis) (EvolutionEdit, error) {
	// Read current content
	currentContent, err := os.ReadFile(targetFile)
	if err != nil {
		return EvolutionEdit{}, fmt.Errorf("read target file: %w", err)
	}

	top := analysis.TopWeakness()
	if top == nil {
		return EvolutionEdit{}, fmt.Errorf("no weaknesses found")
	}

	// Build the prompt
	systemPrompt := `你是一个STS2（杀戮尖塔2）玩家身份系统。你的任务是根据游戏数据分析结果，改进玩家行为指南。

约束规则：
1. 只添加新规则，不删除已有内容
2. 规则必须具体、可执行
3. 规则用中文写
4. 保持简洁，每条规则不超过50字
5. 添加到合适的章节（## Playstyle Directives 或 ## Known Weaknesses）

输出格式：
- 直接输出新增的规则内容（包括 "- " 前缀）
- 在第一行标注要添加到哪个章节，格式：[SECTION: 章节名]`

	userPrompt := fmt.Sprintf(`当前玩家身份文件：
---
%s
---

分析结果：
- 弱点章节：%s
- 弱点描述：%s
- 严重程度：%s

请根据这个弱点，生成一条新的行为指令。第一行标注章节，然后输出规则：`,
		string(currentContent),
		top.Section,
		top.Description,
		top.Severity,
	)

	// Call LLM
	response, err := e.provider.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return EvolutionEdit{}, fmt.Errorf("LLM call failed: %w", err)
	}

	// Parse response - extract section and rule
	lines := strings.Split(strings.TrimSpace(response), "\n")
	var targetSection, newRule string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[SECTION:") {
			// Extract section name
			targetSection = strings.TrimPrefix(line, "[SECTION: ")
			targetSection = strings.TrimSuffix(targetSection, "]")
		} else if strings.HasPrefix(line, "- ") {
			newRule = line
		}
	}

	if newRule == "" {
		newRule = strings.TrimSpace(response)
		if !strings.HasPrefix(newRule, "- ") {
			newRule = "- " + newRule
		}
	}

	if targetSection == "" {
		targetSection = "Playstyle Directives"
	}

	// Find the section to append to
	sectionMarker := fmt.Sprintf("## %s", targetSection)
	var oldText, newText string
	if strings.Contains(string(currentContent), sectionMarker) {
		oldText = sectionMarker + "\n"
		newText = fmt.Sprintf("%s\n\n%s\n", sectionMarker, newRule)
	} else {
		// Create new section
		oldText = ""
		newText = fmt.Sprintf("\n\n## %s\n\n%s\n", targetSection, newRule)
	}

	return EvolutionEdit{
		File:     targetFile,
		OldText:  oldText,
		NewText:  newText,
		Summary:  fmt.Sprintf("[LLM/player-skill] %s", newRule),
		Category: top.Section,
	}, nil
}

// ProposeRewardWeightsEdit generates an edit for reward weights using LLM.
func (e *LLMEvolver) ProposeRewardWeightsEdit(ctx context.Context, targetFile string, reflections []AttemptReflection) (EvolutionEdit, error) {
	// Read current content
	currentContent, err := os.ReadFile(targetFile)
	if err != nil {
		return EvolutionEdit{}, fmt.Errorf("read target file: %w", err)
	}

	// Build analysis of reward mistakes
	var rewardLessons []string
	for _, r := range reflections {
		rewardLessons = append(rewardLessons, r.LessonBuckets.RewardChoice...)
	}

	systemPrompt := `你是一个STS2（杀戮尖塔2）卡牌评分专家。你的任务是根据游戏数据分析结果，调整奖励选择权重。

约束规则：
1. 只修改一个数值（增加或减少0.1-0.5）
2. 必须输出具体的老值和新值，格式：OLD="xxx" NEW="yyy"
3. 只能修改已存在的字段
4. 改进必须基于提供的分析数据

输出格式：
第一行：字段名
第二行：OLD="当前值" NEW="新值"
第三行：简短说明原因（不超过20字）`

	userPrompt := fmt.Sprintf(`当前权重文件内容：
---
%s
---

最近的卡牌选择教训（共%d条）：
%s

请分析这些教训，选择一个需要调整的权重，输出修改建议：`,
		string(currentContent),
		len(rewardLessons),
		strings.Join(rewardLessons[:min(10, len(rewardLessons))], "\n"),
	)

	// Call LLM
	response, err := e.provider.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return EvolutionEdit{}, fmt.Errorf("LLM call failed: %w", err)
	}

	// Parse response - extract old/new values
	lines := strings.Split(strings.TrimSpace(response), "\n")
	if len(lines) < 2 {
		return EvolutionEdit{}, fmt.Errorf("invalid LLM response format")
	}

	var oldVal, newVal string
	for _, line := range lines {
		if strings.HasPrefix(line, `OLD="`) {
			oldVal = strings.TrimPrefix(line, `OLD="`)
			oldVal = strings.TrimSuffix(oldVal, `"`)
		} else if strings.HasPrefix(line, `NEW="`) {
			newVal = strings.TrimPrefix(line, `NEW="`)
			newVal = strings.TrimSuffix(newVal, `"`)
		}
	}

	if oldVal == "" || newVal == "" {
		return EvolutionEdit{}, fmt.Errorf("failed to parse old/new values from LLM response")
	}

	return EvolutionEdit{
		File:     targetFile,
		OldText:  oldVal,
		NewText:  newVal,
		Summary:  fmt.Sprintf("[LLM/reward-weights] %s -> %s", oldVal, newVal),
		Category: "reward",
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ProposeExploratoryRewrite generates a complete rewrite of the guidebook when
// hill-climbing gets stuck. This breaks local optima by reorganizing from scratch.
// Inspired by darwin-skill's Phase 2.5.
func (e *LLMEvolver) ProposeExploratoryRewrite(ctx context.Context, targetFile string, analysis *GuidebookAnalysis) (EvolutionEdit, error) {
	// Read current content
	currentContent, err := os.ReadFile(targetFile)
	if err != nil {
		return EvolutionEdit{}, fmt.Errorf("read target file: %w", err)
	}

	systemPrompt := `你是一个STS2（杀戮尖塔2）策略专家。当前策略文档陷入了局部最优，需要进行探索性重写。

你的任务：
1. 分析现有文档的结构和规则
2. 根据发现的问题模式，重新组织策略规则
3. 删除冗余规则，合并相似规则，添加新的关键规则

约束规则：
1. 保持"## Evolution Rules"部分的核心结构
2. 规则总数不超过10条
3. 每条规则必须具体、可执行
4. 用中文写
5. 突破惯性思维，尝试新的策略角度

输出格式：
- 直接输出完整的"## Evolution Rules"部分（包括标题）
- 格式如下：
## Evolution Rules

- 规则1
- 规则2
...
- 规则N`

	// Build weakness summary
	var weaknessSummary strings.Builder
	for i, w := range analysis.Weaknesses {
		if i >= 3 {
			break
		}
		weaknessSummary.WriteString(fmt.Sprintf("- %s (%s)\n", w.Description, w.Severity))
	}

	userPrompt := fmt.Sprintf(`当前文档内容：
---
%s
---

发现的主要问题：
%s

连续多次优化失败，说明需要突破性重写。
请重新设计策略规则，突破局部最优。`,
		string(currentContent),
		weaknessSummary.String(),
	)

	// Call LLM
	response, err := e.provider.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return EvolutionEdit{}, fmt.Errorf("LLM call failed: %w", err)
	}

	// Parse response - find the Evolution Rules section
	newRules := strings.TrimSpace(response)
	if !strings.HasPrefix(newRules, "## Evolution Rules") {
		newRules = "## Evolution Rules\n\n" + newRules
	}

	// Find existing section to replace
	oldContent := string(currentContent)
	var oldText string

	if idx := strings.Index(oldContent, "## Evolution Rules"); idx >= 0 {
		// Find end of section (next ## or end of file)
		endIdx := strings.Index(oldContent[idx+20:], "\n## ")
		if endIdx < 0 {
			// No next section, replace till end
			oldText = oldContent[idx:]
		} else {
			oldText = oldContent[idx : idx+20+endIdx]
		}
	} else {
		// No existing section, append at end
		oldText = ""
	}

	return EvolutionEdit{
		File:     targetFile,
		OldText:  oldText,
		NewText:  newRules,
		Summary:  "[LLM/exploratory-rewrite] 完整重写策略规则",
		Category: "exploratory",
	}, nil
}
