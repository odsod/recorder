package recorder

import "github.com/odsod/recorder/internal/transcript"

func (r *Recorder) appendEvent(e transcript.Event) {
	r.transcript.AppendEvent(e)
	r.log(e.String())
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func setsEqual(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}
