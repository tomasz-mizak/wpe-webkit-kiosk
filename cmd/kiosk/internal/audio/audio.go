package audio

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var volumeRe = regexp.MustCompile(`\[(\d+)%\]`)
var muteRe = regexp.MustCompile(`\[(on|off)\]`)

// parseAmixerOutput extracts volume percentage and mute state from amixer output.
func parseAmixerOutput(output string) (level int, muted bool, err error) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if !strings.Contains(line, "Playback") {
			continue
		}
		vm := volumeRe.FindStringSubmatch(line)
		if vm == nil {
			continue
		}
		level, _ = strconv.Atoi(vm[1])
		if mm := muteRe.FindStringSubmatch(line); len(mm) == 2 {
			muted = mm[1] == "off"
		}
		return level, muted, nil
	}
	return 0, false, fmt.Errorf("no playback controls found in amixer output")
}

func amixer(args ...string) *exec.Cmd {
	return exec.Command("sudo", append([]string{"amixer"}, args...)...)
}

// GetVolume returns the current master volume level (0-100) and mute state.
func GetVolume() (level int, muted bool, err error) {
	out, err := amixer("sget", "Master").Output()
	if err != nil {
		return 0, false, fmt.Errorf("amixer failed: %w", err)
	}
	return parseAmixerOutput(string(out))
}

// SetVolume sets the master volume to the given percentage (0-100).
func SetVolume(level int) error {
	if level < 0 {
		level = 0
	}
	if level > 100 {
		level = 100
	}
	return amixer("-q", "sset", "Master", fmt.Sprintf("%d%%", level)).Run()
}

// ToggleMute toggles the master channel mute state.
func ToggleMute() error {
	return amixer("-q", "sset", "Master", "toggle").Run()
}

// Mute mutes the master channel.
func Mute() error {
	return amixer("-q", "sset", "Master", "mute").Run()
}

// Unmute unmutes the master channel.
func Unmute() error {
	return amixer("-q", "sset", "Master", "unmute").Run()
}
