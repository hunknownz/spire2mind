package i18n

import "strings"

type Language string

const (
	LanguageEnglish   Language = "en"
	LanguageChinese   Language = "zh"
	LanguageBilingual Language = "bi"
)

type Localizer struct {
	language Language
}

func New(language Language) Localizer {
	switch language {
	case LanguageChinese, LanguageBilingual:
		return Localizer{language: language}
	default:
		return Localizer{language: LanguageEnglish}
	}
}

func ParseLanguage(value string) Language {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "zh", "zh-cn", "cn", "chinese":
		return LanguageChinese
	case "bi", "bilingual", "both", "zh-en", "en-zh":
		return LanguageBilingual
	default:
		return LanguageEnglish
	}
}

func (l Localizer) Language() Language {
	return l.language
}

func (l Localizer) Label(english string, chinese string) string {
	english = strings.TrimSpace(english)
	chinese = strings.TrimSpace(chinese)

	switch l.language {
	case LanguageChinese:
		if chinese != "" {
			return chinese
		}
		return english
	case LanguageBilingual:
		switch {
		case chinese == "":
			return english
		case english == "":
			return chinese
		default:
			return chinese + " / " + english
		}
	default:
		if english != "" {
			return english
		}
		return chinese
	}
}

func (l Localizer) Paragraph(english string, chinese string) string {
	english = strings.TrimSpace(english)
	chinese = strings.TrimSpace(chinese)

	switch l.language {
	case LanguageChinese:
		if chinese != "" {
			return chinese
		}
		return english
	case LanguageBilingual:
		switch {
		case chinese == "":
			return english
		case english == "":
			return chinese
		default:
			return chinese + "\n" + english
		}
	default:
		if english != "" {
			return english
		}
		return chinese
	}
}
