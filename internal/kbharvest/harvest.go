package kbharvest

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	stsWikiAPI    = "https://slaythespire.wiki.gg/api.php"
	stsSteamApp   = "https://store.steampowered.com/app/2868840/Slay_the_Spire_2/"
	stsSteamNews  = "https://store.steampowered.com/news/app/2868840/"
	defaultUA     = "spire2mind-kb-harvester/0.1 (+https://github.com/)"
	wikiNamespace = 3000
)

var sts1CoreTitles = []string{
	"Cards",
	"Relics",
	"Potions",
	"Events",
	"Characters",
	"Keywords",
	"Mechanics",
	"Bosses",
	"Elites",
}

type Options struct {
	OutputPath      string
	InventoryPath   string
	WikiLimit       int
	IncludeSTS1Core bool
}

type Result struct {
	OutputPath    string
	InventoryPath string
	Documents     []Document
	Inventory     Inventory
}

type Document struct {
	ID          string    `json:"id"`
	Game        string    `json:"game"`
	Language    string    `json:"language"`
	Source      string    `json:"source"`
	Kind        string    `json:"kind"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Tags        []string  `json:"tags,omitempty"`
	RetrievedAt time.Time `json:"retrieved_at"`
	Text        string    `json:"text"`
}

type Inventory struct {
	GeneratedAt     time.Time        `json:"generated_at"`
	DocumentCount   int              `json:"document_count"`
	SourceBreakdown []InventoryEntry `json:"source_breakdown"`
	Findings        Findings         `json:"findings"`
}

type InventoryEntry struct {
	Name      string `json:"name"`
	Language  string `json:"language"`
	Kind      string `json:"kind"`
	Documents int    `json:"documents"`
	Notes     string `json:"notes,omitempty"`
}

type Findings struct {
	PublicWebMatchData    string   `json:"public_web_match_data"`
	LocalMatchData        []string `json:"local_match_data"`
	BestStructuredSources []string `json:"best_structured_sources"`
	ChineseSourceNotes    []string `json:"chinese_source_notes"`
}

type wikiAllPagesResponse struct {
	Continue struct {
		APContinue string `json:"apcontinue"`
	} `json:"continue"`
	Query struct {
		AllPages []struct {
			Title string `json:"title"`
		} `json:"allpages"`
	} `json:"query"`
}

type wikiContentResponse struct {
	Query struct {
		Pages []struct {
			Title     string `json:"title"`
			Missing   bool   `json:"missing,omitempty"`
			Revisions []struct {
				Content string `json:"content"`
			} `json:"revisions"`
		} `json:"pages"`
	} `json:"query"`
}

func Harvest(ctx context.Context, opts Options) (*Result, error) {
	if opts.WikiLimit < 0 {
		opts.WikiLimit = 0
	}
	if strings.TrimSpace(opts.OutputPath) == "" {
		opts.OutputPath = filepath.Join("data", "kb", "external-docs.json")
	}
	if strings.TrimSpace(opts.InventoryPath) == "" {
		opts.InventoryPath = filepath.Join("data", "kb", "source-inventory.json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	now := time.Now().UTC()
	documents := make([]Document, 0, max(8, opts.WikiLimit+8))

	steamDocs, err := harvestSteamDocs(ctx, client, now)
	if err != nil {
		return nil, err
	}
	documents = append(documents, steamDocs...)

	if opts.WikiLimit > 0 {
		wikiDocs, err := harvestSTS2WikiDocs(ctx, client, opts.WikiLimit, now)
		if err != nil {
			return nil, err
		}
		documents = append(documents, wikiDocs...)
	}

	if opts.IncludeSTS1Core {
		sts1Docs, err := harvestSTS1CoreDocs(ctx, client, now)
		if err != nil {
			return nil, err
		}
		documents = append(documents, sts1Docs...)
	}

	documents = dedupeDocs(documents)
	sort.Slice(documents, func(i, j int) bool {
		if documents[i].Source != documents[j].Source {
			return documents[i].Source < documents[j].Source
		}
		return documents[i].Title < documents[j].Title
	})

	inventory := buildInventory(now, documents)
	if err := writeJSON(opts.OutputPath, documents); err != nil {
		return nil, err
	}
	if err := writeJSON(opts.InventoryPath, inventory); err != nil {
		return nil, err
	}

	return &Result{
		OutputPath:    opts.OutputPath,
		InventoryPath: opts.InventoryPath,
		Documents:     documents,
		Inventory:     inventory,
	}, nil
}

func harvestSteamDocs(ctx context.Context, client *http.Client, now time.Time) ([]Document, error) {
	pages := []struct {
		idSuffix string
		url      string
		language string
		kind     string
		tags     []string
	}{
		{
			idSuffix: "store-en",
			url:      stsSteamApp + "?l=english",
			language: "en",
			kind:     "official-store",
			tags:     []string{"official", "store", "sts2"},
		},
		{
			idSuffix: "store-zh-hans",
			url:      stsSteamApp + "?l=schinese",
			language: "zh-Hans",
			kind:     "official-store",
			tags:     []string{"official", "store", "sts2"},
		},
		{
			idSuffix: "news-en",
			url:      stsSteamNews + "?l=english",
			language: "en",
			kind:     "official-news",
			tags:     []string{"official", "news", "sts2"},
		},
		{
			idSuffix: "news-zh-hans",
			url:      stsSteamNews + "?l=schinese",
			language: "zh-Hans",
			kind:     "official-news",
			tags:     []string{"official", "news", "sts2"},
		},
	}

	docs := make([]Document, 0, len(pages))
	for _, page := range pages {
		body, err := fetchText(ctx, client, page.url)
		if err != nil {
			return nil, fmt.Errorf("fetch steam page %s: %w", page.url, err)
		}
		title := extractHTMLTitle(body)
		text := cleanHTML(body)
		if title == "" {
			title = page.url
		}
		docs = append(docs, Document{
			ID:          "steam-" + page.idSuffix,
			Game:        "sts2",
			Language:    page.language,
			Source:      "steam",
			Kind:        page.kind,
			Title:       title,
			URL:         page.url,
			Tags:        append([]string(nil), page.tags...),
			RetrievedAt: now,
			Text:        text,
		})
	}
	return docs, nil
}

func harvestSTS2WikiDocs(ctx context.Context, client *http.Client, limit int, now time.Time) ([]Document, error) {
	titles, err := listWikiTitles(ctx, client, wikiNamespace, limit)
	if err != nil {
		return nil, fmt.Errorf("list sts2 wiki titles: %w", err)
	}
	docs := make([]Document, 0, len(titles))
	for _, title := range titles {
		if strings.Contains(title, "/") {
			continue
		}
		content, err := fetchWikiPageContent(ctx, client, title)
		if err != nil {
			return nil, fmt.Errorf("fetch wiki page %q: %w", title, err)
		}
		text := cleanWikiText(content)
		if text == "" {
			continue
		}
		docs = append(docs, Document{
			ID:          "wiki-sts2-" + slugify(title),
			Game:        "sts2",
			Language:    "en",
			Source:      "wiki.gg",
			Kind:        "wiki-page",
			Title:       title,
			URL:         wikiPageURL(title),
			Tags:        []string{"community", "wiki", "sts2"},
			RetrievedAt: now,
			Text:        text,
		})
	}
	return docs, nil
}

func harvestSTS1CoreDocs(ctx context.Context, client *http.Client, now time.Time) ([]Document, error) {
	docs := make([]Document, 0, len(sts1CoreTitles))
	for _, title := range sts1CoreTitles {
		content, err := fetchWikiPageContent(ctx, client, title)
		if err != nil {
			return nil, fmt.Errorf("fetch sts1 core page %q: %w", title, err)
		}
		text := cleanWikiText(content)
		if text == "" {
			continue
		}
		docs = append(docs, Document{
			ID:          "wiki-sts1-" + slugify(title),
			Game:        "sts1",
			Language:    "en",
			Source:      "wiki.gg",
			Kind:        "wiki-page",
			Title:       title,
			URL:         wikiPageURL(title),
			Tags:        []string{"community", "wiki", "sts1", "core-overview"},
			RetrievedAt: now,
			Text:        text,
		})
	}
	return docs, nil
}

func listWikiTitles(ctx context.Context, client *http.Client, namespace int, limit int) ([]string, error) {
	if limit <= 0 {
		return nil, nil
	}
	titles := make([]string, 0, limit)
	apcontinue := ""
	for len(titles) < limit {
		values := url.Values{}
		values.Set("action", "query")
		values.Set("list", "allpages")
		values.Set("apnamespace", fmt.Sprintf("%d", namespace))
		values.Set("aplimit", fmt.Sprintf("%d", min(50, limit-len(titles))))
		values.Set("format", "json")
		if apcontinue != "" {
			values.Set("apcontinue", apcontinue)
		}
		body, err := fetchText(ctx, client, stsWikiAPI+"?"+values.Encode())
		if err != nil {
			return nil, err
		}
		var response wikiAllPagesResponse
		if err := json.Unmarshal([]byte(body), &response); err != nil {
			return nil, err
		}
		if len(response.Query.AllPages) == 0 {
			break
		}
		for _, page := range response.Query.AllPages {
			if strings.TrimSpace(page.Title) == "" {
				continue
			}
			titles = append(titles, page.Title)
			if len(titles) >= limit {
				break
			}
		}
		if response.Continue.APContinue == "" {
			break
		}
		apcontinue = response.Continue.APContinue
	}
	return titles, nil
}

func fetchWikiPageContent(ctx context.Context, client *http.Client, title string) (string, error) {
	values := url.Values{}
	values.Set("action", "query")
	values.Set("titles", title)
	values.Set("prop", "revisions")
	values.Set("rvprop", "content")
	values.Set("format", "json")
	values.Set("formatversion", "2")
	body, err := fetchText(ctx, client, stsWikiAPI+"?"+values.Encode())
	if err != nil {
		return "", err
	}
	var response wikiContentResponse
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		return "", err
	}
	for _, page := range response.Query.Pages {
		if page.Missing || len(page.Revisions) == 0 {
			continue
		}
		return page.Revisions[0].Content, nil
	}
	return "", nil
}

func fetchText(ctx context.Context, client *http.Client, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", defaultUA)
	req.Header.Set("Accept-Language", "en-US,en;q=0.8,zh-CN;q=0.6")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func buildInventory(now time.Time, docs []Document) Inventory {
	type key struct {
		source   string
		language string
		kind     string
	}
	counts := make(map[key]int)
	for _, doc := range docs {
		counts[key{source: doc.Source, language: doc.Language, kind: doc.Kind}]++
	}
	entries := make([]InventoryEntry, 0, len(counts))
	for k, count := range counts {
		entry := InventoryEntry{
			Name:      k.source,
			Language:  k.language,
			Kind:      k.kind,
			Documents: count,
		}
		switch {
		case k.source == "wiki.gg":
			entry.Notes = "Community wiki with stable MediaWiki API; best English structured source for STS2 and broad STS1 coverage."
		case k.source == "steam" && strings.Contains(k.kind, "store"):
			entry.Notes = "Official bilingual store page; good for Chinese official wording and release positioning."
		case k.source == "steam" && strings.Contains(k.kind, "news"):
			entry.Notes = "Official update log surface; useful for balance changes and version drift."
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Name != entries[j].Name {
			return entries[i].Name < entries[j].Name
		}
		if entries[i].Language != entries[j].Language {
			return entries[i].Language < entries[j].Language
		}
		return entries[i].Kind < entries[j].Kind
	})

	return Inventory{
		GeneratedAt:     now,
		DocumentCount:   len(docs),
		SourceBreakdown: entries,
		Findings: Findings{
			PublicWebMatchData: "No stable public web API for full run-by-run Slay the Spire 2 match telemetry was confirmed. Public web data is strong for wiki knowledge and official patch notes, but not for authoritative live match logs.",
			LocalMatchData: []string{
				"This repo's Bridge exposes live match state over GET /state and state transitions over GET /events/stream.",
				"This repo already persists per-run artifacts under scratch/agent-runs and aggregates them into scratch/guidebook/living-codex.json and run-index.sqlite.",
				"Game-local save/settings files are useful for environment and mod state, but match-history extraction should be treated as a separate reverse-engineering task.",
			},
			BestStructuredSources: []string{
				"wiki.gg MediaWiki API for English STS2 and STS1 pages",
				"Steam official store/news pages for official English and Simplified Chinese text",
			},
			ChineseSourceNotes: []string{
				"Reliable structured Chinese STS2 community sources are much thinner than English sources.",
				"The safest Chinese baseline today is the official Simplified Chinese Steam page and official Steam news surface, then align English wiki material into your own schema.",
			},
		},
	}
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	bytes = append(bytes, '\n')
	return os.WriteFile(path, bytes, 0o644)
}

func dedupeDocs(docs []Document) []Document {
	seen := make(map[string]Document, len(docs))
	for _, doc := range docs {
		if doc.ID == "" {
			continue
		}
		seen[doc.ID] = doc
	}
	result := make([]Document, 0, len(seen))
	for _, doc := range seen {
		result = append(result, doc)
	}
	return result
}

func wikiPageURL(title string) string {
	return "https://slaythespire.wiki.gg/wiki/" + url.PathEscape(strings.ReplaceAll(title, " ", "_"))
}

func extractHTMLTitle(body string) string {
	match := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`).FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	return collapseWhitespace(html.UnescapeString(match[1]))
}

func cleanHTML(body string) string {
	text := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`).ReplaceAllString(body, " ")
	text = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	return collapseWhitespace(text)
}

func cleanWikiText(body string) string {
	text := body
	text = regexp.MustCompile(`(?s)<!--.*?-->`).ReplaceAllString(text, " ")
	text = stripNestedDelimited(text, "{{", "}}")
	text = stripNestedDelimited(text, "{|", "|}")
	text = regexp.MustCompile(`(?m)^==+\s*(.*?)\s*==+$`).ReplaceAllString(text, "\n$1\n")
	text = regexp.MustCompile(`\[\[([^|\]]+)\|([^\]]+)\]\]`).ReplaceAllString(text, "$2")
	text = regexp.MustCompile(`\[\[([^\]]+)\]\]`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`\[(https?://[^\s\]]+)\s+([^\]]+)\]`).ReplaceAllString(text, "$2")
	text = regexp.MustCompile(`(?m)^\*+`).ReplaceAllString(text, "-")
	text = strings.NewReplacer("'''", "", "''", "", "__TOC__", " ").Replace(text)
	text = regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	return collapseWhitespace(text)
}

func stripNestedDelimited(input string, open string, close string) string {
	if input == "" {
		return ""
	}
	var out strings.Builder
	depth := 0
	for i := 0; i < len(input); {
		switch {
		case strings.HasPrefix(input[i:], open):
			depth++
			i += len(open)
		case strings.HasPrefix(input[i:], close) && depth > 0:
			depth--
			i += len(close)
		case depth == 0:
			out.WriteByte(input[i])
			i++
		default:
			i++
		}
	}
	return out.String()
}

func collapseWhitespace(input string) string {
	input = strings.ReplaceAll(input, "\r", "\n")
	input = regexp.MustCompile(`\n{3,}`).ReplaceAllString(input, "\n\n")
	fields := strings.Fields(input)
	return strings.TrimSpace(strings.Join(fields, " "))
}

func slugify(input string) string {
	replacer := strings.NewReplacer(":", "-", "/", "-", " ", "-", "(", "", ")", "", "'", "", "\"", "")
	slug := replacer.Replace(strings.ToLower(strings.TrimSpace(input)))
	slug = regexp.MustCompile(`[^a-z0-9_-]+`).ReplaceAllString(slug, "-")
	slug = regexp.MustCompile(`-+`).ReplaceAllString(slug, "-")
	return strings.Trim(slug, "-")
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
