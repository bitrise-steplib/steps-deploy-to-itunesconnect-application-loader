package main

import (
	"fmt"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-xcode/ipa"
	"github.com/bitrise-io/go-xcode/plistutil"
)

// getPlatformType maps platform to an altool parameter
//
//	-t, --type {macos | ios | appletvos}     Specify the platform of the file, or of the host app when using --upload-hosted-content. (Output by 'xcrun altool -h')
//
// if 'auto' is selected the 'DTPlatformName' is read from Info.plist
func getPlatformType(ipaPath, platform string) platformType {
	fallback := func() platformType {
		log.Warnf("Failed to analyze %s, fallback platform type to ios", ipaPath)
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

func getBundleID(ipaPath string) (string, error) {
	plistPath, err := ipa.UnwrapEmbeddedInfoPlist(ipaPath)
	if err != nil {
		return "", fmt.Errorf("failed to unwrap Info.plist from the ipa: %w", err)
	}
	plist, err := plistutil.NewPlistDataFromFile(plistPath)
	if err != nil {
		return "", fmt.Errorf("failed to read Info.plist: %w", err)
	}
	bundleID, ok := plist.GetString("CFBundleIdentifier")
	if !ok {
		return "", fmt.Errorf("failed to find CFBundleIdentifier in Info.plist")
	}
	return bundleID, nil
}

func getBundleVersion(ipaPath string) (string, error) {
	plistPath, err := ipa.UnwrapEmbeddedInfoPlist(ipaPath)
	if err != nil {
		return "", fmt.Errorf("failed to unwrap Info.plist from the ipa: %w", err)
	}
	plist, err := plistutil.NewPlistDataFromFile(plistPath)
	if err != nil {
		return "", fmt.Errorf("failed to read Info.plist: %w", err)
	}
	bundleVersion, ok := plist.GetString("CFBundleVersion")
	if !ok {
		return "", fmt.Errorf("failed to find CFBundleVersion in Info.plist")
	}
	return bundleVersion, nil
}

func getBundleShortVersionString(ipaPath string) (string, error) {
	plistPath, err := ipa.UnwrapEmbeddedInfoPlist(ipaPath)
	if err != nil {
		return "", fmt.Errorf("failed to unwrap Info.plist from the ipa: %w", err)
	}
	plist, err := plistutil.NewPlistDataFromFile(plistPath)
	if err != nil {
		return "", fmt.Errorf("failed to read Info.plist: %w", err)
	}
	bundleShortVersion, ok := plist.GetString("CFBundleShortVersionString")
	if !ok {
		return "", fmt.Errorf("failed to find CFBundleShortVersionString in Info.plist")
	}
	return bundleShortVersion, nil
}
