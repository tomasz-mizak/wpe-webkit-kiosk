package main

import (
	"fmt"
	"os"

	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/tui"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "kiosk",
	Short: "WPE WebKit Kiosk management tool",
	Long:  "CLI and TUI tool for managing the WPE WebKit Kiosk service.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Run()
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
