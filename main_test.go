package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/stretchr/testify/require"
)

func Test_xcodePath(t *testing.T) {
	t.Log("Xcode path test")
	{
		got, err := xcodePath()
		require.NoError(t, err)
		require.True(t, sliceutil.IsStringInSlice(got, []string{"/Applications/Xcode.app", "/Applications/Xcode-beta.app"}))
	}
}

func Test_getKeyID(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"random file test", "file:///my/path/to/file.json", "Bitrise"},
		{"key file test", "file:///my/path/to/AuthKey_MyID.p8", "MyID"},
		{"random url test", "https://www.bitrise.io/my/path/to/file.json", "Bitrise"},
		{"key url test", "https://www.bitrise.io/my/path/to/AuthKey_MyID.p8", "MyID"},
		{"random url test with params", "https://www.bitrise.io/my/path/to/file.json?test_param=.p8", "Bitrise"},
		{"key url test with params", "https://www.bitrise.io/my/path/to/AuthKey_MyID.p8?another_param=AuthKey", "MyID"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.url)
			if err != nil {
				t.Fatal()
			}
			if got := getKeyID(u); got != tt.want {
				t.Errorf("getKeyID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_copyOrDownloadFile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "the only valid content"); err != nil {
			t.Fatal(err)
		}
	}))
	defer ts.Close()

	tmpdir, err := pathutil.NormalizedOSTempDirPath("test")
	if err != nil {
		t.Fatal(err)
	}
	testFilePathInputForPath := filepath.Join(tmpdir, "testfile")
	testFilePathOutputForPath := filepath.Join(tmpdir, "testfile2")
	testFilePathOutputForURL := filepath.Join(tmpdir, "testfile3")

	if err := ioutil.WriteFile(testFilePathInputForPath, []byte("the only valid content"), 0777); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		url     string
		pth     string
		wantErr bool
	}{
		{"local url check", ts.URL, testFilePathOutputForURL, false},
		{"local file check", "file://" + testFilePathInputForPath, testFilePathOutputForPath, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.url)
			if err != nil {
				t.Fatal(err)
			}
			err = copyOrDownloadFile(u, tt.pth)
			if (err != nil) != tt.wantErr {
				t.Errorf("copyOrDownloadFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if content, err := ioutil.ReadFile(tt.pth); err != nil || string(content) != "the only valid content" {
				t.Fatal("error or invalid file", err, content)
			}
		})
	}
}

func Test_regexTest(t *testing.T) {
	const (
		samplelog = `Generated JWT: eyJhbGciOiJFUzI1NiIsImtpZCI6IjdBUDgzS1Y1NEIiLCJ0eXAiOiJKV1QifQ.eyJleHAiOjE1NzAxMDA4MzcsImlzcyI6IjY5YTZkZTdiLTczMjUtNDdlMy1lMDUzLTViOGM3YzExYTRkMSIsImF1ZCI6ImFwcHN0b3JlY29ubmVjdC12MSIsImlhdCI6MTU3MDA5OTYzN30.r9c7XKA4sUv-pmN55IgzxzcVmrO7RFl2VnfK9EpD7KrTr3wQQrAzlgskynYX7Eg8JeNZitnkrPpJNuOU-17d1A
No errors uploading '/Users/vagrant/deploy/Application Loader Test.ipa'
`
		expectation = `Generated JWT: [REDACTED]
No errors uploading '/Users/vagrant/deploy/Application Loader Test.ipa'
`
	)

	if matches := regexp.MustCompile(`(?i)Generated JWT: (.*)`).FindStringSubmatch(samplelog); len(matches) == 2 {
		if strings.Replace(samplelog, matches[1], "[REDACTED]", -1) != expectation {
			t.Fatal(expectation, matches[0], strings.Replace(samplelog, matches[1], "[REDACTED]", -1))
		}
	}

}

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
