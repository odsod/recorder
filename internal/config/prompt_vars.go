package config

import "strings"

// OwnerPromptVars holds user-specific framing for summarization prompts.
type OwnerPromptVars struct {
	Role       string `json:"role"`
	SummaryFor string `json:"summaryFor"`
}

// PromptVarsConfig holds template variables for prompt rendering.
type PromptVarsConfig struct {
	Languages         []string        `json:"languages"`
	FillerWords       []string        `json:"fillerWords"`
	Owner             OwnerPromptVars `json:"owner"`
	IncludeInSummary  []string        `json:"includeInSummary"`
	TitleMaxWords     int             `json:"titleMaxWords"`
	SkipMaxGreetLines int             `json:"skipMaxGreetLines"`
	TitleStopWords    []string        `json:"titleStopWords"`
	SummaryLabels     []string        `json:"summaryLabels"`
}

// PromptTemplateData is the data passed to prompt text templates.
type PromptTemplateData struct {
	Owner              OwnerPromptVars
	IncludeInSummary   []string
	TitleMaxWords      int
	SkipMaxGreetLines  int
	LanguagesOr        string
	LanguagesJoin      string
	FillerWordsJoin    string
	TitleStopWordsJoin string
	SummaryLabelsJoin  string
}

func defaultPromptVars() PromptVarsConfig {
	return PromptVarsConfig{
		Languages: []string{"Swedish", "English"},
		FillerWords: []string{
			"um", "uh", "er", "like", "you know", "basically",
			"liksom", "typ", "alltså", "asså", "ba", "ju", "väl",
		},
		Owner: OwnerPromptVars{
			Role:       "software engineer",
			SummaryFor: "a human inbox",
		},
		IncludeInSummary: []string{
			"Technical decisions, action items, information shared",
			"Personal context about colleagues (birthdays, travel plans, interests, family, life events)",
			"Tool discoveries, workflow insights, opinions expressed",
			"Even casual conversation has value if it reveals something about people",
		},
		TitleMaxWords:     8,
		SkipMaxGreetLines: 3,
		TitleStopWords:    []string{"the", "a", "of", "about"},
		SummaryLabels:     []string{"Decided:", "Insight:", "Problem:", "Context:", "Next:"},
	}
}

func mergePromptVars(overrides, defaults PromptVarsConfig) PromptVarsConfig {
	merged := defaults
	if len(overrides.Languages) > 0 {
		merged.Languages = overrides.Languages
	}
	if len(overrides.FillerWords) > 0 {
		merged.FillerWords = overrides.FillerWords
	}
	if overrides.Owner.Role != "" {
		merged.Owner.Role = overrides.Owner.Role
	}
	if overrides.Owner.SummaryFor != "" {
		merged.Owner.SummaryFor = overrides.Owner.SummaryFor
	}
	if len(overrides.IncludeInSummary) > 0 {
		merged.IncludeInSummary = overrides.IncludeInSummary
	}
	if overrides.TitleMaxWords > 0 {
		merged.TitleMaxWords = overrides.TitleMaxWords
	}
	if overrides.SkipMaxGreetLines > 0 {
		merged.SkipMaxGreetLines = overrides.SkipMaxGreetLines
	}
	if len(overrides.TitleStopWords) > 0 {
		merged.TitleStopWords = overrides.TitleStopWords
	}
	if len(overrides.SummaryLabels) > 0 {
		merged.SummaryLabels = overrides.SummaryLabels
	}
	return merged
}

func promptTemplateData(vars PromptVarsConfig) PromptTemplateData {
	return PromptTemplateData{
		Owner:              vars.Owner,
		IncludeInSummary:   vars.IncludeInSummary,
		TitleMaxWords:      vars.TitleMaxWords,
		SkipMaxGreetLines:  vars.SkipMaxGreetLines,
		LanguagesOr:        joinOr(vars.Languages),
		LanguagesJoin:      strings.Join(vars.Languages, " and "),
		FillerWordsJoin:    strings.Join(vars.FillerWords, ", "),
		TitleStopWordsJoin: joinQuoted(vars.TitleStopWords),
		SummaryLabelsJoin:  formatSummaryLabels(vars.SummaryLabels),
	}
}

func joinOr(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " or " + items[len(items)-1]
	}
}

func joinQuoted(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = `"` + item + `"`
	}
	return strings.Join(quoted, ", ")
}

func formatSummaryLabels(labels []string) string {
	parts := make([]string, len(labels))
	for i, label := range labels {
		name := strings.TrimSuffix(label, ":")
		parts[i] = "**" + name + ":**"
	}
	return strings.Join(parts, ", ")
}
