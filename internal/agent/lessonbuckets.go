package agentruntime

import "strings"

type lessonBucketSection struct {
	Title   string
	Lessons []string
}

func (b ReflectionLessonBuckets) IsEmpty() bool {
	return len(b.Pathing) == 0 &&
		len(b.RewardChoice) == 0 &&
		len(b.ShopEconomy) == 0 &&
		len(b.CombatSurvival) == 0 &&
		len(b.Runtime) == 0
}

func (b ReflectionLessonBuckets) Clone() ReflectionLessonBuckets {
	return ReflectionLessonBuckets{
		Pathing:        append([]string(nil), b.Pathing...),
		RewardChoice:   append([]string(nil), b.RewardChoice...),
		ShopEconomy:    append([]string(nil), b.ShopEconomy...),
		CombatSurvival: append([]string(nil), b.CombatSurvival...),
		Runtime:        append([]string(nil), b.Runtime...),
	}
}

func (b *ReflectionLessonBuckets) Merge(other ReflectionLessonBuckets, limit int) {
	if b == nil {
		return
	}
	if limit <= 0 {
		limit = 4
	}

	for _, lesson := range other.Pathing {
		b.Pathing = appendUniqueTrimmed(b.Pathing, lesson, limit)
	}
	for _, lesson := range other.RewardChoice {
		b.RewardChoice = appendUniqueTrimmed(b.RewardChoice, lesson, limit)
	}
	for _, lesson := range other.ShopEconomy {
		b.ShopEconomy = appendUniqueTrimmed(b.ShopEconomy, lesson, limit)
	}
	for _, lesson := range other.CombatSurvival {
		b.CombatSurvival = appendUniqueTrimmed(b.CombatSurvival, lesson, limit)
	}
	for _, lesson := range other.Runtime {
		b.Runtime = appendUniqueTrimmed(b.Runtime, lesson, limit)
	}
}

func (b ReflectionLessonBuckets) Sections() []lessonBucketSection {
	sections := make([]lessonBucketSection, 0, 5)
	appendSection := func(title string, lessons []string) {
		if len(lessons) == 0 {
			return
		}
		sections = append(sections, lessonBucketSection{
			Title:   title,
			Lessons: append([]string(nil), lessons...),
		})
	}

	appendSection("Combat survival", b.CombatSurvival)
	appendSection("Pathing", b.Pathing)
	appendSection("Reward choice", b.RewardChoice)
	appendSection("Shop economy", b.ShopEconomy)
	appendSection("Runtime", b.Runtime)
	return sections
}

func (b ReflectionLessonBuckets) ToDataMap() map[string]any {
	if b.IsEmpty() {
		return nil
	}

	return map[string]any{
		"pathing":         append([]string(nil), b.Pathing...),
		"reward_choice":   append([]string(nil), b.RewardChoice...),
		"shop_economy":    append([]string(nil), b.ShopEconomy...),
		"combat_survival": append([]string(nil), b.CombatSurvival...),
		"runtime":         append([]string(nil), b.Runtime...),
	}
}

func LessonBucketsFromData(value any) ReflectionLessonBuckets {
	switch typed := value.(type) {
	case ReflectionLessonBuckets:
		return typed.Clone()
	case map[string]any:
		return ReflectionLessonBuckets{
			Pathing:        stringsFromData(typed["pathing"]),
			RewardChoice:   stringsFromData(typed["reward_choice"]),
			ShopEconomy:    stringsFromData(typed["shop_economy"]),
			CombatSurvival: stringsFromData(typed["combat_survival"]),
			Runtime:        stringsFromData(typed["runtime"]),
		}
	default:
		return ReflectionLessonBuckets{}
	}
}

func stringsFromData(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []interface{}:
		return stringifySlice(typed)
	default:
		return nil
	}
}

func InferLessonBuckets(lessons []string) ReflectionLessonBuckets {
	buckets := ReflectionLessonBuckets{}
	for _, lesson := range lessons {
		lower := strings.ToLower(strings.TrimSpace(lesson))
		switch {
		case lower == "":
			continue
		case strings.Contains(lower, "state") ||
			strings.Contains(lower, "stale") ||
			strings.Contains(lower, "transition") ||
			strings.Contains(lower, "runtime") ||
			strings.Contains(lower, "indexed action"):
			buckets.add("runtime", lesson)
		case strings.Contains(lower, "gold") ||
			strings.Contains(lower, "shop") ||
			strings.Contains(lower, "removal") ||
			strings.Contains(lower, "relic"):
			buckets.add("shop_economy", lesson)
		case strings.Contains(lower, "path") ||
			strings.Contains(lower, "route") ||
			strings.Contains(lower, "rest") ||
			strings.Contains(lower, "safer"):
			buckets.add("pathing", lesson)
		case strings.Contains(lower, "reward") ||
			strings.Contains(lower, "card pick") ||
			strings.Contains(lower, "deck") ||
			strings.Contains(lower, "scaling") ||
			strings.Contains(lower, "burst"):
			buckets.add("reward_choice", lesson)
		case strings.Contains(lower, "hp") ||
			strings.Contains(lower, "block") ||
			strings.Contains(lower, "damage") ||
			strings.Contains(lower, "survival") ||
			strings.Contains(lower, "combat"):
			buckets.add("combat_survival", lesson)
		default:
			buckets.add("runtime", lesson)
		}
	}

	return buckets
}

func UncategorizedLessons(all []string, buckets ReflectionLessonBuckets, limit int) []string {
	if limit <= 0 {
		limit = len(all)
	}
	if buckets.IsEmpty() {
		result := make([]string, 0, len(all))
		for _, lesson := range all {
			result = appendUniqueTrimmed(result, lesson, limit)
		}
		return result
	}

	categorized := make(map[string]struct{})
	for _, lesson := range buckets.Flatten(limit + len(all)) {
		key := strings.ToLower(strings.TrimSpace(lesson))
		if key == "" {
			continue
		}
		categorized[key] = struct{}{}
	}

	result := make([]string, 0, len(all))
	for _, lesson := range all {
		key := strings.ToLower(strings.TrimSpace(lesson))
		if key == "" {
			continue
		}
		if _, ok := categorized[key]; ok {
			continue
		}
		result = appendUniqueTrimmed(result, lesson, limit)
	}
	return result
}
