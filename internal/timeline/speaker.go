package timeline

import (
	"slices"
	"sort"
	"sync"
	"time"
)

// SpeakerChange records a speaker transition at a point in time.
type SpeakerChange struct {
	Time time.Time
	Name string
	// Active reports whether Name became active or inactive at Time.
	// Empty Name with Active=false means all speakers became inactive.
	Active bool
}

// SpeakerTimeline is a time-indexed log of speaker start/stop events with LRU eviction.
type SpeakerTimeline struct {
	mu        sync.Mutex
	changes   []SpeakerChange
	maxAgeSec float64
}

// NewSpeakerTimeline creates a timeline that evicts entries older than maxAgeSecs.
func NewSpeakerTimeline(maxAgeSecs int) *SpeakerTimeline {
	return &SpeakerTimeline{maxAgeSec: float64(maxAgeSecs)}
}

// Append records a full active-speaker-set change at the given timestamp.
func (t *SpeakerTimeline) Append(ts time.Time, name string) {
	active := make(map[string]struct{})
	if name != "" {
		active[name] = struct{}{}
	}
	t.SetActive(ts, active)
}

// SetSpeakerActive records an independent active/inactive transition for one speaker.
func (t *SpeakerTimeline) SetSpeakerActive(ts time.Time, name string, active bool) {
	if name == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.changes = append(t.changes, SpeakerChange{Time: ts, Name: name, Active: active})
	t.evict()
}

// SetActive records the full active speaker set at the given timestamp.
func (t *SpeakerTimeline) SetActive(ts time.Time, active map[string]struct{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	current := t.activeAtLocked(ts)
	for name := range current {
		if _, ok := active[name]; !ok {
			t.changes = append(t.changes, SpeakerChange{Time: ts, Name: name, Active: false})
		}
	}

	names := make([]string, 0, len(active))
	for name := range active {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, ok := current[name]; !ok {
			t.changes = append(t.changes, SpeakerChange{Time: ts, Name: name, Active: true})
		}
	}

	t.evict()
}

// SpeakerDuration pairs a speaker name with their total speaking time.
type SpeakerDuration struct {
	Name     string
	Duration time.Duration
}

// SpeakersInWithDurations returns speakers active during [start, end], ordered
// by total speaking time (dominant speaker first), with durations included.
func (t *SpeakerTimeline) SpeakersInWithDurations(start, end time.Time) []SpeakerDuration {
	attribution := t.Coverage(start, end, SpeakerLookupOptions{})
	entries := make([]SpeakerDuration, 0, len(attribution.Candidates))
	for _, candidate := range attribution.Candidates {
		entries = append(entries, SpeakerDuration{
			Name:     candidate.Name,
			Duration: candidate.Coverage,
		})
	}
	return entries
}

// SpeakersIn returns speakers active during [start, end], ordered by total
// speaking time (dominant speaker first).
func (t *SpeakerTimeline) SpeakersIn(start, end time.Time) []string {
	entries := t.SpeakersInWithDurations(start, end)
	result := make([]string, len(entries))
	for i, e := range entries {
		result[i] = e.Name
	}
	return result
}

// SpeakerLookupOptions controls percentage-based speaker attribution.
type SpeakerLookupOptions struct {
	MinCandidatePct      float64
	MinCandidateDuration time.Duration
}

// SpeakerCandidate is one active speaker observed in a lookup window.
type SpeakerCandidate struct {
	Name        string
	Coverage    time.Duration
	CoveragePct float64
}

// SpeakerAttribution is the ranked speaker coverage for a lookup window.
type SpeakerAttribution struct {
	Candidates []SpeakerCandidate
}

// Coverage returns active speakers in [start, end], ranked by coverage percentage.
func (t *SpeakerTimeline) Coverage(start, end time.Time, opts SpeakerLookupOptions) SpeakerAttribution {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !end.After(start) {
		return SpeakerAttribution{}
	}

	window := end.Sub(start)
	active := make(map[string]struct{})
	coverage := make(map[string]time.Duration)
	cursor := start

	for _, c := range t.changes {
		if !c.Time.After(start) {
			applyChange(active, c)
			continue
		}
		if c.Time.After(end) {
			break
		}
		addCoverage(coverage, active, c.Time.Sub(cursor))
		applyChange(active, c)
		cursor = c.Time
	}
	addCoverage(coverage, active, end.Sub(cursor))

	candidates := make([]SpeakerCandidate, 0, len(coverage))
	for name, duration := range coverage {
		pct := duration.Seconds() / window.Seconds()
		if duration < opts.MinCandidateDuration {
			continue
		}
		if pct+floatEpsilon < opts.MinCandidatePct {
			continue
		}
		candidates = append(candidates, SpeakerCandidate{
			Name:        name,
			Coverage:    duration,
			CoveragePct: pct,
		})
	}
	slices.SortFunc(candidates, func(a, b SpeakerCandidate) int {
		if diff := b.Coverage - a.Coverage; diff != 0 {
			if diff > 0 {
				return 1
			}
			return -1
		}
		return stringsCompare(a.Name, b.Name)
	})
	return SpeakerAttribution{Candidates: candidates}
}

func (t *SpeakerTimeline) evict() {
	if len(t.changes) == 0 {
		return
	}
	cutoff := t.changes[len(t.changes)-1].Time
	i := 0
	for i < len(t.changes) {
		age := cutoff.Sub(t.changes[i].Time).Seconds()
		if age > t.maxAgeSec {
			i++
		} else {
			break
		}
	}
	if i > 0 {
		t.changes = t.changes[i:]
	}
}

const floatEpsilon = 1e-9

func (t *SpeakerTimeline) activeAtLocked(ts time.Time) map[string]struct{} {
	active := make(map[string]struct{})
	for _, c := range t.changes {
		if c.Time.After(ts) {
			break
		}
		applyChange(active, c)
	}
	return active
}

func applyChange(active map[string]struct{}, c SpeakerChange) {
	if c.Name == "" && !c.Active {
		clear(active)
		return
	}
	if c.Active {
		active[c.Name] = struct{}{}
	} else {
		delete(active, c.Name)
	}
}

func addCoverage(coverage map[string]time.Duration, active map[string]struct{}, duration time.Duration) {
	if duration <= 0 {
		return
	}
	for name := range active {
		coverage[name] += duration
	}
}

func stringsCompare(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
