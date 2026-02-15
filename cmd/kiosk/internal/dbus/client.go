package dbus

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

const (
	busName       = "com.wpe.Kiosk"
	objectPath    = "/"
	interfaceName = "com.wpe.Kiosk"
)

// Client communicates with the WPE Kiosk D-Bus interface.
type Client struct {
	conn *dbus.Conn
	obj  dbus.BusObject
}

// NewClient connects to the system bus and returns a Client.
func NewClient() (*Client, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to system bus: %w", err)
	}
	obj := conn.Object(busName, objectPath)
	return &Client{conn: conn, obj: obj}, nil
}

// Open navigates the kiosk to the given URL.
func (c *Client) Open(url string) error {
	call := c.obj.Call(interfaceName+".Open", 0, url)
	return wrapCallError(call, "Open")
}

// Reload reloads the current page.
func (c *Client) Reload() error {
	call := c.obj.Call(interfaceName+".Reload", 0)
	return wrapCallError(call, "Reload")
}

// GetUrl returns the currently loaded URL.
func (c *Client) GetUrl() (string, error) {
	var url string
	call := c.obj.Call(interfaceName+".GetUrl", 0)
	if call.Err != nil {
		return "", wrapCallError(call, "GetUrl")
	}
	if err := call.Store(&url); err != nil {
		return "", fmt.Errorf("failed to read GetUrl response: %w", err)
	}
	return url, nil
}

// ClearData clears browser data. Scope must be "cache", "cookies", or "all".
func (c *Client) ClearData(scope string) error {
	call := c.obj.Call(interfaceName+".ClearData", 0, scope)
	return wrapCallError(call, "ClearData")
}

func wrapCallError(call *dbus.Call, method string) error {
	if call.Err == nil {
		return nil
	}
	dbusErr, ok := call.Err.(dbus.Error)
	if ok && dbusErr.Name == "org.freedesktop.DBus.Error.ServiceUnknown" {
		return fmt.Errorf("kiosk service is not running")
	}
	return fmt.Errorf("D-Bus %s failed: %w", method, call.Err)
}
