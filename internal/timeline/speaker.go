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
	DecayDuration        time.Duration
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

// speakerCoverageState tracks one speaker's state during coverage accumulation.
type speakerCoverageState struct {
	active        bool
	deactivatedAt time.Time
}

// Coverage returns active speakers in [start, end], ranked by coverage percentage.
func (t *SpeakerTimeline) Coverage(start, end time.Time, opts SpeakerLookupOptions) SpeakerAttribution {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !end.After(start) {
		return SpeakerAttribution{}
	}

	if opts.DecayDuration <= 0 {
		return t.coverageBinary(start, end, opts)
	}
	return t.coverageDecay(start, end, opts)
}

func (t *SpeakerTimeline) coverageBinary(start, end time.Time, opts SpeakerLookupOptions) SpeakerAttribution {
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

	return buildAttribution(coverage, window, opts)
}

func (t *SpeakerTimeline) coverageDecay(start, end time.Time, opts SpeakerLookupOptions) SpeakerAttribution {
	window := end.Sub(start)
	states := make(map[string]*speakerCoverageState)
	coverage := make(map[string]float64)
	cursor := start

	// Build initial state from events at or before the query window.
	for _, c := range t.changes {
		if !c.Time.After(start) {
			applyCoverageChange(states, c)
			continue
		}
		if c.Time.After(end) {
			break
		}
		addDecayCoverage(coverage, states, cursor, c.Time, opts.DecayDuration)
		applyCoverageChange(states, c)
		cursor = c.Time
	}
	addDecayCoverage(coverage, states, cursor, end, opts.DecayDuration)

	// Convert float64 weighted seconds to time.Duration.
	durCoverage := make(map[string]time.Duration, len(coverage))
	for name, secs := range coverage {
		durCoverage[name] = time.Duration(secs * float64(time.Second))
	}
	return buildAttribution(durCoverage, window, opts)
}

func buildAttribution(
	coverage map[string]time.Duration,
	window time.Duration,
	opts SpeakerLookupOptions,
) SpeakerAttribution {
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

func applyCoverageChange(states map[string]*speakerCoverageState, c SpeakerChange) {
	if c.Name == "" && !c.Active {
		for _, s := range states {
			if s.active {
				s.active = false
				s.deactivatedAt = c.Time
			}
		}
		return
	}
	state := states[c.Name]
	if state == nil {
		state = &speakerCoverageState{}
		states[c.Name] = state
	}
	if c.Active {
		state.active = true
		state.deactivatedAt = time.Time{}
	} else {
		state.active = false
		state.deactivatedAt = c.Time
	}
}

// addDecayCoverage accumulates weighted seconds for each speaker during [sliceStart, sliceEnd].
func addDecayCoverage(
	coverage map[string]float64,
	states map[string]*speakerCoverageState,
	sliceStart, sliceEnd time.Time,
	decayDuration time.Duration,
) {
	if !sliceEnd.After(sliceStart) {
		return
	}
	for name, state := range states {
		if state.active {
			coverage[name] += sliceEnd.Sub(sliceStart).Seconds()
		} else if !state.deactivatedAt.IsZero() {
			contrib := decayContribution(sliceStart, sliceEnd, state.deactivatedAt, decayDuration)
			if contrib > 0 {
				coverage[name] += contrib
			}
		}
	}
}

// decayContribution computes the integral of the linear decay function over [sliceStart, sliceEnd].
// The decay function is: weight(t) = max(0, 1 - (t - deactivatedAt) / decayDuration).
func decayContribution(sliceStart, sliceEnd, deactivatedAt time.Time, decayDuration time.Duration) float64 {
	decaySecs := decayDuration.Seconds()
	decayEnd := deactivatedAt.Add(decayDuration)

	// Clamp slice to the decaying interval.
	if !sliceEnd.After(deactivatedAt) {
		return 0
	}
	if !decayEnd.After(sliceStart) {
		return 0
	}
	a := sliceStart
	if a.Before(deactivatedAt) {
		a = deactivatedAt
	}
	b := sliceEnd
	if b.After(decayEnd) {
		b = decayEnd
	}

	// Weight at endpoints of [a, b].
	wa := 1.0 - a.Sub(deactivatedAt).Seconds()/decaySecs
	wb := 1.0 - b.Sub(deactivatedAt).Seconds()/decaySecs

	// Trapezoid area.
	return (wa + wb) / 2.0 * b.Sub(a).Seconds()
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
