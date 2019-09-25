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

func Test_checkKeyFormat(t *testing.T) {
	tests := []struct {
		name    string
		keyName string
		want    bool
	}{
		{"validKeyFormat1", "AuthKey_1234.p8", true},
		{"validKeyFormat2", "AuthKey_BITISE.p8", true},
		{"invalidKeyFormat1", "AuthKey_.p8", false},
		{"invalidKeyFormat2", "BITISE.p8", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkKeyFormat(tt.keyName); got != tt.want {
				t.Errorf("checkKeyFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getDstName(t *testing.T) {
	tests := []struct {
		name    string
		keyName string
		want    string
		wantErr bool
	}{
		{"keyNameTestValid1", "sample", "AuthKey_sample.p8", false},
		{"keyNameTestValid2", "1234", "AuthKey_1234.p8", false},
		{"keyNameTestInvalid1", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := keyNameForAltool(tt.keyName)
			if (err != nil) != tt.wantErr {
				t.Errorf("keyNameForAltool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("keyNameForAltool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getDstPath(t *testing.T) {
	tests := []struct {
		name    string
		keyPath string
		want    string
		wantErr bool
	}{
		{"keyPathTestValid1", "path/to/my/sample.p8", "private_keys/AuthKey_sample.p8", false},
		{"keyPathTestValid2", "path/to/my/1234.p8", "private_keys/AuthKey_1234.p8", false},
		{"keyPathTestValidNoExtension", "path/to/my/BITRISE", "private_keys/AuthKey_BITRISE.p8", false},
		{"keyPathTestInvalid1", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := keyPathForAltool(tt.keyPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("keyPathForAltool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("keyPathForAltool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getAPIKeyFromFileName(t *testing.T) {
	tests := []struct {
		name    string
		keyName string
		want    string
		wantErr bool
	}{
		{"apiKeyTestValid1", "path/to/my/AuthKey_sample.p8", "sample", false},
		{"apiKeyTestValid2", "path/to/my/AuthKey_1234.p8", "1234", false},
		{"apiKeyTestValid2", "path/to/my/1234.p8", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := APIKeyFromFileName(tt.keyName)
			if (err != nil) != tt.wantErr {
				t.Errorf("APIKeyFromFileName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("APIKeyFromFileName() = %v, want %v", got, tt.want)
			}
		})
	}
}
