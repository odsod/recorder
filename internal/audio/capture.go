package audio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

func GetMonitorSource(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "pactl", "get-default-sink")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pactl get-default-sink: %w", err)
	}
	return strings.TrimSpace(string(out)) + ".monitor", nil
}

func GetMicSource(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "pactl", "get-default-source")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pactl get-default-source: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func StartParec(ctx context.Context, device string, sampleRate int) (*exec.Cmd, io.Reader, error) {
	cmd := exec.CommandContext(ctx, "parec",
		"--device="+device,
		fmt.Sprintf("--rate=%d", sampleRate),
		"--channels=1",
		"--format=s16le",
		"--raw",
	)
	cmd.Stderr = io.Discard

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start parec: %w", err)
	}

	return cmd, stdout, nil
}

func ReadFrame(r io.Reader) ([]byte, error) {
	buf := make([]byte, FrameBytes)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func SilentFrame() []byte {
	return bytes.Repeat([]byte{0}, FrameBytes)
}
