package main

import (
	"fmt"
	"strconv"

	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/audio"

	"github.com/spf13/cobra"
)

const volumeStep = 5

var volumeCmd = &cobra.Command{
	Use:   "volume",
	Short: "Show or adjust audio volume",
	RunE: func(cmd *cobra.Command, args []string) error {
		level, muted, err := audio.GetVolume()
		if err != nil {
			return fmt.Errorf("cannot read volume: %w", err)
		}
		if muted {
			fmt.Printf("Volume: %d%% (muted)\n", level)
		} else {
			fmt.Printf("Volume: %d%%\n", level)
		}
		return nil
	},
}

var volumeSetCmd = &cobra.Command{
	Use:   "set <0-100>",
	Short: "Set volume to a specific level",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		level, err := strconv.Atoi(args[0])
		if err != nil || level < 0 || level > 100 {
			return fmt.Errorf("volume must be a number between 0 and 100")
		}
		if err := audio.SetVolume(level); err != nil {
			return fmt.Errorf("cannot set volume: %w", err)
		}
		fmt.Printf("Volume set to %d%%\n", level)
		return nil
	},
}

var volumeUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Increase volume by 5%",
	RunE: func(cmd *cobra.Command, args []string) error {
		level, _, err := audio.GetVolume()
		if err != nil {
			return fmt.Errorf("cannot read volume: %w", err)
		}
		newLevel := level + volumeStep
		if newLevel > 100 {
			newLevel = 100
		}
		if err := audio.SetVolume(newLevel); err != nil {
			return fmt.Errorf("cannot set volume: %w", err)
		}
		fmt.Printf("Volume: %d%%\n", newLevel)
		return nil
	},
}

var volumeDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Decrease volume by 5%",
	RunE: func(cmd *cobra.Command, args []string) error {
		level, _, err := audio.GetVolume()
		if err != nil {
			return fmt.Errorf("cannot read volume: %w", err)
		}
		newLevel := level - volumeStep
		if newLevel < 0 {
			newLevel = 0
		}
		if err := audio.SetVolume(newLevel); err != nil {
			return fmt.Errorf("cannot set volume: %w", err)
		}
		fmt.Printf("Volume: %d%%\n", newLevel)
		return nil
	},
}

var volumeMuteCmd = &cobra.Command{
	Use:   "mute",
	Short: "Mute audio",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := audio.Mute(); err != nil {
			return fmt.Errorf("cannot mute: %w", err)
		}
		fmt.Println("Audio muted")
		return nil
	},
}

var volumeUnmuteCmd = &cobra.Command{
	Use:   "unmute",
	Short: "Unmute audio",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := audio.Unmute(); err != nil {
			return fmt.Errorf("cannot unmute: %w", err)
		}
		fmt.Println("Audio unmuted")
		return nil
	},
}

func init() {
	volumeCmd.AddCommand(volumeSetCmd, volumeUpCmd, volumeDownCmd, volumeMuteCmd, volumeUnmuteCmd)
	rootCmd.AddCommand(volumeCmd)
}
