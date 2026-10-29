package collector

import (
	"os"
	"runtime"
	"testing"
	"time"
)

func TestGetProcessInfoCurrentProcess(t *testing.T) {
	pid := int32(os.Getpid())
	
	info, err := GetProcessInfo(pid)
	if err != nil {
		t.Fatalf("GetProcessInfo failed for current process: %v", err)
	}
	
	if info.ExeName == "" {
		t.Errorf("expected non-empty exe name")
	}
	
	t.Logf("Current process info: %+v", info)
}

func TestGetProcessInfoInvalidPID(t *testing.T) {
	info, err := GetProcessInfo(999999)
	if err == nil {
		t.Logf("Got info for invalid PID: %+v", info)
	} else {
		t.Logf("Expected error for invalid PID: %v", err)
	}
}

func TestProcessCache(t *testing.T) {
	pid := int32(os.Getpid())
	
	CleanExpiredCache()
	
	info1, err := GetProcessInfo(pid)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	
	info2, err := GetProcessInfo(pid)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	
	if info1.ExeName != info2.ExeName || info1.ExePath != info2.ExePath {
		t.Errorf("cache not working: %+v vs %+v", info1, info2)
	}
}

func TestInvalidateCache(t *testing.T) {
	pid := int32(os.Getpid())
	
	CleanExpiredCache()
	
	info1, _ := GetProcessInfo(pid)
	
	InvalidateProcessCache(pid)
	
	info2, _ := GetProcessInfo(pid)
	
	if info1.ExeName != info2.ExeName || info1.ExePath != info2.ExePath {
		t.Log("Cache invalidation caused re-fetch")
	}
}

func TestCleanExpiredCache(t *testing.T) {
	CleanExpiredCache()
	time.Sleep(10 * time.Millisecond)
	CleanExpiredCache()
}

func TestGetProcessInfoMultipleCalls(t *testing.T) {
	pid := int32(os.Getpid())
	
	for i := 0; i < 5; i++ {
		info, err := GetProcessInfo(pid)
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if info.ExeName == "" {
			t.Errorf("call %d: empty exe name", i)
		}
	}
	
	if runtime.GOOS == "windows" {
		t.Log("Windows: process cache test passed")
	}
}