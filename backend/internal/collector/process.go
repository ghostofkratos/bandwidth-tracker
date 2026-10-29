package collector

import (
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

type ProcessInfo struct {
	ExeName string
	ExePath string
}

var (
	processCache = make(map[int32]cachedProcessInfo)
	cacheMu      sync.RWMutex
	cacheTTL     = 30 * time.Second
)

type cachedProcessInfo struct {
	info      ProcessInfo
	expiresAt time.Time
}

func GetProcessInfo(pid int32) (ProcessInfo, error) {
	cacheMu.RLock()
	if cached, ok := processCache[pid]; ok && time.Now().Before(cached.expiresAt) {
		cacheMu.RUnlock()
		return cached.info, nil
	}
	cacheMu.RUnlock()

	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return ProcessInfo{}, err
	}

	exeName, err := proc.Name()
	if err != nil {
		exeName = "unknown"
	}

	exePath, err := proc.Exe()
	if err != nil {
		exePath = ""
	}

	info := ProcessInfo{ExeName: exeName, ExePath: exePath}

	cacheMu.Lock()
	processCache[pid] = cachedProcessInfo{
		info:      info,
		expiresAt: time.Now().Add(cacheTTL),
	}
	cacheMu.Unlock()

	return info, nil
}

func InvalidateProcessCache(pid int32) {
	cacheMu.Lock()
	delete(processCache, pid)
	cacheMu.Unlock()
}

func CleanExpiredCache() {
	cacheMu.Lock()
	now := time.Now()
	for pid, cached := range processCache {
		if now.After(cached.expiresAt) {
			delete(processCache, pid)
		}
	}
	cacheMu.Unlock()
}