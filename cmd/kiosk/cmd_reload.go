package main

import (
	"fmt"

	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/dbus"

	"github.com/spf13/cobra"
)

var reloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload current page",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := dbus.NewClient()
		if err != nil {
			return err
		}
		if err := client.Reload(); err != nil {
			return err
		}
		fmt.Println("Page reloaded")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(reloadCmd)
}
