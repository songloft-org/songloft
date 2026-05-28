package source

import "testing"

func TestValidate(t *testing.T) {
	defaultOpts := DefaultValidationOpts()

	tests := []struct {
		name     string
		info     AudioInfoLike
		expected float64
		opts     ValidationOpts
		wantOK   bool
		wantRsn  ValidationReason
	}{
		{
			name:   "disabled passes everything",
			info:   SimpleAudioInfo{Duration: 1, Size: 100},
			opts:   ValidationOpts{Enabled: false},
			wantOK: true,
		},
		{
			name:    "nil info -> probe_failed",
			info:    nil,
			opts:    defaultOpts,
			wantRsn: ReasonProbeFailed,
		},
		{
			name:    "zero duration -> probe_failed",
			info:    SimpleAudioInfo{Duration: 0, Size: 1024 * 1024},
			opts:    defaultOpts,
			wantRsn: ReasonProbeFailed,
		},
		{
			name:    "too short truncated file",
			info:    SimpleAudioInfo{Duration: 5, Size: 1024 * 1024},
			opts:    defaultOpts,
			wantRsn: ReasonTooShort,
		},
		{
			name:     "duration mismatch low (插件说240s,实测60s)",
			info:     SimpleAudioInfo{Duration: 60, Size: 4 * 1024 * 1024},
			expected: 240,
			opts:     defaultOpts,
			wantRsn:  ReasonDurationMismatchLow,
		},
		{
			name:     "duration mismatch high (插件说30s,实测3600s)",
			info:     SimpleAudioInfo{Duration: 3600, Size: 50 * 1024 * 1024},
			expected: 30,
			opts:     defaultOpts,
			wantRsn:  ReasonDurationMismatchHigh,
		},
		{
			name:     "bitrate too low (silence padding)",
			info:     SimpleAudioInfo{Duration: 240, Size: 10 * 1024}, // 0.3 kbps
			expected: 240,
			opts:     defaultOpts,
			wantRsn:  ReasonBitrateTooLow,
		},
		{
			name:     "valid full file",
			info:     SimpleAudioInfo{Duration: 240, Size: 4 * 1024 * 1024},
			expected: 240,
			opts:     defaultOpts,
			wantOK:   true,
		},
		{
			name:     "no expected, only absolute lower bound",
			info:     SimpleAudioInfo{Duration: 180, Size: 3 * 1024 * 1024},
			expected: 0,
			opts:     defaultOpts,
			wantOK:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := Validate(tt.info, tt.expected, tt.opts)
			if tt.wantOK {
				if !res.Valid {
					t.Errorf("expected Valid, got reason=%s actual=%v", res.Reason, res.Actual)
				}
				return
			}
			if res.Valid {
				t.Errorf("expected Invalid (%s), got Valid", tt.wantRsn)
				return
			}
			if res.Reason != tt.wantRsn {
				t.Errorf("reason mismatch: want=%s got=%s", tt.wantRsn, res.Reason)
			}
		})
	}
}
