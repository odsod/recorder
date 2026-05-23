package transcript

import "testing"

func TestFormatLine_SysWithSpeakers(t *testing.T) {
	got := FormatLine("15:04:32", "🔊 sys", "Hello world", []string{"Alice"})
	want := "[15:04:32] 🔊 **sys** [Alice] Hello world"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatLine_MicNoSpeakers(t *testing.T) {
	got := FormatLine("09:00:00", "🎤 mic", "Some text", nil)
	want := "[09:00:00] 🎤 **mic** Some text"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatLine_MultipleSpeakers(t *testing.T) {
	got := FormatLine("10:30:00", "🔊 sys", "Discussion", []string{"Alice", "Bob"})
	want := "[10:30:00] 🔊 **sys** [Alice, Bob] Discussion"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatLine_EmptyText(t *testing.T) {
	got := FormatLine("12:00:00", "📍 pin", "", nil)
	want := "[12:00:00] 📍 **pin**"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatMessage_RecTag(t *testing.T) {
	got := FormatMessage("🟢 rec", "started", nil)
	want := "🟢 **rec** started"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatMessage_NoName(t *testing.T) {
	got := FormatMessage("✂️", "segment", nil)
	want := "**✂️** segment"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
