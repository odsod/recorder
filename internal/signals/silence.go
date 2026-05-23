package signals

type SilenceMonitor struct {
	threshold int
	notified  bool
}

func NewSilenceMonitor(thresholdSecs int) *SilenceMonitor {
	return &SilenceMonitor{threshold: thresholdSecs}
}

func (m *SilenceMonitor) Tick(consecutiveSilentSecs int) bool {
	if consecutiveSilentSecs >= m.threshold && !m.notified {
		m.notified = true
		return true
	}
	return false
}

func (m *SilenceMonitor) Reset() {
	m.notified = false
}
