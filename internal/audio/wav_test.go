package audio

import (
	"encoding/binary"
	"testing"
)

func TestMakeWAV_HeaderStructure(t *testing.T) {
	pcm := make([]byte, 100)
	wav := MakeWAV(pcm, 16000)

	if len(wav) != 44+100 {
		t.Fatalf("wav length = %d, want %d", len(wav), 144)
	}
	if string(wav[0:4]) != "RIFF" {
		t.Error("missing RIFF marker")
	}
	if string(wav[8:12]) != "WAVE" {
		t.Error("missing WAVE marker")
	}
	if string(wav[36:40]) != "data" {
		t.Error("missing data marker")
	}

	fileSize := binary.LittleEndian.Uint32(wav[4:8])
	if fileSize != uint32(36+100) {
		t.Errorf("file size = %d, want %d", fileSize, 136)
	}

	sampleRate := binary.LittleEndian.Uint32(wav[24:28])
	if sampleRate != 16000 {
		t.Errorf("sample rate = %d, want 16000", sampleRate)
	}

	channels := binary.LittleEndian.Uint16(wav[22:24])
	if channels != 1 {
		t.Errorf("channels = %d, want 1", channels)
	}

	bitsPerSample := binary.LittleEndian.Uint16(wav[34:36])
	if bitsPerSample != 16 {
		t.Errorf("bits per sample = %d, want 16", bitsPerSample)
	}
}

func TestMakeWAV_EmptyPCM(t *testing.T) {
	wav := MakeWAV(nil, 16000)
	if len(wav) != 44 {
		t.Fatalf("empty WAV length = %d, want 44", len(wav))
	}
	dataSize := binary.LittleEndian.Uint32(wav[40:44])
	if dataSize != 0 {
		t.Errorf("data size = %d, want 0", dataSize)
	}
}
