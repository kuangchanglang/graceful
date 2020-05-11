package graceful

import (
	"testing"
)

func TestNewServer(t *testing.T) {
	s := NewServer()
	if s == nil {
		t.Errorf("NewServer return nil")
	}
}
