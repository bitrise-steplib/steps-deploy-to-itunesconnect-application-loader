package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	httpretry "github.com/bitrise-io/go-utils/retry"
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/bitrise-io/go-xcode/appleauth"
	"github.com/bitrise-io/go-xcode/devportalservice"
	"github.com/bitrise-io/go-xcode/ipa"
	"github.com/bitrise-io/go-xcode/plistutil"
	"github.com/bitrise-io/go-xcode/utility"
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
	Platform          string `env:"platform,opt[auto,ios,macos,tvos]"`
	ItunesConnectUser string `env:"itunescon_user"`
	AdditionalParams  string `env:"altool_options"`
	RetryTimes        string `env:"retries"`

	// Used to get Bitrise Apple Developer Portal Connection
	BuildURL      string          `env:"BITRISE_BUILD_URL"`
	BuildAPIToken stepconf.Secret `env:"BITRISE_BUILD_API_TOKEN"`
}

type platformType string

const (
	iOS   platformType = "ios"
	tvOS               = "appletvos"
	macOS              = "macos"
)

func (cfg Config) validateArtifact() error {
	cfg.IpaPath = strings.TrimSpace(cfg.IpaPath)
	cfg.PkgPath = strings.TrimSpace(cfg.PkgPath)

	var (
		deployIPA = cfg.IpaPath != ""
		deployPKG = cfg.PkgPath != ""
	)

	if deployIPA == deployPKG {
		return fmt.Errorf("one artifact is required and only one is allowed, either provide ipa_path or pkg_path")
	}

	return nil
}

// getPlatformType maps platform to an altool parameter
//  -t, --type {macos | ios | appletvos}     Specify the platform of the file, or of the host app when using --upload-hosted-content. (Output by 'xcrun altool -h')
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
Most likely because there is no Apple Developer Portal Account connected to the build.
Read more: https://devcenter.bitrise.io/getting-started/configuring-bitrise-steps-that-require-apple-developer-account-data/`

func handleSessionDataError(err error) {
	if err == nil {
		return
	}

	if networkErr, ok := err.(devportalservice.NetworkError); ok && networkErr.Status == http.StatusUnauthorized {
		fmt.Println()
		log.Warnf("%s", "Unauthorized to query Connected Apple Developer Portal Account. This happens by design, with a public app's PR build, to protect secrets.")

		return
	}

	fmt.Println()
	log.Errorf("Failed to activate Bitrise Apple Developer Portal connection: %s", err)
	log.Warnf("Read more: https://devcenter.bitrise.io/getting-started/configuring-bitrise-steps-that-require-apple-developer-account-data/")
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

const typeKey = "--type"

func main() {
	var cfg Config
	if err := stepconf.Parse(&cfg); err != nil {
		failf("Error: %s", err)
	}

	stepconf.Print(cfg)
	fmt.Println()

	if err := cfg.validateArtifact(); err != nil {
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

	xcodeVersion, err := utility.GetXcodeVersion()
	if err != nil {
		failf("Failed to determine Xcode version: %s", err)
	}

	//
	// Select and fetch Apple authenication source
	authSources, err := parseAuthSources(cfg.BitriseConnection)
	if err != nil {
		failf("Invalid input: unexpected value for Bitrise Apple Developer Connection (%s)", cfg.BitriseConnection)
	}

	var devportalConnectionProvider *devportalservice.BitriseClient
	if cfg.BuildURL != "" && cfg.BuildAPIToken != "" {
		devportalConnectionProvider = devportalservice.NewBitriseClient(httpretry.NewHTTPClient().StandardClient(), cfg.BuildURL, string(cfg.BuildAPIToken))
	} else {
		fmt.Println()
		log.Warnf("Connected Apple Developer Portal Account not found. Step is not running on bitrise.io: BITRISE_BUILD_URL and BITRISE_BUILD_API_TOKEN envs are not set")
	}
	var conn *devportalservice.AppleDeveloperConnection
	if cfg.BitriseConnection != "off" && devportalConnectionProvider != nil {
		var err error
		conn, err = devportalConnectionProvider.GetAppleDeveloperConnection()
		if err != nil {
			handleSessionDataError(err)
		}

		if conn != nil && (conn.APIKeyConnection == nil && conn.AppleIDConnection == nil) {
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
		password := string(authConfig.AppleID.Password)
		if string(authConfig.AppleID.AppSpecificPassword) != "" {
			password = string(authConfig.AppleID.AppSpecificPassword)
		}
		authParams = []string{"--username", authConfig.AppleID.Username, "--password", password}
	}

	filePth := cfg.IpaPath
	if filePth == "" {
		filePth = cfg.PkgPath
	}
	if filePth == "" {
		failf("Either IPA path or PKG path has to be provided")
	}

	additionalParams, err := shellquote.Split(cfg.AdditionalParams)
	if err != nil {
		failf("Failed to parse additional parameters, error: %s", err)
	}

	uploadParams := []string{"--upload-app", "-f", filePth}
	// Platform type parameter was introduced in Xcode 13
	if xcodeVersion.MajorVersion >= 13 && !sliceutil.IsStringInSlice(typeKey, additionalParams) {
		uploadParams = append(uploadParams, typeKey, string(getPlatformType(cfg.IpaPath, cfg.Platform)))
	}

	altoolParams := append([]string{"altool"}, uploadParams...)
	altoolParams = append(altoolParams, authParams...)
	altoolParams = append(altoolParams, additionalParams...)
	out, err := uploadWithRetry(newAltoolUploader(altoolParams, filePth, authConfig), cfg.RetryTimes)
	if err != nil {
		fmt.Println(out)
		failf("Uploading IPA failed: %s", err)
	}

	fmt.Println(out)
	log.Donef("IPA uploaded")
}

type uploader interface {
	upload() (string, string, error)
}

type altoolUploader struct {
	altoolParams []string
	filePth      string
	authConfig   appleauth.Credentials
}

func newAltoolUploader(altoolParams []string, filePth string, authConfig appleauth.Credentials) uploader {
	return altoolUploader{altoolParams: altoolParams, filePth: filePth, authConfig: authConfig}
}

func (a altoolUploader) upload() (string, string, error) {
	cmd := command.New("xcrun", a.altoolParams...)
	var sb bytes.Buffer
	var eb bytes.Buffer
	cmd.SetStdout(&sb)
	cmd.SetStderr(io.MultiWriter(&eb, &sb))

	fileName := filepath.Base(a.filePth)
	log.Infof("Uploading - %s ...", fileName)

	commandStr := cmd.PrintableCommandArgs()
	authConfig := a.authConfig
	if authConfig.APIKey == nil {
		if authConfig.AppleID.Password != "" {
			commandStr = strings.Replace(commandStr, authConfig.AppleID.Password, "[REDACTED]", -1)
		}
		if authConfig.AppleID.AppSpecificPassword != "" {
			commandStr = strings.Replace(commandStr, authConfig.AppleID.AppSpecificPassword, "[REDACTED]", -1)
		}
	}
	log.Printf("$ %s", commandStr)

	err := cmd.Run()
	combinedOutput := sb.String()
	errorOutput := eb.String()
	if errorOutput != "" {
		log.Errorf("%s", errorOutput)
	}

	if matches := regexp.MustCompile(`(?i)Generated JWT: (.*)`).FindStringSubmatch(combinedOutput); len(matches) == 2 {
		combinedOutput = strings.Replace(combinedOutput, matches[1], "[REDACTED]", -1)
	}

	if err != nil {
		if errorutil.IsExitStatusError(err) {
			return combinedOutput, errorOutput, fmt.Errorf("xcrun command failed: %w", err)
		}

		return combinedOutput, errorOutput, fmt.Errorf("command execution failed: %w", err)
	}

	return combinedOutput, errorOutput, nil
}

func uploadWithRetry(uploader uploader, retryTimes string, opts ...retry.Option) (string, error) {
	var regexList = []string{
		// https://bitrise.atlassian.net/browse/STEP-1190
		`(?s).*Unable to determine the application using bundleId.*-19201.*`,
		`(?s).*Unable to determine app platform for 'Undefined' software type.*1194.*`,
		`(?s).*TransporterService.*error occurred trying to read the bundle.*-18000.*`,
		`(?s).*Unable to authenticate.*-19209.*`,
		`(?s).*server returned an invalid response.*try your request again.*`,
		`(?s).*The request timed out.*`,
	}
	var result string
	parsedRetryTimes, err := strconv.ParseInt(retryTimes, 10, 32)
	attempts := uint(parsedRetryTimes)
	if err != nil {
		attempts = uint(10)
	}
	mOpts := []retry.Option{
		retry.Attempts(attempts),
		retry.Delay(300 * time.Millisecond),
		retry.LastErrorOnly(true),
		retry.OnRetry(func(n uint, err error) {
			if n == 0 {
				log.Warnf("Upload failed, but we recognized it as possibly recoverable error, retrying...")
			} else if n != attempts-1 {
				log.Warnf("Attempt %d failed, retrying...", n+1)
			} else {
				log.Warnf("Attempt %d failed", attempts)
			}
		}),
	}

	for _, opt := range opts {
		mOpts = append(mOpts, opt)
	}

	err = retry.Do(
		func() error {
			r, errorString, err := uploader.upload()
			result = r
			if err != nil {
				for _, re := range regexList {
					matched, err2 := regexp.MatchString(re, errorString)
					if err2 != nil {
						log.Warnf("Couldn't match %s with regex %s", errorString, re)
						continue
					}
					if matched {
						return err
					}
				}
				return retry.Unrecoverable(err)
			}
			return nil
		},
		mOpts...)
	if err != nil {
		return result, err
	}
	return result, nil
}

func failf(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}
