package main

import (
	"fmt"
	"github.com/bitrise-io/go-xcode/models"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-xcode/utility"
	"github.com/bitrise-tools/go-steputils/input"
	"github.com/bitrise-tools/go-steputils/stepconf"
)

// Config ...
type Config struct {
	IpaPath           string          `env:"ipa_path"`
	PkgPath           string          `env:"pkg_path"`
	ItunesConnectUser string          `env:"itunescon_user"`
	Password          stepconf.Secret `env:"password"`
	AppPassword       stepconf.Secret `env:"app_password"`
	APIKey            string          `env:"api_key"`
	APIIssuer         string          `env:"api_issuer"`
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

	xcodeVersion, err := utility.GetXcodeVersion()
	if err != nil {
		failf("Failed to determine Xcode version, error: %s", err)
	}

	cmd, err := getUploadCommand(xcodeVersion, cfg, xcpath, filePth)
	if err != nil {
		failf("Failed to get authentication options", err)
	}

	if err := cmd.Run(); err != nil {
		failf("Uploading IPA failed: %s", err)
	}

	fmt.Println()
	log.Donef("IPA uploaded")
}

// getUploadCommand gets the upload command to run.
func getUploadCommand(xcodeVersion models.XcodebuildVersionModel, cfg Config, xcpath, filePth string) (*command.Model, error) {
	authOpts, redact, err := getAuthOptions(cfg)
	if err != nil {
		return nil, err
	}
	var cmd *command.Model
	if xcodeVersion.MajorVersion < 11 {
		altool := filepath.Join(xcpath, "/Contents/Applications/Application Loader.app/Contents/Frameworks/ITunesSoftwareService.framework/Support/altool")
		cmd = command.New(altool, "--upload-app", "-f", filePth, authOpts)
	} else {
		cmd = command.New("xcrun", "altool", "--upload-app", "-f", authOpts)
	}
	cmd.SetStdout(os.Stdout)
	cmd.SetStderr(os.Stderr)

	commandStr := cmd.PrintableCommandArgs()
	if redact != "" {
		commandStr = strings.Replace(commandStr, redact, "[REDACTED]", -1)
	}
	fileName := filepath.Base(filePth)
	log.Infof("Uploading - %s ...", fileName)
	log.Printf("$ %s", commandStr)
	return cmd, nil
}

// getAuthOptions provides the command options for authentication. Either a user and password pair or API key and issuer
// ID should be set in the config.
func getAuthOptions(cfg Config) (string, string, error) {
	passOptions, redact, passErr := getUserAndPasswordOptions(cfg)
	if passErr != nil {
		apiOptions, apiErr := getAPIKeyAndIssuerOptions(cfg)
		if apiErr != nil {
			return "", "", fmt.Errorf("neither usen and password or API key and issuer ID is provided correctly, please define one of the pairs in the config.\nIssue with user and password: %s\nIssue with API key and issuer ID: %s", passErr, apiErr)
		}
		return apiOptions, "", nil
	}
	return passOptions, redact, nil
}

// getUserAndPasswordOptions provides the user and password pair for authentication.
func getUserAndPasswordOptions(cfg Config) (string, string, error) {
	if cfg.ItunesConnectUser == "" {
		return "", "", fmt.Errorf("iTunes Connect user is not configured, please define it in the config")
	}
	password := getPasswordFromConfig(cfg)
	if password == "" {
		return "", "", fmt.Errorf("neither password or app password is configured, please define it the config")
	}
	return fmt.Sprintf("-u %s -p %s", cfg.ItunesConnectUser, password), password, nil
}

// getAPIKeyAndIsssuerOptions provides the API key and issuer ID pair for authentication.
func getAPIKeyAndIssuerOptions(cfg Config) (string, error) {
	if cfg.APIKey == "" {
		return "", fmt.Errorf("API key is not configured, please define it in the config")
	}
	if cfg.APIIssuer == "" {
		return "", fmt.Errorf("API Issuer is not configured, please define it in the config")
	}
	return fmt.Sprintf("--apiKey %s --apiIssuer %s", cfg.APIKey, cfg.APIIssuer), nil
}

func getPasswordFromConfig(cfg Config) string {
	password := string(cfg.Password)
	if string(cfg.AppPassword) != "" {
		password = string(cfg.AppPassword)
	}
	return password
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
