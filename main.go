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
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/pathutil"
	httpretry "github.com/bitrise-io/go-utils/retry"
	"github.com/bitrise-io/go-utils/sliceutil"
	fileutilv2 "github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-xcode/appleauth"
	"github.com/bitrise-io/go-xcode/devportalservice"
	"github.com/bitrise-io/go-xcode/utility"
	"github.com/bitrise-io/go-xcode/v2/metaparser"
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

	IpaPath string `env:"ipa_path"`
	PkgPath string `env:"pkg_path"`

	// App details
	Platform           string `env:"platform,opt[auto,ios,macos,tvos]"`
	AppID              string `env:"app_id"`
	BundleID           string `env:"bundle_id"`
	BundleVersion      string `env:"bundle_version"`
	BundleShortVersion string `env:"bundle_short_version_string"`

	// Debug
	IsVerbose        bool   `env:"verbose_log,opt[yes,no]"`
	AdditionalParams string `env:"altool_options"`
	RetryTimes       string `env:"retries"`

	// Used to get Bitrise Apple Developer Portal Connection
	BuildURL      string          `env:"BITRISE_BUILD_URL"`
	BuildAPIToken stepconf.Secret `env:"BITRISE_BUILD_API_TOKEN"`
}

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

func handleSessionDataError(logger log.Logger, err error) {
	if err == nil {
		return
	}

	if networkErr, ok := err.(devportalservice.NetworkError); ok && networkErr.Status == http.StatusUnauthorized {
		fmt.Println()
		logger.Warnf("%s", "Unauthorized to query Connected Apple Developer Portal Account. This happens by design, with a public app's PR build, to protect secrets.")

		return
	}

	fmt.Println()
	logger.Errorf("Failed to activate Bitrise Apple Developer Portal connection: %s", err)
	logger.Warnf("Read more: https://devcenter.bitrise.io/getting-started/configuring-bitrise-steps-that-require-apple-developer-account-data/")
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

const (
	typeKey    = "--type"
	verboseKey = "--verbose"
)

func main() {
	logger := log.NewLogger()
	parser := metaparser.New(logger, fileutilv2.NewFileManager())

	var cfg Config
	if err := stepconf.Parse(&cfg); err != nil {
		failf(logger, "Error: %s", err)
	}

	stepconf.Print(cfg)
	fmt.Println()
	logger.EnableDebugLog(cfg.IsVerbose)

	if err := cfg.validateArtifact(); err != nil {
		failf(logger, "Input error: %s", err)
	}

	cfg.AppID = strings.TrimSpace(cfg.AppID)
	cfg.BundleID = strings.TrimSpace(cfg.BundleID)
	cfg.BundleVersion = strings.TrimSpace(cfg.BundleVersion)
	cfg.BundleShortVersion = strings.TrimSpace(cfg.BundleShortVersion)

	authInputs := appleauth.Inputs{
		Username:            cfg.AppleID,
		Password:            string(cfg.Password),
		AppSpecificPassword: string(cfg.AppSpecificPassword),
		APIIssuer:           cfg.APIIssuer,
		APIKeyPath:          string(cfg.APIKeyPath),
	}
	if err := authInputs.Validate(); err != nil {
		failf(logger, "Issue with authentication related inputs: %v", err)
	}

	xcodeVersion, err := utility.GetXcodeVersion()
	if err != nil {
		failf(logger, "Failed to determine Xcode version: %s", err)
	}

	// Select and fetch Apple authenication source
	authSources, err := parseAuthSources(cfg.BitriseConnection)
	if err != nil {
		failf(logger, "Invalid input: unexpected value for Bitrise Apple Developer Connection (%s)", cfg.BitriseConnection)
	}

	var devportalConnectionProvider *devportalservice.BitriseClient
	if cfg.BuildURL != "" && cfg.BuildAPIToken != "" {
		devportalConnectionProvider = devportalservice.NewBitriseClient(httpretry.NewHTTPClient().StandardClient(), cfg.BuildURL, string(cfg.BuildAPIToken))
	} else {
		fmt.Println()
		logger.Warnf("Connected Apple Developer Portal Account not found. Step is not running on bitrise.io: BITRISE_BUILD_URL and BITRISE_BUILD_API_TOKEN envs are not set")
	}
	var conn *devportalservice.AppleDeveloperConnection
	if cfg.BitriseConnection != "off" && devportalConnectionProvider != nil {
		var err error
		conn, err = devportalConnectionProvider.GetAppleDeveloperConnection()
		if err != nil {
			handleSessionDataError(logger, err)
		}

		if conn != nil && (conn.APIKeyConnection == nil && conn.AppleIDConnection == nil) {
			fmt.Println()
			logger.Debugf("%s", notConnected)
		}
	}

	authConfig, err := appleauth.Select(conn, authSources, authInputs)
	if err != nil {
		failf(logger, "Could not configure Apple Service authentication: %v", err)
	}
	if authConfig.AppleID != nil && authConfig.AppleID.AppSpecificPassword == "" {
		logger.Warnf("If 2FA enabled, Application-specific password is required when using Apple ID authentication.")
	}

	// Prepare command
	var authParams []string
	if authConfig.APIKey != nil {
		if err := writeAPIKey(string(authConfig.APIKey.PrivateKey), authConfig.APIKey.KeyID); err != nil {
			failf(logger, "Failed to prepare certificate for authentication, error: %s", err)
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
		failf(logger, "Either IPA path or PKG path has to be provided")
	}

	additionalParams, err := shellquote.Split(cfg.AdditionalParams)
	if err != nil {
		failf(logger, "Failed to parse additional parameters, error: %s", err)
	}

	var uploadParams []string
	if xcodeVersion.MajorVersion >= 26 {
		// Use upload-package from Xcode 26. This will cause less of a breaking change,
		// as App ID, BundleID, Version and ShortVersion are optional in Xcode 26, but required in Xcode 16.
		uploadParams = []string{"--upload-package", filePth}
	} else {
		uploadParams = []string{"--upload-app", "-f", filePth}
	}

	// Platform type parameter was introduced in Xcode 13
	if !sliceutil.IsStringInSlice(typeKey, additionalParams) {
		uploadParams = append(uploadParams, typeKey, string(getPlatformType(logger, cfg.IpaPath, cfg.Platform)))
	}

	if xcodeVersion.MajorVersion < 26 && cfg.AppID != "" {
		logger.Warnf("App ID is not supported with Xcode versions below 26, ignoring it.")
	}
	if xcodeVersion.MajorVersion >= 26 && cfg.AppID != "" { // If App ID is provided, BundleID, Version and ShortVersion must be provided too, or read from the package
		if cfg.IpaPath == "" {
			failf(logger, "App ID not supported with PKG upload yet.")
		}

		packageDetails, err := readPackageDetails(parser, filePth)
		if err != nil {
			failf(logger, "Failed to read package details: %s", err)
		}
		if cfg.BundleID == "" {
			cfg.BundleID = packageDetails.bundleID
		}
		if cfg.BundleVersion == "" {
			cfg.BundleVersion = packageDetails.bundleVersion
		}
		if cfg.BundleShortVersion == "" {
			cfg.BundleShortVersion = packageDetails.bundleShortVersionString
		}

		uploadParams = append(uploadParams, "--apple-id", cfg.AppID) // Specifies the App Store Connect Apple ID of the app. (e.g. 1023456789)
		uploadParams = append(uploadParams, "--bundle-id", cfg.BundleID)
		uploadParams = append(uploadParams, "--bundle-version", cfg.BundleVersion)                   // Specifies the CFBundleVersion of the app package.
		uploadParams = append(uploadParams, "--bundle-short-version-string", cfg.BundleShortVersion) // Specifies the CFBundleShortVersionString of the app package.
	}
	if cfg.IsVerbose && !sliceutil.IsStringInSlice(verboseKey, additionalParams) {
		additionalParams = append(additionalParams, verboseKey)
	}

	altoolParams := append([]string{"altool"}, uploadParams...)
	altoolParams = append(altoolParams, authParams...)
	altoolParams = append(altoolParams, additionalParams...)
	out, err := uploadWithRetry(logger, newAltoolUploader(logger, altoolParams, filePth, authConfig), cfg.RetryTimes)
	if err != nil {
		failf(logger, "Uploading IPA failed: %s", err)
	}

	if matches := regexp.MustCompile(`(?i)Generated JWT: (.*)`).FindStringSubmatch(out); len(matches) == 2 {
		out = strings.ReplaceAll(out, matches[1], "[REDACTED]")
	}

	fmt.Println(out)

	logger.Donef("IPA uploaded")
}

type uploader interface {
	upload() (string, string, error)
}

type altoolUploader struct {
	logger       log.Logger
	altoolParams []string
	filePth      string
	authConfig   appleauth.Credentials
}

func newAltoolUploader(logger log.Logger, altoolParams []string, filePth string, authConfig appleauth.Credentials) uploader {
	return altoolUploader{logger: logger, altoolParams: altoolParams, filePth: filePth, authConfig: authConfig}
}

func (a altoolUploader) upload() (string, string, error) {
	cmd := command.New("xcrun", a.altoolParams...)
	var sb bytes.Buffer
	var eb bytes.Buffer
	cmd.SetStdout(io.MultiWriter(&sb, os.Stdout))
	cmd.SetStderr(io.MultiWriter(&eb, os.Stderr))

	fileName := filepath.Base(a.filePth)
	a.logger.Infof("Uploading - %s ...", fileName)

	commandStr := cmd.PrintableCommandArgs()
	authConfig := a.authConfig
	if authConfig.APIKey == nil {
		if authConfig.AppleID.Password != "" {
			commandStr = strings.ReplaceAll(commandStr, authConfig.AppleID.Password, "[REDACTED]")
		}
		if authConfig.AppleID.AppSpecificPassword != "" {
			commandStr = strings.ReplaceAll(commandStr, authConfig.AppleID.AppSpecificPassword, "[REDACTED]")
		}
	}
	a.logger.Printf("$ %s", commandStr)

	err := cmd.Run()
	ioString := sb.String()
	errorString := eb.String()

	if err != nil {
		return ioString, errorString, err
	}

	// Xcode 26RC altool always returns exit code 0, even on some failures
	errorRe := regexp.MustCompile(`(?s).*ERROR:.*`)
	sucessRe := regexp.MustCompile(`(?s).*UPLOAD SUCCEEDED.*`)
	if errorRe.MatchString(errorString) && !sucessRe.MatchString(ioString) && !sucessRe.MatchString(errorString) {
		return ioString, errorString, fmt.Errorf("Upload failed, output: %s", errorString)
	}

	return ioString, errorString, nil
}

func uploadWithRetry(logger log.Logger, uploader uploader, retryTimes string, opts ...retry.Option) (string, error) {
	var retriableRegexes = []*regexp.Regexp{
		// https://bitrise.atlassian.net/browse/STEP-1190
		regexp.MustCompile(`(?s).*Unable to determine the application using bundleId.*-19201.*`),
		regexp.MustCompile(`(?s).*Unable to determine app platform for 'Undefined' software type.*1194.*`),
		regexp.MustCompile(`(?s).*TransporterService.*error occurred trying to read the bundle.*-18000.*`),
		regexp.MustCompile(`(?s).*Unable to authenticate.*-19209.*`),
		regexp.MustCompile(`(?s).*server returned an invalid response.*try your request again.*`),
		regexp.MustCompile(`(?s).*The request timed out.*`),
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
				logger.Warnf("Upload failed, but we recognized it as possibly recoverable error, retrying...")
			} else if n != attempts-1 {
				logger.Warnf("Attempt %d failed, retrying...", n+1)
			} else {
				logger.Warnf("Attempt %d failed", attempts)
			}
		}),
	}

	mOpts = append(mOpts, opts...)

	err = retry.Do(
		func() error {
			r, errorString, err := uploader.upload()
			result = r
			if err != nil {
				fmt.Printf("Upload error, checking retries: %s\n", err)
				for _, re := range retriableRegexes {
					matched := re.MatchString(errorString)
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
		return "", err
	}
	return result, nil
}

func failf(logger log.Logger, format string, v ...interface{}) {
	logger.Errorf(format, v...)
	os.Exit(1)
}
