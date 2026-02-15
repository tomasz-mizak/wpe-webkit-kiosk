package dbus

import (
	"strings"
	"testing"

	godbus "github.com/godbus/dbus/v5"
)

func TestWrapCallErrorServiceUnknown(t *testing.T) {
	call := &godbus.Call{
		Err: godbus.Error{
			Name: "org.freedesktop.DBus.Error.ServiceUnknown",
			Body: []interface{}{"service not found"},
		},
	}
	err := wrapCallError(call, "Open")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("expected 'not running' message, got: %s", err.Error())
	}
}

func TestWrapCallErrorNil(t *testing.T) {
	call := &godbus.Call{Err: nil}
	err := wrapCallError(call, "Open")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestWrapCallErrorGeneric(t *testing.T) {
	call := &godbus.Call{
		Err: godbus.Error{
			Name: "com.wpe.Kiosk.Error.NotReady",
			Body: []interface{}{"session not initialized"},
		},
	}
	err := wrapCallError(call, "ClearData")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "D-Bus ClearData failed") {
		t.Errorf("expected method name in error, got: %s", err.Error())
	}
}
