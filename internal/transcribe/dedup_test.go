package transcribe

import "testing"

func TestTextsOverlap_ExactMatch(t *testing.T) {
	if !TextsOverlap("hello world", "hello world", 0.6) {
		t.Error("exact match should overlap")
	}
}

func TestTextsOverlap_SubstringMatch(t *testing.T) {
	if !TextsOverlap("hello world how are you", "hello world", 0.6) {
		t.Error("substring should overlap")
	}
}

func TestTextsOverlap_HighOverlap(t *testing.T) {
	a := "we should migrate the database to postgres"
	b := "we should migrate the database to postgres immediately"
	if !TextsOverlap(a, b, 0.6) {
		t.Errorf("high overlap texts should match")
	}
}

func TestTextsOverlap_LowOverlap(t *testing.T) {
	a := "we should migrate the database to postgres"
	b := "the weather is nice today outside"
	if TextsOverlap(a, b, 0.6) {
		t.Errorf("unrelated texts should not overlap")
	}
}

func TestTextsOverlap_EmptyText(t *testing.T) {
	if TextsOverlap("", "hello", 0.6) {
		t.Error("empty text should not overlap")
	}
	if TextsOverlap("hello", "", 0.6) {
		t.Error("empty text should not overlap")
	}
}

func TestTextsOverlap_ShortTexts(t *testing.T) {
	// "hi" is a substring of "hi there" after normalization, so it matches
	if !TextsOverlap("hi", "hi there", 0.6) {
		t.Error("substring match should overlap regardless of token count")
	}
	// Two short unrelated texts with < 3 tokens should not overlap
	if TextsOverlap("hi", "bye", 0.6) {
		t.Error("short unrelated texts should not overlap")
	}
}

func TestTextsOverlap_PunctuationIgnored(t *testing.T) {
	a := "Hello, world! How are you doing?"
	b := "hello world how are you doing"
	if !TextsOverlap(a, b, 0.6) {
		t.Error("punctuation should be ignored in comparison")
	}
}

func TestTextsOverlap_CaseInsensitive(t *testing.T) {
	a := "The Database Migration Plan"
	b := "the database migration plan"
	if !TextsOverlap(a, b, 0.6) {
		t.Error("comparison should be case insensitive")
	}
}

func TestTextsOverlap_ThresholdBehavior(t *testing.T) {
	a := "one two three four five six seven eight nine ten"
	b := "one two three four five alpha beta gamma delta epsilon"
	// 5 common out of 10 shorter = 50%
	if TextsOverlap(a, b, 0.6) {
		t.Error("50% overlap should not meet 60% threshold")
	}
	if !TextsOverlap(a, b, 0.5) {
		t.Error("50% overlap should meet 50% threshold")
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello, World!", "hello world"},
		{"  multiple   spaces  ", "multiple spaces"},
		{"UPPERCASE", "uppercase"},
		{"punctuation...removed!", "punctuation removed"},
	}
	for _, tt := range tests {
		got := normalize(tt.input)
		if got != tt.want {
			t.Errorf("normalize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
