package main

import (
	"fmt"

	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/dbus"

	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open <url>",
	Short: "Navigate kiosk to URL",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := dbus.NewClient()
		if err != nil {
			return err
		}
		if err := client.Open(args[0]); err != nil {
			return err
		}
		fmt.Printf("Navigated to %s\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(openCmd)
}
