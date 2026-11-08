package server

import "testing"

func TestListenRejectsNonLoopbackAndWrongPort(t *testing.T) {
	if _, err := listen("0.0.0.0:43393"); err == nil {
		t.Fatal("expected 0.0.0.0:43393 to fail")
	}
	if _, err := listen("127.0.0.1:8080"); err == nil {
		t.Fatal("expected 127.0.0.1:8080 to fail")
	}
}
