package agentruntime

import (
	"strings"
	"unicode/utf8"
)

var visibleTextReplacer = strings.NewReplacer(
	"\uFFFD", "",
)

func cleanVisibleText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = visibleTextReplacer.Replace(text)
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text)
}

func cleanVisibleTextSlice(items []string) []string {
	cleaned := make([]string, 0, len(items))
	for _, item := range items {
		item = cleanVisibleText(item)
		if item == "" {
			continue
		}
		cleaned = append(cleaned, item)
	}
	return dedupeGuideLines(cleaned, 0)
}

func normalizeSeenContentDisplay(entry *SeenContentEntry, catalog *codexCatalog) {
	if entry == nil {
		return
	}

	rawName := cleanVisibleText(firstNonEmpty(strings.TrimSpace(entry.RawName), strings.TrimSpace(entry.Name)))
	nameEN := cleanVisibleText(strings.TrimSpace(entry.NameEN))
	if nameEN == "" {
		nameEN = cleanVisibleText(canonicalCatalogName(catalog, entry.Category, entry.ID))
	}

	entry.RawName = rawName
	entry.NameEN = nameEN
	if nameEN != "" {
		entry.Name = nameEN
		return
	}
	entry.Name = bestSeenContentNameValues(rawName, "", entry.ID)
}

func canonicalCatalogName(catalog *codexCatalog, category string, id string) string {
	if catalog == nil {
		return ""
	}
	return cleanVisibleText(catalog.canonicalName(category, id))
}

func bestSeenContentName(entry SeenContentEntry) string {
	return bestSeenContentNameValues(entry.Name, entry.NameEN, entry.ID)
}

func bestSeenContentNameValues(name string, nameEN string, id string) string {
	name = cleanVisibleText(strings.TrimSpace(name))
	nameEN = cleanVisibleText(strings.TrimSpace(nameEN))
	id = strings.TrimSpace(id)

	switch {
	case nameEN != "":
		return nameEN
	case name != "" && looksReadableVisibleName(name):
		return name
	case name != "":
		return name
	default:
		return id
	}
}

func looksReadableVisibleName(name string) bool {
	if name == "" || !utf8.ValidString(name) {
		return false
	}
	return !strings.Contains(name, "\uFFFD")
}
