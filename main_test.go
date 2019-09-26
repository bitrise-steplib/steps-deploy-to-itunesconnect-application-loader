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

func Test_altoolCommandNoProvider(t *testing.T) {
	t.Log("altoolCommand test - no provider")
	{
		got := altoolCommand("altoolPath", "ipapath.ipa", "ascUser", "ascPassword", "")
		require.True(t, got.PrintableCommandArgs() == `"altoolPath --upload-app" "-f" "ipapath.ipa" "-u" "ascUser" "-p" "ascPassword"`)
	}
}
