package main

import (
	"fmt"

	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/config"
	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/dbus"

	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open <url>",
	Short: "Navigate kiosk to URL and save to config",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]

		client, err := dbus.NewClient()
		if err != nil {
			return err
		}
		if err := client.Open(url); err != nil {
			return err
		}

		cfg, err := config.Load(config.DefaultPath)
		if err != nil {
			fmt.Printf("Navigated to %s (could not save to config: %v)\n", url, err)
			return nil
		}
		cfg.Set("URL", url)
		if err := cfg.Save(); err != nil {
			fmt.Printf("Navigated to %s (could not save to config: %v)\n", url, err)
			return nil
		}

		fmt.Printf("Navigated to %s (saved to config)\n", url)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(openCmd)
}
