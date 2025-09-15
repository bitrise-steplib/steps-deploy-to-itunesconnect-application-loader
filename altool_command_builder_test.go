package main

import (
	"testing"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/stretchr/testify/require"
)

func Test_buildAltoolCommand(t *testing.T) {
	logger := log.NewLogger()
	tests := []struct {
		name              string
		filePth           string
		packageDetails    packageDetails
		platform          string
		additionalParams  []string
		authParams        []string
		xcodeMajorVersion int64
		appID             string
		isVerbose         bool
		want              []string
	}{
		{
			name:    "Xcode 26, iOS, with App ID",
			filePth: "/path/to/file.ipa",
			packageDetails: packageDetails{
				bundleID:                 "com.example.app",
				bundleVersion:            "1.0.0",
				bundleShortVersionString: "1.0",
			},
			platform:          "auto",
			additionalParams:  []string{"--team-id", "TEAMID"},
			authParams:        []string{"-u", "user", "-p", "pass"},
			xcodeMajorVersion: 26,
			appID:             "1023456789",
			isVerbose:         true,
			want: []string{
				"altool",
				"--upload-package", "/path/to/file.ipa",
				"--type", "ios",
				"--apple-id", "1023456789",
				"--bundle-id", "com.example.app",
				"--bundle-version", "1.0.0",
				"--bundle-short-version-string", "1.0",
				"-u", "user", "-p", "pass",
				"--team-id", "TEAMID",
				"--verbose",
			},
		},
		{
			name:    "Xcode 26, iOS, no App ID",
			filePth: "/path/to/file.ipa",
			packageDetails: packageDetails{
				bundleID:                 "com.example.app",
				bundleVersion:            "1.0.0",
				bundleShortVersionString: "1.0",
			},
			platform:          "auto",
			additionalParams:  []string{"--team-id", "TEAMID"},
			authParams:        []string{"-u", "user", "-p", "pass"},
			xcodeMajorVersion: 26,
			appID:             "",
			isVerbose:         true,
			want: []string{
				"altool",
				"--upload-package", "/path/to/file.ipa",
				"--type", "ios",
				"-u", "user", "-p", "pass",
				"--team-id", "TEAMID",
				"--verbose",
			},
		},
		{
			name:    "Xcode 16, iOS, App ID ignored",
			filePth: "/path/to/file.ipa",
			packageDetails: packageDetails{
				bundleID:                 "com.example.app",
				bundleVersion:            "1.0.0",
				bundleShortVersionString: "1.0",
			},
			platform:          "ios",
			additionalParams:  []string{},
			authParams:        []string{"-u", "user", "-p", "pass"},
			xcodeMajorVersion: 16,
			appID:             "1023456789",
			isVerbose:         false,
			want: []string{
				"altool",
				"--upload-app", "-f", "/path/to/file.ipa",
				"--type", "ios",
				"-u", "user", "-p", "pass",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAltoolCommand(logger, tt.filePth, tt.packageDetails, tt.platform, tt.additionalParams, tt.authParams, tt.xcodeMajorVersion, tt.appID, tt.isVerbose)

			require.Equal(t, tt.want, got)
		})
	}
}
