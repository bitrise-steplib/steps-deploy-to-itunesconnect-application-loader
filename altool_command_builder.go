package main

import (
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/bitrise-io/go-utils/v2/log"
)

const (
	typeKey         = "--type"
	verboseKey      = "--verbose"
	outputFormatKey = "--output-format"
)

func buildAltoolCommand(logger log.Logger, filePth string, packageDetails packageDetails, platform string, additionalParams []string, authParams []string, xcodeMajorVersion int64, appID string, isVerbose bool) []string {
	var uploadParams []string
	if xcodeMajorVersion >= 26 {
		// Use upload-package from Xcode 26. This will cause less of a breaking change,
		// as App ID, BundleID, Version and ShortVersion are optional in Xcode 26, but required in Xcode 16.
		uploadParams = []string{"--upload-package", filePth}
	} else {
		uploadParams = []string{"--upload-app", "-f", filePth}
	}

	// Platform type parameter was introduced in Xcode 13
	if !sliceutil.IsStringInSlice(typeKey, additionalParams) {
		uploadParams = append(uploadParams, typeKey, string(getPlatformType(logger, filePth, platform)))
	}

	if appID != "" {
		if xcodeMajorVersion < 26 {
			logger.Warnf("App ID is not supported with Xcode versions below 26, ignoring it.")
		} else {
			// Specifies the App Store Connect Apple ID of the app. (e.g. 1023456789)
			uploadParams = append(uploadParams, "--apple-id", appID)
			uploadParams = append(uploadParams, "--bundle-id", packageDetails.bundleID)
			// Specifies the CFBundleVersion of the app package.
			uploadParams = append(uploadParams, "--bundle-version", packageDetails.bundleVersion)
			// Specifies the CFBundleShortVersionString of the app package.
			uploadParams = append(uploadParams, "--bundle-short-version-string", packageDetails.bundleShortVersionString)
		}
	}

	// Set JSON output format so we can parse the output better
	if !sliceutil.IsStringInSlice(outputFormatKey, additionalParams) {
		additionalParams = append(additionalParams, outputFormatKey, "json")
	} else {
		logger.Warnf("Custom %s set, altool output parsing might fail!", outputFormatKey)
	}
	if isVerbose && !sliceutil.IsStringInSlice(verboseKey, additionalParams) {
		additionalParams = append(additionalParams, verboseKey)
	}

	altoolParams := append([]string{"altool"}, uploadParams...)
	altoolParams = append(altoolParams, authParams...)
	altoolParams = append(altoolParams, additionalParams...)

	return altoolParams
}
