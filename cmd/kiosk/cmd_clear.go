package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/dbus"

	"github.com/spf13/cobra"
)

var skipConfirm bool

var clearCacheCmd = &cobra.Command{
	Use:   "clear-cache",
	Short: "Clear browser disk and memory cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		return clearWithConfirm("cache", "disk and memory cache")
	},
}

var clearCookiesCmd = &cobra.Command{
	Use:   "clear-cookies",
	Short: "Clear browser cookies",
	RunE: func(cmd *cobra.Command, args []string) error {
		return clearWithConfirm("cookies", "all cookies")
	},
}

var clearDataCmd = &cobra.Command{
	Use:   "clear-data",
	Short: "Clear all browsing data (cache, cookies, storage)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return clearWithConfirm("all", "all browsing data (cache, cookies, storage)")
	},
}

func clearWithConfirm(scope, description string) error {
	if !skipConfirm {
		fmt.Printf("Clear %s? [y/N] ", description)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	client, err := dbus.NewClient()
	if err != nil {
		return err
	}
	if err := client.ClearData(scope); err != nil {
		return err
	}
	fmt.Printf("Cleared %s\n", description)
	return nil
}

func init() {
	for _, cmd := range []*cobra.Command{clearCacheCmd, clearCookiesCmd, clearDataCmd} {
		cmd.Flags().BoolVarP(&skipConfirm, "yes", "y", false, "Skip confirmation prompt")
		rootCmd.AddCommand(cmd)
	}
}
