package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/dbus"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show kiosk service status",
	RunE: func(cmd *cobra.Command, args []string) error {
		state, err := systemctlProperty("ActiveState")
		if err != nil {
			state = "unknown"
		}

		uptime := ""
		if state == "active" {
			if out, err := exec.Command("systemctl", "show", serviceName,
				"--property=ActiveEnterTimestamp", "--value").Output(); err == nil {
				uptime = strings.TrimSpace(string(out))
			}
		}

		url := ""
		client, dbusErr := dbus.NewClient()
		if dbusErr == nil {
			url, _ = client.GetUrl()
		}

		fmt.Printf("Service:  %s\n", state)
		if uptime != "" {
			fmt.Printf("Since:    %s\n", uptime)
		}
		if url != "" {
			fmt.Printf("URL:      %s\n", url)
		} else if dbusErr != nil {
			fmt.Printf("URL:      (service not reachable)\n")
		}

		return nil
	},
}

const serviceName = "wpe-webkit-kiosk"

func systemctlProperty(prop string) (string, error) {
	out, err := exec.Command("systemctl", "show", serviceName,
		"--property="+prop, "--value").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
