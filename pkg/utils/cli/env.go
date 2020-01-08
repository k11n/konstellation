package cli

import (
	"io/ioutil"
	"os"
	"path"
)

const (
	ROOT_KEY = "KONSTELLATION_ROOT"
)

func GetRootDir() string {
	// by default install to ~/.konstellation, but allow overrides via env
	dir := os.Getenv(ROOT_KEY)
	if dir != "" {
		return dir
	}

	home, err := os.UserHomeDir()
	if err == nil {
		return path.Join(home, ".konstellation")
	} else {
		// this shouldn't happen
		return "konstellation"
	}
}

func GetBinDir() string {
	return path.Join(GetRootDir(), "bin")
}

func TestSetRootTempdir() string {
	dir, err := ioutil.TempDir("", "kontest")
	if err != nil {
		panic("Could not get tempdir")
	}
	os.Setenv(ROOT_KEY, dir)
	return dir
}

func ClearRootEnv() {
	os.Unsetenv(ROOT_KEY)
}
