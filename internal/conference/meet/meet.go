// Package meet implements the conference.Provider interface for Google Meet.
// It extracts participant names and speaking state from Meet's DOM using
// [data-participant-id] tiles and .notranslate name elements.
package meet

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/odsod/recorder/internal/conference"
)

var validCSSClass = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Provider implements conference.Provider for Google Meet.
type Provider struct{}

// New returns a new Google Meet provider.
func New() *Provider { return &Provider{} }

// Name returns "meet".
func (p *Provider) Name() string { return "meet" }

// MatchesURL reports whether the URL is a Google Meet tab.
func (p *Provider) MatchesURL(url string) bool {
	return strings.Contains(url, "meet.google.com")
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
	return fmt.Sprintf(pollJSTemplate, speakingClass, speakingClass), nil
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
  function getName(tile) {
    var names = Array.from(tile.querySelectorAll('.notranslate'))
      .map(function(n) { return n.innerText.trim(); })
      .filter(function(s) {
        return s.length > 2 && s.length < 50
          && s[0] === s[0].toUpperCase()
          && s.includes(' ');
      });
    return names[0] || null;
  }
  return JSON.stringify(
    Array.from(document.querySelectorAll('[data-participant-id]')).map(function(t) {
      var name = getName(t);
      var classes = new Set();
      t.querySelectorAll('[class]').forEach(function(el) {
        el.classList.forEach(function(c) { classes.add(c); });
      });
      return {name: name, classes: Array.from(classes)};
    }).filter(function(x) { return x.name; })
  );
})()`

const pollJSTemplate = `(function() {
  function getName(tile) {
    var names = Array.from(tile.querySelectorAll('.notranslate'))
      .map(function(n) { return n.innerText.trim(); })
      .filter(function(s) {
        return s.length > 2 && s.length < 50
          && s[0] === s[0].toUpperCase()
          && s.includes(' ');
      });
    return names[0] || null;
  }
  return JSON.stringify(
    Array.from(document.querySelectorAll('[data-participant-id]')).map(function(t) {
      var name = getName(t);
      if (!name) return null;
      var speaking = !!t.querySelector('.%s') || !!t.closest('.%s');
      return {name: name, speaking: speaking};
    }).filter(Boolean)
  );
})()`
