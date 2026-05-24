package frame

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/odsod/recorder/internal/audio/pcm"
)

func TestRead(t *testing.T) {
	data := bytes.Repeat([]byte{0x01}, pcm.FrameBytes)
	got, err := Read(bytes.NewReader(data), pcm.FrameBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Error("Read data mismatch")
	}
}

func TestRead_ShortRead(t *testing.T) {
	_, err := Read(bytes.NewReader([]byte{1}), pcm.FrameBytes)
	if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("Read() err = %v, want EOF", err)
	}
}

func TestSilent(t *testing.T) {
	got := Silent(pcm.FrameBytes)
	if len(got) != pcm.FrameBytes {
		t.Fatalf("len = %d, want %d", len(got), pcm.FrameBytes)
	}
	for _, b := range got {
		if b != 0 {
			t.Error("Silent contains non-zero byte")
			break
		}
	}
}
