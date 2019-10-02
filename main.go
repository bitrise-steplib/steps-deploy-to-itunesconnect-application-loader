package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bitrise-io/go-utils/pathutil"

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
	APIKeyPath        string          `env:"api_key_path"`
	APIIssuer         string          `env:"api_issuer"`
}

func (cfg Config) validateEnvs() error {
	if err := input.ValidateIfNotEmpty(cfg.IpaPath); err != nil {
		if err := input.ValidateIfNotEmpty(cfg.PkgPath); err != nil {
			return fmt.Errorf("neither ipa_path nor pkg_path is provided")
		}
	}

	switch isJWTAuthType, isAppleIDAuthType := (cfg.APIKeyPath != "" || cfg.APIIssuer != ""), (cfg.AppPassword != "" || cfg.Password != "" || cfg.ItunesConnectUser != ""); {

	case isAppleIDAuthType == isJWTAuthType:

		return fmt.Errorf("one type of authentication required, either provide Apple ID with password/app password or API key with issuer")

	case isAppleIDAuthType:

		if err := input.ValidateIfNotEmpty(string(cfg.ItunesConnectUser)); err != nil {
			return fmt.Errorf("no Apple ID provided")
		}
		if err := input.ValidateIfNotEmpty(string(cfg.Password)); err != nil {
			if err := input.ValidateIfNotEmpty(string(cfg.AppPassword)); err != nil {
				return fmt.Errorf("neither password nor app_password is provided")
			}
		}

	case isJWTAuthType:

		if err := input.ValidateIfNotEmpty(string(cfg.APIIssuer)); err != nil {
			return fmt.Errorf("no API Issuer provided")
		}
		if err := input.ValidateIfNotEmpty(string(cfg.APIKeyPath)); err != nil {
			return fmt.Errorf("no API Key path provided")
		}

	}

	return nil
}

// prepares key and returns the ID the tool need to use
func prepareAPIKey(apiKeyPath string) (string, error) {
	// input should accept url and filepath
	// 4 location existing, check the files there first
	// if there is a file on any of the 4 locations use that
	// if ther eis no that file found then move it to one of the locations
	// if the filename is not conventional for the tool then create a new unique one

	// see these in the altool's man page
	var (
		keyPaths = []string{filepath.Join(os.Getenv("HOME"), ".appstoreconnect/private_keys"), "./private_keys", filepath.Join(os.Getenv("HOME"), "private_keys"), filepath.Join(os.Getenv("HOME"), ".private_keys")}
		keyID    = "Bitrise" // as default if no ID found in file name
	)

	// parse string to url
	fileURL, err := url.Parse(apiKeyPath)
	if err != nil {
		return "", err
	}

	// get the ID of the key from the file
	if matches := regexp.MustCompile(`AuthKey_(.+)\.p8`).FindStringSubmatch(filepath.Base(fileURL.Path)); len(matches) == 2 {
		keyID = matches[1]
	}

	certName := fmt.Sprintf("AuthKey_%s.p8", keyID)

	// if certName already exists on any of the following locations then return that's ID here
	for _, path := range keyPaths {
		exists, err := pathutil.IsPathExists(filepath.Join(path, certName))
		if err != nil {
			return "", err
		}
		if exists {
			return keyID, nil
		}
	}

	// cert file not found on any of the locations, so copy or download it

	// create cert file on the first location
	certFile, err := os.Create(filepath.Join(keyPaths[0], certName))
	if err != nil {
		return "", err
	}
	defer func() {
		if err := certFile.Close(); err != nil {
			log.Errorf("Failed to close file, error: %s", err)
		}
	}()

	// if file -> download
	if fileURL.Scheme == "file" {
		b, err := ioutil.ReadFile(fileURL.Path)
		if err != nil {
			return "", err
		}
		_, err = certFile.Write(b)
		return keyID, err
	}

	// otherwise download
	f, err := http.Get(apiKeyPath)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := f.Body.Close(); err != nil {
			log.Errorf("Failed to close file, error: %s", err)
		}
	}()

	if _, err := io.Copy(certFile, f.Body); err != nil {
		return "", err
	}

	return keyID, nil
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

	xcodeVersion, err := utility.GetXcodeVersion()
	if err != nil {
		failf("Failed to determine Xcode version, error: %s", err)
	}

	authParams := []string{"-u", cfg.ItunesConnectUser, "-p", password}

	if cfg.APIKeyPath != "" {
		apiKeyID, err := prepareAPIKey(cfg.APIKeyPath)
		if err != nil {
			failf("Failed to prepare certificate for authentication, error: %s", err)
		}
		authParams = []string{"--apiKey", apiKeyID, "--apiIssuer", cfg.APIIssuer}
	}

	var cmd *command.Model
	if xcodeVersion.MajorVersion < 11 {
		altool := filepath.Join(xcpath, "/Contents/Applications/Application Loader.app/Contents/Frameworks/ITunesSoftwareService.framework/Support/altool")
		cmd = command.New(altool, append([]string{"--upload-app", "-f", filePth}, authParams...)...)
	} else {
		cmd = command.New("xcrun", append([]string{"altool", "--upload-app", "-f", filePth}, authParams...)...)
	}
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

func failf(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}
