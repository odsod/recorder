package transcribe

import (
	"regexp"
	"strings"
)

var nonWordRe = regexp.MustCompile(`[^\w\s]`)
var multiSpaceRe = regexp.MustCompile(`\s+`)

func normalize(text string) string {
	s := strings.ToLower(text)
	s = nonWordRe.ReplaceAllString(s, " ")
	s = multiSpaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func TextsOverlap(textA, textB string, threshold float64) bool {
	a := normalize(textA)
	b := normalize(textB)
	if a == "" || b == "" {
		return false
	}
	if a == b || strings.Contains(a, b) || strings.Contains(b, a) {
		return true
	}

	tokensA := strings.Fields(a)
	tokensB := strings.Fields(b)
	shorter := len(tokensA)
	if len(tokensB) < shorter {
		shorter = len(tokensB)
	}
	if shorter < 3 {
		return false
	}

	counts := make(map[string]int)
	for _, t := range tokensA {
		counts[t]++
	}
	common := 0
	for _, t := range tokensB {
		if counts[t] > 0 {
			counts[t]--
			common++
		}
	}

	return float64(common)/float64(shorter) >= threshold
}
