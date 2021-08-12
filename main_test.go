package main

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/avast/retry-go/v3"
	"github.com/stretchr/testify/assert"

	"github.com/bitrise-io/go-utils/pathutil"
)

const unableToDetermine = `*** Error: Error uploading '/Users/vagrant/deploy/MY-test-ios.ipa'.
*** Error: Failed to upload package. Unable to determine the application using bundleId: io.bitrise.test. (-19201)`

const transporterService = `*** Error: Error uploading '/Users/vagrant/deploy/io.bitrise.ipa'.
*** Error: could not find the service with interface (com.apple.transporter.osgi.TransporterService) (-18000)
*** Error: Resolver: Install error - com.apple.transporter.aspera-macos-x64 Exception's name: org.osgi.framework.BundleException, Exception's message: An error occurred trying to read the bundle (-18000)`

const invalidResponse = `*** Error: Server returned an invalid MIME type: text/plain, body: Unauthenticated

Request ID: MQ4DVCDRVZ4VS33A66KIKKP6SQ.0.0
*** Error: Errors uploading '/Users/vagrant/deploy/bitrise.ipa': (
    "Error Domain=NSCocoaErrorDomain Code=-1011 \"Authentication failed\" UserInfo={NSLocalizedDescription=Authentication failed, NSLocalizedFailureReason=Failed to authenticate for session: (\n    \"Error Domain=ITunesConnectionAuthenticationErrorDomain Code=-26000 \\\"The server returned an invalid response. This may indicate that a network proxy is interfering with communication, or that Apple servers are having issues. Please try your request again later.\\\" UserInfo={NSLocalizedRecoverySuggestion=The server returned an invalid response. This may indicate that a network proxy is interfering with communication, or that Apple servers are having issues. Please try your request again later., NSLocalizedDescription=The server returned an invalid response. This may indicate that a network proxy is interfering with communication, or that Apple servers are having issues. Please try your request again later., NSLocalizedFailureReason=App Store operation failed.}\"\n)}"
)`

const undefinedSoftwareType = `{
    EnableJWTForAllCalls = 0;
    ErrorCode = 1194;
    ErrorMessage = "Unable to determine app platform for 'Undefined' software type. (1194)";
    Errors =     (
        "Unable to determine app platform for 'Undefined' software type. (1194)"
    );
    RestartClient = 0;
    ShouldUseRESTAPIs = 0;
    Success = 0;
}
Non-localized server string received: 'Unable to determine app platform for 'Undefined' software type.'.
Non-localized server string received: 'Unable to determine app platform for 'Undefined' software type. (1194)'.`

func Test_getKeyPath(t *testing.T) {
	tmpPath, err := pathutil.NormalizedOSTempDirPath("testing")
	if err != nil {
		t.Fatal(err)
	}

	tmpKeyPaths := []string{
		filepath.Join(tmpPath, "test2"),
		filepath.Join(tmpPath, "test1"),
		filepath.Join(tmpPath, "test3"),
	}

	tmpKeyPaths2 := []string{
		filepath.Join(tmpPath, "test22"),
		filepath.Join(tmpPath, "test12"),
		filepath.Join(tmpPath, "test32"),
	}

	if err := os.MkdirAll(tmpKeyPaths2[2], 0777); err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(filepath.Join(tmpKeyPaths2[2], "AuthKey_MyGreatID.p8"), []byte("content"), 0777); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name            string
		keyID           string
		keyPaths        []string
		want            string
		wantErr         bool
		wantOsExistsErr bool
	}{
		{name: "check nonexisting", keyID: "MyID", keyPaths: tmpKeyPaths, want: filepath.Join(tmpKeyPaths[0], "AuthKey_MyID.p8"), wantErr: false, wantOsExistsErr: false},
		{name: "check existing", keyID: "MyGreatID", keyPaths: tmpKeyPaths2, want: filepath.Join(tmpKeyPaths2[2], "AuthKey_MyGreatID.p8"), wantErr: false, wantOsExistsErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getKeyPath(tt.keyID, tt.keyPaths)
			if tt.wantOsExistsErr && (err != os.ErrExist) {
				t.Errorf("not os.ErrExists")
				return
			}
			if !tt.wantOsExistsErr && (err != nil) != tt.wantErr {
				t.Errorf("getKeyPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getKeyPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_uploadSuccessful(t *testing.T) {
	uploader := createUploaderWithSuccess()

	result, err := uploadWithRetry(uploader)

	assert.NoError(t, err)
	assert.Equal(t, result, "success")
	uploader.AssertNumberOfCalls(t, "upload", 1)
}

func Test_uploadFailsWithUnknownError(t *testing.T) {
	uploader := createUploaderWithUnknownError()

	_, err := uploadWithRetry(uploader)

	assert.Error(t, err)
	uploader.AssertNumberOfCalls(t, "upload", 1)
}

func Test_uploadRetriesOnUnableToDetermine(t *testing.T) {
	uploader := createUploaderWithUnableToDetermineError()

	_, err := uploadWithRetry(uploader, retry.Delay(0))

	assert.Error(t, err)
	uploader.AssertNumberOfCalls(t, "upload", 10)
}

func Test_uploadRetriesOnTransporterService(t *testing.T) {
	uploader := createUploaderWithTransporterService()

	_, err := uploadWithRetry(uploader, retry.Delay(0))

	assert.Error(t, err)
	uploader.AssertNumberOfCalls(t, "upload", 10)
}

func Test_uploadRetriesOnInvalidResponse(t *testing.T) {
	uploader := createUploaderWithInvalidResponse()

	_, err := uploadWithRetry(uploader, retry.Delay(0))

	assert.Error(t, err)
	uploader.AssertNumberOfCalls(t, "upload", 10)
}

func Test_uploadRecoversAfterErrorOnValidResponse(t *testing.T) {
	uploader := createUploaderWithFailingAndRecoveringResponse()

	result, err := uploadWithRetry(uploader, retry.Delay(0))

	assert.NoError(t, err)
	assert.Equal(t, result, "success")
	uploader.AssertNumberOfCalls(t, "upload", 4)
}

func Test_uploadRecoversAfterUndefinedSoftwareType(t *testing.T) {
	uploader := createUploaderWithUndefinedSoftwareType()

	result, err := uploadWithRetry(uploader, retry.Delay(0))

	assert.NoError(t, err)
	assert.Equal(t, result, "success")
	uploader.AssertNumberOfCalls(t, "upload", 2)
}

func createUploaderWithUnknownError() (uploader *mockUploader) {
	uploader = new(mockUploader)
	uploader.On("upload").Return("", "unknown-error", errors.New("test-error"))
	return
}

func createUploaderWithUnableToDetermineError() (uploader *mockUploader) {
	uploader = new(mockUploader)
	uploader.On("upload").Return("", unableToDetermine, errors.New("test-error"))
	return
}

func createUploaderWithTransporterService() (uploader *mockUploader) {
	uploader = new(mockUploader)
	uploader.On("upload").Return("", transporterService, errors.New("test-error"))
	return
}

func createUploaderWithUndefinedSoftwareType() (uploader *mockUploader) {
	uploader = new(mockUploader)
	uploader.On("upload").Return("", undefinedSoftwareType, errors.New("test-error")).Once()
	uploader.On("upload").Return("success", "", nil)
	return
}

func createUploaderWithInvalidResponse() (uploader *mockUploader) {
	uploader = new(mockUploader)
	uploader.On("upload").Return("", invalidResponse, errors.New("test-error"))
	return
}

func createUploaderWithFailingAndRecoveringResponse() (uploader *mockUploader) {
	uploader = new(mockUploader)
	uploader.On("upload").Return("", unableToDetermine, errors.New("test-error")).Once()
	uploader.On("upload").Return("", transporterService, errors.New("test-error")).Once()
	uploader.On("upload").Return("", invalidResponse, errors.New("test-error")).Once()
	uploader.On("upload").Return("success", "", nil)
	return
}

func createUploaderWithSuccess() (uploader *mockUploader) {
	uploader = new(mockUploader)
	uploader.On("upload").Return("success", "", nil)
	return
}
