package main

import (
	"fmt"

	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/dbus"

	"github.com/spf13/cobra"
)

var urlCmd = &cobra.Command{
	Use:   "url",
	Short: "Print current URL",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := dbus.NewClient()
		if err != nil {
			return err
		}
		url, err := client.GetUrl()
		if err != nil {
			return err
		}
		fmt.Println(url)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(urlCmd)
}
