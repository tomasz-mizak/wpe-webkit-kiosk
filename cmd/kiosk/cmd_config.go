package main

import (
	"fmt"

	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/config"
	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/dbus"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage kiosk configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultPath)
		if err != nil {
			return err
		}
		for _, kv := range cfg.KeyValues() {
			fmt.Printf("%s=%s\n", kv.Key, kv.Value)
		}
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]

		if !config.ValidKeys[key] {
			return fmt.Errorf("unknown config key: %s (valid: URL, INSPECTOR_PORT, INSPECTOR_HTTP_PORT)", key)
		}

		cfg, err := config.Load(config.DefaultPath)
		if err != nil {
			return err
		}

		cfg.Set(key, value)
		if err := cfg.Save(); err != nil {
			return err
		}

		fmt.Printf("Set %s=%s\n", key, value)

		if config.LiveKeys[key] {
			client, err := dbus.NewClient()
			if err != nil {
				fmt.Println("Config saved. Service not reachable — change will apply on next start.")
				return nil
			}
			switch key {
			case "URL":
				if err := client.Open(value); err != nil {
					fmt.Printf("Config saved but live apply failed: %v\n", err)
					return nil
				}
				fmt.Println("Applied live (no restart needed)")
			}
		} else {
			fmt.Println("Restart required for this change: kiosk restart")
		}

		return nil
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	rootCmd.AddCommand(configCmd)
}
