package main

import (
	"testing"

	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/stretchr/testify/require"
)

func Test_xcodePath(t *testing.T) {
	t.Log("Xcode path test")
	{
		got, err := xcodePath()
		require.NoError(t, err)
		require.True(t, sliceutil.IsStringInSlice(got, []string{"/Applications/Xcode.app", "/Applications/Xcode-beta.app"}))
	}
}

func Test_getAuthOptions(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		want    string
		want1   string
		wantErr bool
	}{
		{"userAndPassword", Config{"dummyIPAPath", "dummyPkgPath", "iTunesUser", "password", "", "", ""}, "-u iTunesUser -p password", "password", false},
		{"userAndAppPassword1", Config{"dummyIPAPath", "dummyPkgPath", "iTunesUser", "", "appPassword", "", ""}, "-u iTunesUser -p appPassword", "appPassword", false},
		{"userAndAppPassword2", Config{"dummyIPAPath", "dummyPkgPath", "iTunesUser", "password", "appPassword", "", ""}, "-u iTunesUser -p appPassword", "appPassword", false},
		{"apiKeyAndIssuerID", Config{"dummyIPAPath", "dummyPkgPath", "", "", "", "APIKey", "IssuerID"}, "--apiKey APIKey --apiIssuer IssuerID", "", false},
		{"allProvided", Config{"dummyIPAPath", "dummyPkgPath", "iTunesUser", "password", "appPassword", "APIKey", "IssuerID"}, "-u iTunesUser -p appPassword", "appPassword", false},
		{"missingPassword", Config{"dummyIPAPath", "dummyPkgPath", "iTunesUser", "", "", "", ""}, "", "", true},
		{"missingUser", Config{"dummyIPAPath", "dummyPkgPath", "", "password", "appPassword", "", ""}, "", "", true},
		{"missingAPIKey", Config{"dummyIPAPath", "dummyPkgPath", "", "", "", "", "IssuerID"}, "", "", true},
		{"missingAPIIssuer", Config{"dummyIPAPath", "dummyPkgPath", "", "", "", "APIKey", ""}, "", "", true},
		{"allMissing", Config{"dummyIPAPath", "dummyPkgPath", "", "", "", "", ""}, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := getAuthOptions(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("getAuthOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getAuthOptions() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("getAuthOptions() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
