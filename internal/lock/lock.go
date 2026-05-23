package lock

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	heartbeatInterval = 30 * time.Second
	staleAfter        = 120 * time.Second
)

type lockInfo struct {
	Hostname string `json:"hostname"`
	PID      int    `json:"pid"`
	Updated  int64  `json:"updated"`
}

type RecorderLock struct {
	path          string
	lastHeartbeat time.Time
}

func New(lockDir string) *RecorderLock {
	return &RecorderLock{
		path: filepath.Join(lockDir, ".recorder-lock"),
	}
}

func (l *RecorderLock) Acquire() error {
	existing, err := l.read()
	if err != nil {
		return err
	}
	if existing != nil && !l.isStale(existing) && !l.isSelf(existing) {
		age := time.Since(time.Unix(existing.Updated, 0))
		return fmt.Errorf("recorder already running on %s (pid %d, last heartbeat %ds ago)",
			existing.Hostname, existing.PID, int(age.Seconds()))
	}
	return l.write()
}

func (l *RecorderLock) Heartbeat() error {
	if time.Since(l.lastHeartbeat) >= heartbeatInterval {
		return l.write()
	}
	return nil
}

func (l *RecorderLock) Release() {
	_ = os.Remove(l.path)
}

func (l *RecorderLock) read() (*lockInfo, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var info lockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, nil
	}
	return &info, nil
}

func (l *RecorderLock) isStale(info *lockInfo) bool {
	return time.Since(time.Unix(info.Updated, 0)) > staleAfter
}

func (l *RecorderLock) isSelf(info *lockInfo) bool {
	hostname, _ := os.Hostname()
	return info.Hostname == hostname && info.PID == os.Getpid()
}

func (l *RecorderLock) write() error {
	hostname, _ := os.Hostname()
	info := lockInfo{
		Hostname: hostname,
		PID:      os.Getpid(),
		Updated:  time.Now().Unix(),
	}
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	tmpPath := l.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, l.path); err != nil {
		return err
	}
	l.lastHeartbeat = time.Now()
	return nil
}
