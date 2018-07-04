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

	cmd := command.New(`/Applications/Xcode.app/Contents/Applications/Application Loader.app/Contents/Frameworks/ITunesSoftwareService.framework/Support/altool`, "--upload-app", "-f", filePth, "-u", cfg.ItunesConnectUser, "-p", password)
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

func failf(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}
