package frame

import "io"

// Dual holds one system and one microphone audio frame.
type Dual struct {
	Sys []byte
	Mic []byte
}

// Read reads exactly one audio frame from the reader.
func Read(r io.Reader, frameBytes int) ([]byte, error) {
	buf := make([]byte, frameBytes)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// Silent returns a zero-filled frame representing silence.
func Silent(frameBytes int) []byte {
	return make([]byte, frameBytes)
}
