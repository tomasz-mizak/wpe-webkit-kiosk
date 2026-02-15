package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart kiosk service",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := exec.Command("sudo", "systemctl", "restart", serviceName)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to restart service: %w", err)
		}
		fmt.Println("Service restarted")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)
}
