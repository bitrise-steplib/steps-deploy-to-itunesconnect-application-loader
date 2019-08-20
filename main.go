package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	ASCProvider       string          `env:"asc_provider"`
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

	altool := filepath.Join(xcpath, "/Contents/Applications/Application Loader.app/Contents/Frameworks/ITunesSoftwareService.framework/Support/altool")
	cmd := altoolCommand(altool, filePth, cfg.ItunesConnectUser, password, cfg.ASCProvider)
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

/*
	Returns a command.Model object, that when executed will run altool & upload the given ipa file.

	Note: Provide ascProvider as "" to not apply --asc_provider.

	Parameters:
	- altoolPath: The path to the altool executable.
	- ipaPath: The path to the .ipa file that'll be uploaded.
	- ascUser: The user's App Store Connect username.
	- ascPassword: The user's App Store Connect password.
	- ascProvider: The team ID that the .ipa should be uploaded against. Used to specify a team where multiple are available.
 */
func altoolCommand(altoolPath string, ipaPath string, ascUser string, ascPassword string, ascProvider string) *command.Model {
	if ascProvider == "" {
		return command.New(altoolPath, "--upload-app", "-f", ipaPath, "-u", ascUser, "-p", ascPassword)
	} else {
		return command.New(altoolPath, "--upload-app", "-f", ipaPath, "-u", ascUser, "-p", ascPassword, "--asc_provider", ascProvider)
	}
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

func failf(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}
