package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"strconv"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-tools/go-steputils/input"
	"github.com/bitrise-tools/go-steputils/stepconf"
)

// Config ...
type Config struct {
	IpaPath           string          `env:"ipa_path"`
	PkgPath           string          `env:"pkg_path"`
	ItunesConnectUser string          `env:"itunescon_user,required"`
	Password          stepconf.Secret `env:"password"`
	AppPassword       stepconf.Secret `env:"app_password"`
}

func (cfg Config) validateEnvs() error {
	if err := input.ValidateIfNotEmpty(cfg.IpaPath); err != nil {
		if err := input.ValidateIfNotEmpty(cfg.PkgPath); err != nil {
			return fmt.Errorf("neither ipa_path nor pkg_path is provided")
		}
	}

	if err := input.ValidateIfNotEmpty(string(cfg.Password)); err != nil {
		if err := input.ValidateIfNotEmpty(string(cfg.AppPassword)); err != nil {
			return fmt.Errorf("neither password nor app_password is provided")
		}
	}

	return nil
}

func main() {
	var cfg Config
	if err := stepconf.Parse(&cfg); err != nil {
		failf("Error: %s", err)
	}

	stepconf.Print(cfg)
	fmt.Println()

	if err := cfg.validateEnvs(); err != nil {
		failf("Input error: %s", err)
	}

	password := string(cfg.Password)
	if string(cfg.AppPassword) != "" {
		password = string(cfg.AppPassword)
	}

	filePth := cfg.IpaPath
	if cfg.PkgPath != "" {
		filePth = cfg.PkgPath
	}

	xcpath, err := xcodePath()
	if err != nil {
		failf("Failed to find Xcode path, error: %s", err)
	}

	log.Printf("Xcode path: %s", xcpath)
	fmt.Println()

	var altool string
	if xcodeVersion() < 11 {
		altool = filepath.Join(xcpath, "/Contents/Applications/Application Loader.app/Contents/Frameworks/ITunesSoftwareService.framework/Support/altool")
	} else {
		altool = filepath.Join(xcpath, "xcrun altool")
	}
	cmd := command.New(altool, "--upload-app", "-f", filePth, "-u", cfg.ItunesConnectUser, "-p", password)
	cmd.SetStdout(os.Stdout)
	cmd.SetStderr(os.Stderr)

	fileName := filepath.Base(filePth)
	commandStr := cmd.PrintableCommandArgs()
	commandStr = strings.Replace(commandStr, password, "[REDACTED]", -1)

	log.Infof("Uploading - %s ...", fileName)
	log.Printf("$ %s", commandStr)

	if err := cmd.Run(); err != nil {
		failf("Uploading IPA failed: %s", err)
	}

	fmt.Println()
	log.Donef("IPA uploaded")
}

func xcodePath() (string, error) {
	cmd := command.New("xcode-select", "-p")

	log.Infof("Get Xcode path")
	log.Printf(cmd.PrintableCommandArgs())

	resp, err := cmd.RunAndReturnTrimmedOutput()
	if err != nil {
		return "", err
	}

	// Default: /Applications/Xcode.app/Contents/Developer
	// Beta: /Applications/Xcode-beta.app/Contents/Developer
	split := strings.Split(resp, "/Contents")
	if len(split) != 2 {
		return "", fmt.Errorf("failed to find Xcode path")
	}

	return split[0], nil
}

func xcodeVersion() float64 {
	cmd := command.New("xcodebuild -version | sed -En 's/Xcode[[:space:]]+([0-9\\.]*)/\\1/p'")
	
	log.Infof("Get Xcode version")
	log.Printf(cmd.PrintableCommandArgs())
	
	resp, err := cmd.RunAndReturnTrimmedOutput()
	if err != nil {
		return -1.0
	}
	
	version, err := strconv.ParseFloat("123", 0, 64)
	if err != nil {
		return -2.0
	}
	return version
	
}

func failf(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}
