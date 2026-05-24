package speaker_test

import (
	"context"
	"errors"
	"testing"

	"github.com/odsod/recorder/internal/conference"
	"github.com/odsod/recorder/internal/protocol/cdp"
	"github.com/odsod/recorder/internal/signals"
	"github.com/odsod/recorder/internal/speaker"
)

type mockTabLister struct {
	tabs []cdp.Tab
	err  error
}

func (m *mockTabLister) ListTabs(_ context.Context, _ cdp.ListTabsRequest) (cdp.ListTabsResponse, error) {
	if m.err != nil {
		return cdp.ListTabsResponse{}, m.err
	}
	return cdp.ListTabsResponse{Tabs: m.tabs}, nil
}

type evaluateCall struct {
	expression string
}

type mockEvaluator struct {
	value string
	err   error
	calls []evaluateCall
}

func (m *mockEvaluator) Evaluate(_ context.Context, req cdp.EvaluateRequest) (cdp.EvaluateResponse, error) {
	m.calls = append(m.calls, evaluateCall{expression: req.Expression})
	if m.err != nil {
		return cdp.EvaluateResponse{}, m.err
	}
	return cdp.EvaluateResponse{Value: m.value}, nil
}

type mockProvider struct {
	name         string
	matchesURL   bool
	snapshotJS   string
	snapshots    []conference.ParticipantSnapshot
	snapshotErr  error
	pollJS       string
	pollErr      error
	participants []conference.Participant
	parsePollErr error
}

func (m *mockProvider) Name() string               { return m.name }
func (m *mockProvider) MatchesURL(_ string) bool   { return m.matchesURL }
func (m *mockProvider) SnapshotExpression() string { return m.snapshotJS }

func (m *mockProvider) ParseSnapshot(_ string) ([]conference.ParticipantSnapshot, error) {
	return m.snapshots, m.snapshotErr
}

func (m *mockProvider) PollExpression(_ string) (string, error) {
	return m.pollJS, m.pollErr
}

func (m *mockProvider) ParsePoll(_ string) ([]conference.Participant, error) {
	return m.participants, m.parsePollErr
}

func TestPoll_NoMeeting(t *testing.T) {
	tabs := &mockTabLister{tabs: []cdp.Tab{
		{Title: "Google", URL: "https://google.com", Type: "page", WebSocketDebuggerURL: "ws://localhost:9222/page/1"},
	}}
	eval := &mockEvaluator{}
	provider := &mockProvider{name: "test", matchesURL: false}

	d := speaker.NewDetector(tabs, eval, []int{9222}, []conference.Provider{provider})
	result, err := d.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.MeetingChange != nil {
		t.Errorf("expected no meeting change, got %+v", result.MeetingChange)
	}
	if result.Participants != nil {
		t.Errorf("expected no participants, got %+v", result.Participants)
	}
}

func TestPoll_MeetingJoined(t *testing.T) {
	tabs := &mockTabLister{tabs: []cdp.Tab{
		{
			Title:                "Team Standup",
			URL:                  "https://meet.example.com/abc",
			Type:                 "page",
			WebSocketDebuggerURL: "ws://localhost:9222/page/1",
		},
	}}
	eval := &mockEvaluator{value: `[{"name":"Alice","classes":["cls-a"]}]`}
	provider := &mockProvider{
		name:       "test",
		matchesURL: true,
		snapshotJS: "snapshot()",
		snapshots: []conference.ParticipantSnapshot{
			{Name: "Alice", Classes: []string{"cls-a"}},
		},
	}

	d := speaker.NewDetector(tabs, eval, []int{9222}, []conference.Provider{provider})
	result, err := d.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.MeetingChange == nil {
		t.Fatal("expected meeting change")
	}
	if result.MeetingChange.Title != "Team Standup" {
		t.Errorf("expected title 'Team Standup', got %q", result.MeetingChange.Title)
	}
}

func TestPoll_MeetingEnded(t *testing.T) {
	tabs := &mockTabLister{tabs: []cdp.Tab{
		{
			Title:                "Meeting",
			URL:                  "https://meet.example.com/abc",
			Type:                 "page",
			WebSocketDebuggerURL: "ws://localhost:9222/page/1",
		},
	}}
	eval := &mockEvaluator{value: "[]"}
	provider := &mockProvider{
		name:       "test",
		matchesURL: true,
		snapshotJS: "snapshot()",
		snapshots:  []conference.ParticipantSnapshot{},
	}

	d := speaker.NewDetector(tabs, eval, []int{9222}, []conference.Provider{provider})

	// First poll: join meeting
	_, _ = d.Poll(context.Background())

	// Meeting ends (no matching tabs)
	provider.matchesURL = false
	result, err := d.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.MeetingChange == nil {
		t.Fatal("expected meeting change")
	}
	if result.MeetingChange.Title != "" {
		t.Errorf("expected empty title (ended), got %q", result.MeetingChange.Title)
	}
}

func TestPoll_DiscoveryToCache(t *testing.T) {
	tabs := &mockTabLister{tabs: []cdp.Tab{
		{
			Title:                "Meeting",
			URL:                  "https://meet.example.com/abc",
			Type:                 "page",
			WebSocketDebuggerURL: "ws://localhost:9222/page/1",
		},
	}}
	eval := &mockEvaluator{}
	provider := &mockProvider{
		name:       "test",
		matchesURL: true,
		snapshotJS: "snapshot()",
	}

	d := speaker.NewDetector(tabs, eval, []int{9222}, []conference.Provider{provider})

	// Poll 1: first snapshot (no diff possible yet)
	provider.snapshots = []conference.ParticipantSnapshot{
		{Name: "Alice", Classes: []string{"cls-a", "cls-b"}},
		{Name: "Bob", Classes: []string{"cls-a"}},
	}
	eval.value = "snapshot1"
	result, err := d.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// First poll: participants returned but all speaking=false
	if len(result.Participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(result.Participants))
	}
	for _, p := range result.Participants {
		if p.Speaking {
			t.Errorf("expected no one speaking on first snapshot, but %s is speaking", p.Name)
		}
	}

	// Poll 2: class changed (Alice gains "speaking-cls")
	provider.snapshots = []conference.ParticipantSnapshot{
		{Name: "Alice", Classes: []string{"cls-a", "cls-b", "x"}},
		{Name: "Bob", Classes: []string{"cls-a"}},
	}
	eval.value = "snapshot2"
	result, err = d.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Speaking class discovered: "x" (shortest changed class)
	if len(result.Participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(result.Participants))
	}
	// Alice should be speaking (has class "x")
	if !result.Participants[0].Speaking {
		t.Error("expected Alice to be speaking")
	}
	if result.Participants[1].Speaking {
		t.Error("expected Bob to not be speaking")
	}

	// Poll 3: should use cached polling now
	provider.pollJS = "poll()"
	provider.participants = []conference.Participant{
		{Name: "Alice", Speaking: false},
		{Name: "Bob", Speaking: true},
	}
	eval.value = "poll-result"
	result, err = d.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(result.Participants))
	}
	if result.Participants[0].Speaking {
		t.Error("expected Alice to not be speaking in cached poll")
	}
	if !result.Participants[1].Speaking {
		t.Error("expected Bob to be speaking in cached poll")
	}
}

func TestPoll_PollExpressionError_ResetsToDiscovery(t *testing.T) {
	tabs := &mockTabLister{tabs: []cdp.Tab{
		{
			Title:                "Meeting",
			URL:                  "https://meet.example.com/abc",
			Type:                 "page",
			WebSocketDebuggerURL: "ws://localhost:9222/page/1",
		},
	}}
	eval := &mockEvaluator{value: "result"}
	provider := &mockProvider{
		name:       "test",
		matchesURL: true,
		snapshotJS: "snapshot()",
	}

	d := speaker.NewDetector(tabs, eval, []int{9222}, []conference.Provider{provider})

	// Get into cached state (two discovery polls).
	// Use different-length class names so shortest-pick is deterministic.
	provider.snapshots = []conference.ParticipantSnapshot{{Name: "Alice", Classes: []string{"base-class"}}}
	_, _ = d.Poll(context.Background())
	provider.snapshots = []conference.ParticipantSnapshot{
		{Name: "Alice", Classes: []string{"base-class", "speaking-indicator"}},
	}
	_, _ = d.Poll(context.Background())

	// Now PollExpression returns an error (e.g., invalid class)
	provider.pollErr = errors.New("invalid CSS class")
	result, err := d.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Should return nil participants (reset, not crash)
	if result.Participants != nil {
		t.Errorf("expected nil participants after poll error, got %+v", result.Participants)
	}

	// Next poll should be in discovery mode again (prevSnapshot is stale,
	// so it will diff and find a new class — but that's fine, we verify
	// the detector doesn't crash and returns results).
	provider.pollErr = nil
	provider.snapshots = []conference.ParticipantSnapshot{{Name: "Alice", Classes: []string{"new-class"}}}
	eval.value = "snap"
	result, err = d.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Diff against stale prevSnapshot detects changed classes and picks one.
	// Alice has "new-class" — whether she's "speaking" depends on which class
	// is shortest. We just verify results are returned without error.
	if len(result.Participants) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(result.Participants))
	}
}

func TestPoll_TitleChange(t *testing.T) {
	tabs := &mockTabLister{tabs: []cdp.Tab{
		{
			Title:                "Meeting 1",
			URL:                  "https://meet.example.com/abc",
			Type:                 "page",
			WebSocketDebuggerURL: "ws://localhost:9222/page/1",
		},
	}}
	eval := &mockEvaluator{value: "[]"}
	provider := &mockProvider{
		name:       "test",
		matchesURL: true,
		snapshotJS: "snapshot()",
		snapshots:  []conference.ParticipantSnapshot{},
	}

	d := speaker.NewDetector(tabs, eval, []int{9222}, []conference.Provider{provider})

	// First poll: join
	_, _ = d.Poll(context.Background())

	// Title changes (same WebSocket URL)
	tabs.tabs[0].Title = "Meeting 2"
	result, err := d.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.MeetingChange == nil {
		t.Fatal("expected meeting change on title change")
	}
	if result.MeetingChange.Title != "Meeting 2" {
		t.Errorf("expected 'Meeting 2', got %q", result.MeetingChange.Title)
	}
}

func TestPoll_ListTabsError_Ignored(t *testing.T) {
	tabs := &mockTabLister{err: errors.New("connection refused")}
	eval := &mockEvaluator{}
	provider := &mockProvider{name: "test", matchesURL: true}

	d := speaker.NewDetector(tabs, eval, []int{9222}, []conference.Provider{provider})
	result, err := d.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.MeetingChange != nil {
		t.Error("expected no meeting change")
	}
}

func TestPoll_EvaluateError(t *testing.T) {
	tabs := &mockTabLister{tabs: []cdp.Tab{
		{
			Title:                "Meeting",
			URL:                  "https://meet.example.com/abc",
			Type:                 "page",
			WebSocketDebuggerURL: "ws://localhost:9222/page/1",
		},
	}}
	eval := &mockEvaluator{err: errors.New("websocket closed")}
	provider := &mockProvider{
		name:       "test",
		matchesURL: true,
		snapshotJS: "snapshot()",
	}

	d := speaker.NewDetector(tabs, eval, []int{9222}, []conference.Provider{provider})
	_, err := d.Poll(context.Background())
	if err == nil {
		t.Fatal("expected error from evaluate")
	}
}

func TestPoll_EmptyEvaluateResponse(t *testing.T) {
	tabs := &mockTabLister{tabs: []cdp.Tab{
		{
			Title:                "Meeting",
			URL:                  "https://meet.example.com/abc",
			Type:                 "page",
			WebSocketDebuggerURL: "ws://localhost:9222/page/1",
		},
	}}
	eval := &mockEvaluator{value: ""}
	provider := &mockProvider{
		name:       "test",
		matchesURL: true,
		snapshotJS: "snapshot()",
	}

	d := speaker.NewDetector(tabs, eval, []int{9222}, []conference.Provider{provider})
	result, err := d.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Empty evaluate response means no participants detected
	if result.Participants != nil {
		t.Errorf("expected nil participants, got %+v", result.Participants)
	}
}

// Verify the interface compliance at compile time.
var _ signals.SpeakerPoller = (*speaker.Detector)(nil)
