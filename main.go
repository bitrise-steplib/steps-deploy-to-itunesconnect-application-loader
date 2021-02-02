package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-steplib/steps-deploy-to-itunesconnect-deliver/appleauth"
	"github.com/bitrise-steplib/steps-deploy-to-itunesconnect-deliver/devportalservice"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
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

	// Used to get Bitrise Apple Developer Portal Connection
	BuildURL      string          `env:"BITRISE_BUILD_URL"`
	BuildAPIToken stepconf.Secret `env:"BITRISE_BUILD_API_TOKEN"`
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

func parseAuthSources(connection string) ([]appleauth.Source, error) {
	switch connection {
	case "automatic":
		return []appleauth.Source{
			&appleauth.ConnectionAPIKeySource{},
			&appleauth.ConnectionAppleIDSource{},
			&appleauth.InputAPIKeySource{},
			&appleauth.InputAppleIDSource{},
		}, nil
	case "api_key":
		return []appleauth.Source{&appleauth.ConnectionAPIKeySource{}}, nil
	case "apple_id":
		return []appleauth.Source{&appleauth.ConnectionAppleIDSource{}}, nil
	case "off":
		return []appleauth.Source{
			&appleauth.InputAPIKeySource{},
			&appleauth.InputAppleIDSource{},
		}, nil
	default:
		return nil, fmt.Errorf("invalid connection input: %s", connection)
	}
}

const notConnected = `Connected Apple Developer Portal Account not found.
Most likely because there is no Apple Developer Portal Account connected to the build, or the build is running locally.
Read more: https://devcenter.bitrise.io/getting-started/configuring-bitrise-steps-that-require-apple-developer-account-data/`

func handleSessionDataError(err error) {
	if err == nil {
		return
	}

	if networkErr, ok := err.(devportalservice.NetworkError); ok && networkErr.Status == http.StatusNotFound {
		log.Debugf("")
		log.Debugf("%s", notConnected)
	} else {
		fmt.Println()
		log.Errorf("Failed to activate Bitrise Apple Developer Portal connection: %s", err)
		log.Warnf("Read more: https://devcenter.bitrise.io/getting-started/configuring-bitrise-steps-that-require-apple-developer-account-data/")
	}
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

func writeAPIKey(privateKey, keyID string) error {
	// see these in the altool's man page
	var keyPaths = []string{
		filepath.Join(os.Getenv("HOME"), ".appstoreconnect/private_keys"),
		filepath.Join(os.Getenv("HOME"), ".private_keys"),
		filepath.Join(os.Getenv("HOME"), "private_keys"),
		"./private_keys",
	}

	keyPath, err := getKeyPath(keyID, keyPaths)
	if err != nil {
		if err == os.ErrExist {
			return nil
		}
		return err
	}

	if err := os.MkdirAll(filepath.Dir(keyPath), 0777); err != nil {
		return err
	}

	return fileutil.WriteStringToFile(keyPath, privateKey)
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

	authInputs := appleauth.Inputs{
		Username:            cfg.ItunesConnectUser,
		Password:            string(cfg.Password),
		AppSpecificPassword: string(cfg.AppSpecificPassword),
		APIIssuer:           cfg.APIIssuer,
		APIKeyPath:          string(cfg.APIKeyPath),
	}
	if err := authInputs.Validate(); err != nil {
		failf("Issue with authentication related inputs: %v", err)
	}

	//
	// Select and fetch Apple authenication source
	authSources, err := parseAuthSources(cfg.BitriseConnection)
	if err != nil {
		failf("Input error: unexpected value for Bitrise Apple Developer Connection (%s)", cfg.BitriseConnection)
	}

	var devportalConnectionProvider *devportalservice.BitriseClient
	if cfg.BuildURL != "" && cfg.BuildAPIToken != "" {
		devportalConnectionProvider = devportalservice.NewBitriseClient(http.DefaultClient, cfg.BuildURL, string(cfg.BuildAPIToken))
	} else {
		log.Warnf("Step is not running on bitrise.io: BITRISE_BUILD_URL and BITRISE_BUILD_API_TOKEN envs are not set")
	}
	var conn *devportalservice.AppleDeveloperConnection
	if cfg.BitriseConnection != "off" && devportalConnectionProvider != nil {
		var err error
		conn, err = devportalConnectionProvider.GetAppleDeveloperConnection()
		if err != nil {
			handleSessionDataError(err)
		}

		if conn == nil || (conn.APIKeyConnection == nil && conn.AppleIDConnection == nil) {
			fmt.Println()
			log.Debugf("%s", notConnected)
		}
	}

	authConfig, err := appleauth.Select(conn, authSources, authInputs)
	if err != nil {
		failf("Could not configure Apple Service authentication: %v", err)
	}
	if authConfig.AppleID != nil && authConfig.AppleID.AppSpecificPassword == "" {
		log.Warnf("If 2FA enabled, Application-specific password is required when using Apple ID authentication.")
	}

	// Prepare command
	var authParams []string
	if authConfig.APIKey != nil {
		if err := writeAPIKey(string(authConfig.APIKey.PrivateKey), authConfig.APIKey.KeyID); err != nil {
			failf("Failed to prepare certificate for authentication, error: %s", err)
		}
		authParams = []string{"--apiKey", authConfig.APIKey.KeyID, "--apiIssuer", authConfig.APIKey.IssuerID}
	} else {
		password := string(cfg.Password)
		if string(cfg.AppSpecificPassword) != "" {
			password = string(cfg.AppSpecificPassword)
		}
		authParams = []string{"-u", cfg.ItunesConnectUser, "-p", password}
	}

	filePth := cfg.IpaPath
	if cfg.PkgPath != "" {
		filePth = cfg.PkgPath
	}

	additionalParams, err := shellquote.Split(cfg.AdditionalParams)
	if err != nil {
		failf("Failed to parse additional parameters, error: %s", err)
	}

	altoolParams := append([]string{"altool", "--upload-app", "-f", filePth}, authParams...)
	altoolParams = append(altoolParams, additionalParams...)
	cmd := command.New("xcrun", altoolParams...)
	var outb bytes.Buffer
	cmd.SetStdout(&outb)
	cmd.SetStderr(os.Stderr)

	fileName := filepath.Base(filePth)
	log.Infof("Uploading - %s ...", fileName)

	commandStr := cmd.PrintableCommandArgs()
	if authConfig.APIKey == nil {
		if authConfig.AppleID.Password != "" {
			commandStr = strings.Replace(commandStr, authConfig.AppleID.Password, "[REDACTED]", -1)
		}
		if authConfig.AppleID.AppSpecificPassword != "" {
			commandStr = strings.Replace(commandStr, authConfig.AppleID.AppSpecificPassword, "[REDACTED]", -1)
		}
	}
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

func failf(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}
