package main

import (
	"fmt"
	"path/filepath"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-xcode/v2/artifacts"
	"github.com/bitrise-io/go-xcode/v2/metaparser"
	"github.com/bitrise-io/go-xcode/v2/zip"
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

func (p packageDetails) hasMissingFields() bool {
	return p.bundleID == "" || p.bundleVersion == "" || p.bundleShortVersionString == ""
}

// getPlatformType maps platform to an altool parameter
//
//	-t, --type {macos | ios | appletvos}     Specify the platform of the file, or of the host app when using --upload-hosted-content. (Output by 'xcrun altool -h')
//
// if 'auto' is selected the 'DTPlatformName' is read from Info.plist
func getPlatformType(logger log.Logger, filePath, platform string) platformType {
	fallback := func(autoErr error) platformType {
		logger.Warnf("Automatic platform type lookup failed: %s", autoErr)
		logger.Warnf("Falling back to using `ios` as platform type")
		return iOS
	}
	switch platform {
	case "auto":
		// *.pkg -> macos
		if filepath.Ext(filePath) == ".pkg" {
			return macOS
		}
		platform, err := readDTPlatformName(logger, filePath)
		if err != nil {
			return fallback(err)
		}
		switch platform {
		case "appletvos", "appletvsimulator":
			return tvOS
		case "macosx":
			return macOS
		case "iphoneos", "iphonesimulator", "watchos", "watchsimulator":
			return iOS
		default:
			return fallback(fmt.Errorf("unknown platform: %s", platform))
		}
	case "ios":
		return iOS
	case "macos":
		return macOS
	case "tvos":
		return tvOS
	default:
		return fallback(fmt.Errorf("inconsistent platform: %s", platform))
	}
}

func readDTPlatformName(logger log.Logger, ipaPath string) (string, error) {
	reader, err := zip.NewDefaultReader(ipaPath, logger)
	if err != nil {
		return "", fmt.Errorf("failed to open ipa: %w", err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			logger.Warnf("%s", err)
		}
	}()

	plist, err := artifacts.NewIPAReader(reader).AppInfoPlist()
	if err != nil {
		return "", fmt.Errorf("failed to read Info.plist: %w", err)
	}
	platform, ok := plist.GetString("DTPlatformName")
	if !ok {
		return "", fmt.Errorf("no DTPlatformName found in Info.plist")
	}
	return platform, nil
}

func readPackageDetails(parser *metaparser.Parser, packagePath string, appInfo packageDetails) (packageDetails, error) {
	info, err := parser.ParseIPAData(packagePath)
	if err != nil {
		return packageDetails{}, fmt.Errorf("failed to parse archive: %w", err)
	}
	if info == nil {
		return packageDetails{}, fmt.Errorf("failed to parse archive: no metadata found")
	}

	if appInfo.bundleID == "" {
		appInfo.bundleID = info.AppInfo.BundleID
	}
	if appInfo.bundleVersion == "" {
		appInfo.bundleVersion = info.AppInfo.BuildNumber
	}
	if appInfo.bundleShortVersionString == "" {
		appInfo.bundleShortVersionString = info.AppInfo.Version
	}

	return appInfo, nil
}
