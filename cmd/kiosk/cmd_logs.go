package main

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
)

var logsFollow bool

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show kiosk service logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		jargs := []string{"-u", serviceName, "--no-pager", "-n", "100"}
		if logsFollow {
			jargs = append(jargs, "-f")
		}

		bin, err := exec.LookPath("journalctl")
		if err != nil {
			return err
		}
		return syscall.Exec(bin, append([]string{"journalctl"}, jargs...), os.Environ())
	},
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
	rootCmd.AddCommand(logsCmd)
}
