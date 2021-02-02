package main

import (
	"bytes"
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
	shellquote "github.com/kballard/go-shellquote"
)

// Config ...
type Config struct {
	BitriseConnection   string          `env:"connection,opt[automatic,api_key,apple_id,off]"`
	AppleID             string          `env:"itunescon_user"`
	Password            stepconf.Secret `env:"password"`
	AppSpecificPassword stepconf.Secret `env:"app_password"`
	APIKeyPath          stepconf.Secret `env:"api_key_path"`
	APIIssuer           string          `env:"api_issuer"`

	IpaPath           string `env:"ipa_path"`
	PkgPath           string `env:"pkg_path"`
	ItunesConnectUser string `env:"itunescon_user"`
	AdditionalParams  string `env:"altool_options"`
}

func (cfg Config) validateEnvs() error {
	if err := input.ValidateIfNotEmpty(cfg.IpaPath); err != nil {
		if err := input.ValidateIfNotEmpty(cfg.PkgPath); err != nil {
			return fmt.Errorf("neither ipa_path nor pkg_path is provided")
		}
	}

	var (
		isJWTAuthType     = (cfg.APIKeyPath != "" || cfg.APIIssuer != "")
		isAppleIDAuthType = (cfg.AppSpecificPassword != "" || cfg.Password != "" || cfg.ItunesConnectUser != "")
	)

	switch {

	case isAppleIDAuthType == isJWTAuthType:

		return fmt.Errorf("one type of authentication required, either provide itunescon_user with password/app_password or api_key_path with api_issuer")

	case isAppleIDAuthType:

		if err := input.ValidateIfNotEmpty(string(cfg.ItunesConnectUser)); err != nil {
			return fmt.Errorf("no itunescon_user provided")
		}
		if err := input.ValidateIfNotEmpty(string(cfg.Password)); err != nil {
			if err := input.ValidateIfNotEmpty(string(cfg.AppSpecificPassword)); err != nil {
				return fmt.Errorf("neither password nor app_password is provided")
			}
		}

	case isJWTAuthType:

		if err := input.ValidateIfNotEmpty(string(cfg.APIIssuer)); err != nil {
			return fmt.Errorf("no api_issuer provided")
		}
		if err := input.ValidateIfNotEmpty(string(cfg.APIKeyPath)); err != nil {
			return fmt.Errorf("no api_key_path provided")
		}

	}

	return nil
}

func copyOrDownloadFile(u *url.URL, pth string) error {
	if err := os.MkdirAll(filepath.Dir(pth), 0777); err != nil {
		return err
	}

	certFile, err := os.Create(pth)
	if err != nil {
		return err
	}
	defer func() {
		if err := certFile.Close(); err != nil {
			log.Errorf("Failed to close file, error: %s", err)
		}
	}()

	// if file -> copy
	if u.Scheme == "file" {
		b, err := ioutil.ReadFile(u.Path)
		if err != nil {
			return err
		}
		_, err = certFile.Write(b)
		return err
	}

	// otherwise download
	f, err := http.Get(u.String())
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Body.Close(); err != nil {
			log.Errorf("Failed to close file, error: %s", err)
		}
	}()

	_, err = io.Copy(certFile, f.Body)
	return err
}

func getKeyID(u *url.URL) string {
	var keyID = "Bitrise" // as default if no ID found in file name

	// get the ID of the key from the file
	if matches := regexp.MustCompile(`AuthKey_(.+)\.p8`).FindStringSubmatch(filepath.Base(u.Path)); len(matches) == 2 {
		keyID = matches[1]
	}

	return keyID
}

func getKeyPath(keyID string, keyPaths []string) (string, error) {
	certName := fmt.Sprintf("AuthKey_%s.p8", keyID)

	for _, path := range keyPaths {
		certPath := filepath.Join(path, certName)

		switch exists, err := pathutil.IsPathExists(certPath); {
		case err != nil:
			return "", err
		case exists:
			return certPath, os.ErrExist
		}
	}

	return filepath.Join(keyPaths[0], certName), nil
}

// prepares key and returns the ID the tool need to use
func prepareAPIKey(apiKeyPath string) (string, error) {
	// see these in the altool's man page
	var keyPaths = []string{
		filepath.Join(os.Getenv("HOME"), ".appstoreconnect/private_keys"),
		filepath.Join(os.Getenv("HOME"), ".private_keys"),
		filepath.Join(os.Getenv("HOME"), "private_keys"),
		"./private_keys",
	}

	fileURL, err := url.Parse(apiKeyPath)
	if err != nil {
		return "", err
	}

	keyID := getKeyID(fileURL)

	keyPath, err := getKeyPath(keyID, keyPaths)
	if err != nil {
		if err == os.ErrExist {
			return keyID, nil
		}
		return "", err
	}

	// cert file not found on any of the locations, so copy or download it then return it's ID
	if err := copyOrDownloadFile(fileURL, keyPath); err != nil {
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
	if string(cfg.AppSpecificPassword) != "" {
		password = string(cfg.AppSpecificPassword)
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

	additionalParams, err := shellquote.Split(cfg.AdditionalParams)
	if err != nil {
		failf("Failed to parse additional parameters, error: %s", err)
	}

	var cmd *command.Model
	if xcodeVersion.MajorVersion < 11 {
		altool := filepath.Join(xcpath, "/Contents/Applications/Application Loader.app/Contents/Frameworks/ITunesSoftwareService.framework/Support/altool")
		cmd = command.New(altool, append(append([]string{"--upload-app", "-f", filePth}, authParams...), additionalParams...)...)
	} else {
		if cfg.APIKeyPath != "" {
			apiKeyID, err := prepareAPIKey(string(cfg.APIKeyPath))
			if err != nil {
				failf("Failed to prepare certificate for authentication, error: %s", err)
			}
			authParams = []string{"--apiKey", apiKeyID, "--apiIssuer", cfg.APIIssuer}
		}
		cmd = command.New("xcrun", append(append([]string{"altool", "--upload-app", "-f", filePth}, authParams...), additionalParams...)...)
	}
	var outb bytes.Buffer
	cmd.SetStdout(&outb)
	cmd.SetStderr(os.Stderr)

	fileName := filepath.Base(filePth)
	commandStr := cmd.PrintableCommandArgs()

	if len(password) > 0 {
		commandStr = strings.Replace(commandStr, password, "[REDACTED]", -1)
	}

	log.Infof("Uploading - %s ...", fileName)
	log.Printf("$ %s", commandStr)

	if err := cmd.Run(); err != nil {
		failf("Uploading IPA failed: %s", err)
	}

	out := outb.String()

	if matches := regexp.MustCompile(`(?i)Generated JWT: (.*)`).FindStringSubmatch(out); len(matches) == 2 {
		out = strings.Replace(out, matches[1], "[REDACTED]", -1)
	}

	fmt.Println(out)

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
