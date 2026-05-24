package signals

// SilenceMonitor emits idle signals after consecutive silent seconds cross a threshold.
type SilenceMonitor struct {
	threshold int
	notified  bool
}

// NewSilenceMonitor creates a monitor with the given silence threshold in seconds.
func NewSilenceMonitor(thresholdSecs int) *SilenceMonitor {
	return &SilenceMonitor{threshold: thresholdSecs}
}

// Tick reports whether the threshold was just crossed.
func (m *SilenceMonitor) Tick(consecutiveSilentSecs int) bool {
	if consecutiveSilentSecs >= m.threshold && !m.notified {
		m.notified = true
		return true
	}
	return false
}

// Reset clears the notified state after speech resumes.
func (m *SilenceMonitor) Reset() {
	m.notified = false
}
