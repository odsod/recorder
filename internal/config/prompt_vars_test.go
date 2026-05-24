package config

import "testing"

func TestPromptTemplateData(t *testing.T) {
	data := promptTemplateData(defaultPromptVars())

	if data.LanguagesOr != "Swedish or English" {
		t.Errorf("LanguagesOr = %q", data.LanguagesOr)
	}
	if data.LanguagesJoin != "Swedish and English" {
		t.Errorf("LanguagesJoin = %q", data.LanguagesJoin)
	}
	if data.TitleMaxWords != 8 {
		t.Errorf("TitleMaxWords = %d", data.TitleMaxWords)
	}
	if data.SummaryLabelsJoin != "**Decided:**, **Insight:**, **Problem:**, **Context:**, **Next:**" {
		t.Errorf("SummaryLabelsJoin = %q", data.SummaryLabelsJoin)
	}
}

func TestDefaultPromptVars(t *testing.T) {
	vars := defaultPromptVars()
	if len(vars.Languages) == 0 {
		t.Error("languages should have defaults")
	}
	if len(vars.FillerWords) == 0 {
		t.Error("fillerWords should have defaults")
	}
	if vars.Owner.Role == "" {
		t.Error("owner.role should have default")
	}
	if len(vars.IncludeInSummary) == 0 {
		t.Error("includeInSummary should have defaults")
	}
}
