package audio

import "testing"

func TestParseAmixerOutput(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantLevel int
		wantMuted bool
		wantErr   bool
	}{
		{
			name: "typical unmuted output",
			output: `Simple mixer control 'Master',0
  Capabilities: pvolume pvolume-joined pswitch pswitch-joined
  Playback channels: Mono
  Limits: Playback 0 - 87
  Mono: Playback 70 [80%] [-12.75dB] [on]`,
			wantLevel: 80,
			wantMuted: false,
		},
		{
			name: "muted output",
			output: `Simple mixer control 'Master',0
  Capabilities: pvolume pvolume-joined pswitch pswitch-joined
  Playback channels: Mono
  Limits: Playback 0 - 87
  Mono: Playback 0 [0%] [-65.25dB] [off]`,
			wantLevel: 0,
			wantMuted: true,
		},
		{
			name: "stereo output",
			output: `Simple mixer control 'Master',0
  Capabilities: pvolume pswitch
  Playback channels: Front Left - Front Right
  Limits: Playback 0 - 65536
  Front Left: Playback 52428 [80%] [on]
  Front Right: Playback 52428 [80%] [on]`,
			wantLevel: 80,
			wantMuted: false,
		},
		{
			name: "100% volume",
			output: `Simple mixer control 'Master',0
  Capabilities: pvolume pswitch
  Playback channels: Mono
  Limits: Playback 0 - 87
  Mono: Playback 87 [100%] [0.00dB] [on]`,
			wantLevel: 100,
			wantMuted: false,
		},
		{
			name: "no playback controls",
			output: `Simple mixer control 'Capture',0
  Capabilities: cvolume
  Capture channels: Mono
  Limits: Capture 0 - 63
  Mono: Capture 48 [76%] [12.00dB]`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, muted, err := parseAmixerOutput(tt.output)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if level != tt.wantLevel {
				t.Errorf("level = %d, want %d", level, tt.wantLevel)
			}
			if muted != tt.wantMuted {
				t.Errorf("muted = %v, want %v", muted, tt.wantMuted)
			}
		})
	}
}
