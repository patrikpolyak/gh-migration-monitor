package config

import (
	"testing"
	"time"
)

func TestParseSince(t *testing.T) {
	tests := []struct {
		in      string
		wantNil bool
		wantAge time.Duration
		wantAt  string
		wantErr bool
	}{
		{in: "", wantNil: true},
		{in: "36h", wantAge: 36 * time.Hour},
		{in: "7d", wantAge: 7 * 24 * time.Hour},
		{in: "2024-05-01", wantAt: "2024-05-01T00:00:00Z"},
		{in: "2024-05-01T10:30:00Z", wantAt: "2024-05-01T10:30:00Z"},
		{in: "-1h", wantErr: true},
		{in: "abc", wantErr: true},
		{in: "xd", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			fn, err := ParseSince(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if tt.wantNil {
				if fn != nil {
					t.Fatal("expected nil func")
				}
				return
			}

			got := fn()
			if tt.wantAt != "" {
				if got.UTC().Format(time.RFC3339) != tt.wantAt {
					t.Errorf("got %v, want %s", got, tt.wantAt)
				}
				return
			}
			if diff := time.Since(got) - tt.wantAge; diff < 0 || diff > time.Minute {
				t.Errorf("cutoff is %v old, want about %v", time.Since(got), tt.wantAge)
			}
		})
	}
}
