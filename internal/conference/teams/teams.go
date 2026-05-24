// Package teams implements the conference.Provider interface for Microsoft Teams.
// It extracts participant names and speaking state from Teams' DOM using
// [data-tid="voice-level-stream-outline"] elements.
package teams

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/odsod/recorder/internal/conference"
)

var validCSSClass = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Provider implements conference.Provider for Microsoft Teams.
type Provider struct{}

// New returns a new Microsoft Teams provider.
func New() *Provider { return &Provider{} }

// Name returns "teams".
func (p *Provider) Name() string { return "teams" }

// MatchesURL reports whether the URL is a Microsoft Teams tab.
func (p *Provider) MatchesURL(url string) bool {
	return strings.Contains(url, "teams.microsoft.com")
}

// SnapshotExpression returns JavaScript that captures all participant tiles
// with their CSS class sets for speaking-class discovery.
func (p *Provider) SnapshotExpression() string { return snapshotJS }

// ParseSnapshot parses the JSON result of SnapshotExpression.
func (p *Provider) ParseSnapshot(jsonValue string) ([]conference.ParticipantSnapshot, error) {
	var data []struct {
		Name    string   `json:"name"`
		Classes []string `json:"classes"`
	}
	if err := json.Unmarshal([]byte(jsonValue), &data); err != nil {
		return nil, err
	}
	result := make([]conference.ParticipantSnapshot, len(data))
	for i, d := range data {
		result[i] = conference.ParticipantSnapshot{Name: d.Name, Classes: d.Classes}
	}
	return result, nil
}

// PollExpression returns JavaScript that checks each participant's speaking
// state using the discovered CSS class.
func (p *Provider) PollExpression(speakingClass string) (string, error) {
	if !validCSSClass.MatchString(speakingClass) {
		return "", fmt.Errorf("invalid CSS class: %q", speakingClass)
	}
	return fmt.Sprintf(pollJSTemplate, speakingClass), nil
}

// ParsePoll parses the JSON result of PollExpression.
func (p *Provider) ParsePoll(jsonValue string) ([]conference.Participant, error) {
	var data []struct {
		Name     string `json:"name"`
		Speaking bool   `json:"speaking"`
	}
	if err := json.Unmarshal([]byte(jsonValue), &data); err != nil {
		return nil, err
	}
	result := make([]conference.Participant, len(data))
	for i, d := range data {
		result[i] = conference.Participant{Name: d.Name, Speaking: d.Speaking}
	}
	return result, nil
}

const snapshotJS = `(function() {
  return JSON.stringify(
    Array.from(document.querySelectorAll('[data-tid="voice-level-stream-outline"]')).map(function(el) {
      var p = el.parentElement;
      var tid = p ? p.getAttribute('data-tid') : null;
      var name = (tid && tid.length > 2 && tid.length < 80) ? tid : null;
      var classes = el.className.split(/\s+/);
      return {name: name, classes: classes};
    }).filter(function(x) { return x.name; })
  );
})()`

const pollJSTemplate = `(function() {
  return JSON.stringify(
    Array.from(document.querySelectorAll('[data-tid="voice-level-stream-outline"]')).map(function(el) {
      var p = el.parentElement;
      var tid = p ? p.getAttribute('data-tid') : null;
      if (!tid || tid.length <= 2) return null;
      var speaking = el.classList.contains('%s');
      return {name: tid, speaking: speaking};
    }).filter(Boolean)
  );
})()`
