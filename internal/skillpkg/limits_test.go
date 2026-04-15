package skillpkg

import "testing"

func TestDefaultLimits(t *testing.T) {
	l := DefaultLimits()
	if l.MaxPackedBytes != 5*1024*1024 {
		t.Errorf("MaxPackedBytes = %d, want %d", l.MaxPackedBytes, 5*1024*1024)
	}
	if l.MaxTotalBytes != 20*1024*1024 {
		t.Errorf("MaxTotalBytes = %d, want %d", l.MaxTotalBytes, 20*1024*1024)
	}
	if l.MaxFiles != 100 {
		t.Errorf("MaxFiles = %d, want 100", l.MaxFiles)
	}
	if l.MaxFileBytes != 1024*1024 {
		t.Errorf("MaxFileBytes = %d, want %d", l.MaxFileBytes, 1024*1024)
	}
}
