package audio

import "encoding/binary"

// MakeWAV wraps raw PCM data in a valid WAV header (mono, 16-bit).
func MakeWAV(pcm []byte, sampleRate int) []byte {
	dataSize := len(pcm)
	header := make([]byte, 44)

	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], uint32(36+dataSize))
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16) // subchunk1 size
	binary.LittleEndian.PutUint16(header[20:22], 1)  // PCM format
	binary.LittleEndian.PutUint16(header[22:24], 1)  // mono
	binary.LittleEndian.PutUint32(header[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(header[28:32], uint32(sampleRate*2)) // byte rate
	binary.LittleEndian.PutUint16(header[32:34], 2)                    // block align
	binary.LittleEndian.PutUint16(header[34:36], 16)                   // bits per sample
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], uint32(dataSize))

	return append(header, pcm...)
}
