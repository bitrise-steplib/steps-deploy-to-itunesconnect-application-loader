package main

import "testing"

func Test_xcodePath(t *testing.T) {
	tests := []struct {
		name    string
		want    []string
		wantErr bool
	}{
		{
			name:    "",
			want:    []string{"/Applications/Xcode.app", "/Applications/Xcode-beta.app"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := xcodePath()
			if (err != nil) != tt.wantErr {
				t.Errorf("xcodePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want[0] && got != tt.want[1] {
				t.Errorf("xcodePath() = %v, want %v", got, tt.want)
			}
		})
	}
}
