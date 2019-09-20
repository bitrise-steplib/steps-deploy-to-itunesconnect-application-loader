package main

import (
	"fmt"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-xcode/models"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
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
	APIKeyPath        string          `env:"api_key_path"`
	APIIssuer         string          `env:"api_issuer"`
}

var keyPaths = []string{"./private_keys", "~/private_keys", "~/.appstoreconnect/private_keys"}

const keyFormat = "AuthKey_(.+)\\.p8"

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
		cmd = command.New(altool, append([]string{"--upload-app", "-f", filePth}, authOpts...)...)
	} else {
		cmd = command.New("xcrun", append([]string{"altool", "--upload-app", "-f"}, authOpts...)...)
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
func getAuthOptions(cfg Config) ([]string, string, error) {
	passOptions, redact, passErr := getUserAndPasswordOptions(cfg)
	if passErr != nil {
		apiOptions, apiErr := getAPIKeyAndIssuerOptions(cfg)
		if apiErr != nil {
			return []string{}, "", fmt.Errorf("neither user and password or API key and issuer ID is provided correctly, please define one of the pairs in the config.\nIssue with user and password: %s\nIssue with API key and issuer ID: %s", passErr, apiErr)
		}
		return apiOptions, "", nil
	}
	return passOptions, redact, nil
}

// getUserAndPasswordOptions provides the user and password pair for authentication.
func getUserAndPasswordOptions(cfg Config) ([]string, string, error) {
	if cfg.ItunesConnectUser == "" {
		return []string{}, "", fmt.Errorf("iTunes Connect user is not configured, please define it in the config")
	}
	password := getPasswordFromConfig(cfg)
	if password == "" {
		return []string{}, "", fmt.Errorf("neither password or app password is configured, please define it the config")
	}
	return []string{"-u", cfg.ItunesConnectUser, "-p", password}, password, nil
}

// getPasswordFromConfig gets the password from the config.
func getPasswordFromConfig(cfg Config) string {
	password := string(cfg.Password)
	if string(cfg.AppPassword) != "" {
		password = string(cfg.AppPassword)
	}
	return password
}

// getAPIKeyAndIssuerOptions provides the API key and issuer ID pair for authentication.
func getAPIKeyAndIssuerOptions(cfg Config) ([]string, error) {
	if cfg.APIKeyPath == "" {
		return []string{}, fmt.Errorf("API key is not configured, please define it in the config")
	}
	if cfg.APIIssuer == "" {
		return []string{}, fmt.Errorf("API Issuer is not configured, please define it in the config")
	}

	APIKey, err := getAPIKey(cfg.APIKeyPath, keyPaths)
	if err != nil {
		return []string{}, err
	}
	return []string{"--apiKey", APIKey, "--apiIssuer", cfg.APIIssuer}, nil
}

// getAPIKey gets the API key for the altool command from the path of the input key.
func getAPIKey(keyPath string, keyDirs []string) (string, error) {
	keyName := path.Base(keyPath)
	if checkKeyFormat(keyName) && checkKeyExistsAtDirs(keyName, keyDirs) {
		APIKey, err := getAPIKeyFromFileName(keyName)
		if err != nil {
			return "", err
		}
		return APIKey, nil
	}

	var APIKey string
	var err error
	if strings.HasPrefix(keyPath, "file://") {
		APIKey, err = prepareAndGetLocalKey(keyPath)
		if err != nil {
			return "", fmt.Errorf("failed to prepare key from local file, error: %s", err)
		}
		return APIKey, nil
	}

	APIKey, err = prepareAndGetExternalKey(keyPath)
	if err != nil {
		return "", fmt.Errorf("failed to prepare key from external file, error: %s", err)
	}
	return APIKey, nil
}

// getAPIKeyFromPath gets the API key from the final path of the key.
func getAPIKeyFromPath(pth string) (string, error) {
	keyName := path.Base(pth)
	dstName, err := getDstName(keyName)
	if err != nil {
		return "", err
	}
	return dstName, nil
}

// prepareAndGetLocalKey copies the local key to the proper destination and returns the API key.
func prepareAndGetLocalKey(keyPath string) (string, error) {
	pth := strings.TrimPrefix(keyPath, "file://")
	dstPath, err := getDstPath(pth)
	if err != nil {
		return "", err
	}
	if err := command.CopyFile(pth, dstPath); err != nil {
		return "", fmt.Errorf("failed to copy key to destination %s, error: %s", dstPath, err)
	}
	return getAPIKeyFromPath(dstPath)
}

// prepareAndGetLocalKey downloads the external key to the proper destination and returns the API key.
func prepareAndGetExternalKey(url string) (string, error) {
	log.Infof("Downloading key")
	dstPath, err := getDstPath(url)
	if err != nil {
		return "", fmt.Errorf("could not download key. Error: %s", err)
	}
	if err := download(url, dstPath); err != nil {
		return "", fmt.Errorf("failed to download keystore, error: %s", err)
	}
	return getAPIKeyFromPath(dstPath)
}

// checkKeyFormat checks if the given key's name matches the required format by altool.
func checkKeyFormat(keyName string) bool {
	if _, err := getAPIKeyFromFileName(keyName); err != nil {
		return false
	}
	return true
}

// getAPIKeyFromFileName gets the API key from a given file name.
func getAPIKeyFromFileName(keyName string) (string, error) {
	regex := regexp.MustCompile(keyFormat)
	if regex.MatchString(keyName) {
		return regex.FindStringSubmatch(keyName)[1], nil
	}
	return "", fmt.Errorf("key with name %s does not match the required format %s", keyName, keyFormat)
}

// checkKeyExistsAtDirs check that the given key exists at the predefined paths for altool.
func checkKeyExistsAtDirs(keyName string, dirs []string) bool {
	for _, basePth := range dirs {
		pth := path.Join(basePth, keyName)
		if fileExists(pth) {
			log.Debugf("Found key at %s", pth)
			return true
		}
	}
	return false
}

// fileExists checks if the given file exists (and not a directory) or not.
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// getDstPath gets the destination path for the key.
func getDstPath(keyPath string) (string, error) {
	dstName, err := getDstName(path.Base(keyPath))
	if err != nil {
		return "", fmt.Errorf("could not get destination path. Error: %s", err)
	}
	return path.Join(keyPaths[0], dstName), nil
}

// getDstName gets the name for the key.
func getDstName(keyName string) (string, error) {
	if keyName == "" || keyName == "." {
		return "", fmt.Errorf("keyname could not be empty")
	}
	dstName := strings.Replace(keyFormat, "(.+)\\", "%s", 1)
	return fmt.Sprintf(dstName, pathutil.GetFileName(keyName)), nil
}

// download downloads a file from a given URL to the given path.
func download(url, pth string) error {
	out, err := os.Create(pth)
	defer func() {
		if err := out.Close(); err != nil {
			log.Warnf("Failed to close file: %s. Error: %s", out, err)
		}
	}()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Warnf("Failed to close response body. Error: %s", err)
		}
	}()

	_, err = io.Copy(out, resp.Body)
	return err
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
		return "", fmt.Errorf("failed to find Xcode path from response %s", resp)
	}

	xcodePath := split[0]
	log.Debugf("Found Xcode path at %s", xcodePath)
	return xcodePath, nil
}

func failf(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}
