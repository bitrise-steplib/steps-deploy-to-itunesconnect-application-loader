package main

import (
	"fmt"

	v1Log "github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-xcode/ipa"
	"github.com/bitrise-io/go-xcode/plistutil"
	"github.com/bitrise-io/go-xcode/v2/metaparser"
)

type platformType string

const (
	iOS   platformType = "ios"
	tvOS  platformType = "appletvos"
	macOS platformType = "macos"
)

type packageDetails struct {
	bundleID                 string
	bundleVersion            string
	bundleShortVersionString string
}

// getPlatformType maps platform to an altool parameter
//
//	-t, --type {macos | ios | appletvos}     Specify the platform of the file, or of the host app when using --upload-hosted-content. (Output by 'xcrun altool -h')
//
// if 'auto' is selected the 'DTPlatformName' is read from Info.plist
func getPlatformType(ipaPath, platform string) platformType {
	fallback := func() platformType {
		v1Log.Warnf("Failed to analyze %s, fallback platform type to ios", ipaPath)
		return iOS
	}
	switch platform {
	case "auto":
		// *.pkg -> macos
		if ipaPath == "" {
			return macOS
		}
		plistPath, err := ipa.UnwrapEmbeddedInfoPlist(ipaPath)
		if err != nil {
			return fallback()
		}
		plist, err := plistutil.NewPlistDataFromFile(plistPath)
		if err != nil {
			return fallback()
		}
		platform, ok := plist.GetString("DTPlatformName")
		if !ok {
			return fallback()
		}
		switch platform {
		case "appletvos", "appletvsimulator":
			return tvOS
		case "macosx":
			return macOS
		case "iphoneos", "iphonesimulator", "watchos", "watchsimulator":
			return iOS
		default:
			return fallback()
		}
	case "ios":
		return iOS
	case "macos":
		return macOS
	case "tvos":
		return tvOS
	default:
		return fallback()
	}
}

func readPackageDetails(parser *metaparser.Parser, packagePath string) (packageDetails, error) {
	info, err := parser.ParseIPAData(packagePath)
	if err != nil {
		return packageDetails{}, fmt.Errorf("failed to parse archive: %w", err)
	}
	if info == nil {
		return packageDetails{}, fmt.Errorf("failed to parse archive: no metadata found")
	}

	return packageDetails{
		bundleID:                 info.AppInfo.BundleID,
		bundleVersion:            info.AppInfo.BuildNumber,
		bundleShortVersionString: info.AppInfo.Version,
	}, nil
}
