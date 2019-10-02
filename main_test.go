package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
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
		fmt.Fprint(w, "the only valid content")
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
