package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/bitrise-io/go-utils/pathutil"
)

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
