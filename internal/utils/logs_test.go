package utils

import "testing"

func TestStripANSISecrets(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		initialActive bool
		want          string
		wantActive    bool
	}{
		{
			name:          "plain text",
			input:         "hello world",
			initialActive: false,
			want:          "hello world",
			wantActive:    false,
		},
		{
			name:          "carriage return removed",
			input:         "line1\rline2",
			initialActive: false,
			want:          "line1line2",
			wantActive:    false,
		},
		{
			name:          "color sequence stripped",
			input:         "\x1b[31mhello\x1b[0m",
			initialActive: false,
			want:          "hello",
			wantActive:    false,
		},
		{
			name:          "conceal hides content until reset",
			input:         "\x1b[8msecret\x1b[0mvisible",
			initialActive: false,
			want:          "visible",
			wantActive:    false,
		},
		{
			name:          "conceal remains active across chunks",
			input:         "\x1b[8msecret",
			initialActive: false,
			want:          "",
			wantActive:    true,
		},
		{
			name:          "initial conceal active until reset",
			input:         "hidden\x1b[0mclear",
			initialActive: true,
			want:          "clear",
			wantActive:    false,
		},
		{
			name:          "osc sequence ignored",
			input:         "foo\x1b]0;title\x07bar",
			initialActive: false,
			want:          "foobar",
			wantActive:    false,
		},
		{
			name:          "incomplete csi stops processing",
			input:         "foo\x1b[31",
			initialActive: false,
			want:          "foo",
			wantActive:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, active := StripANSISecrets(tt.input, tt.initialActive)
			if got != tt.want {
				t.Fatalf("StripANSISecrets(%q, %t) string = %q, want %q", tt.input, tt.initialActive, got, tt.want)
			}
			if active != tt.wantActive {
				t.Fatalf("StripANSISecrets(%q, %t) active = %t, want %t", tt.input, tt.initialActive, active, tt.wantActive)
			}
		})
	}
}

