package main

import (
	"fmt"

	"github.com/bitrise-io/go-utils/v2/log"
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
func getPlatformType(logger log.Logger, ipaPath, platform string) (platformType, error) {
	fallback := func() platformType {
		logger.Warnf("Failed to analyze %s, fallback platform type to ios", ipaPath)
		return iOS
	}
	switch platform {
	case "auto":
		// *.pkg -> macos
		if ipaPath == "" {
			return macOS, nil
		}
		plistPath, err := ipa.UnwrapEmbeddedInfoPlist(ipaPath)
		if err != nil {
			return fallback(), fmt.Errorf("failed to unwrap Info.plist: %w", err)
		}
		plist, err := plistutil.NewPlistDataFromFile(plistPath)
		if err != nil {
			return fallback(), fmt.Errorf("failed to read Info.plist: %w", err)
		}
		platform, ok := plist.GetString("DTPlatformName")
		if !ok {
			return fallback(), fmt.Errorf("no DTPlatformName found in Info.plist")
		}
		switch platform {
		case "appletvos", "appletvsimulator":
			return tvOS, nil
		case "macosx":
			return macOS, nil
		case "iphoneos", "iphonesimulator", "watchos", "watchsimulator":
			return iOS, nil
		default:
			return fallback(), fmt.Errorf("unknown platform: %s", platform)
		}
	case "ios":
		return iOS, nil
	case "macos":
		return macOS, nil
	case "tvos":
		return tvOS, nil
	default:
		return fallback(), fmt.Errorf("inconsistent platform: %s", platform)
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
