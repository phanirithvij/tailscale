// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// SubCmdFlags space separated list of command line flags.
const SubCmdFlags = "SUB_CMD_FLAGS"

// setupTmpdir sets up a known golden layout, covering all allowed file/folder types in a nar
func setupTmpdir(t *testing.T) string {
	tmpdir := t.TempDir()
	pwd, _ := os.Getwd()
	os.Chdir(tmpdir)
	defer os.Chdir(pwd)
	os.MkdirAll("sub/dir", 0755)
	os.Symlink("brokenfile", "brokenlink")
	os.Symlink("sub/dir", "dirl")
	os.Create("sub/dir/file1")
	f, _ := os.Create("file2m")
	_ = f.Truncate(2 * 1024 * 1024)
	f.Close()
	os.Symlink("../file2m", "sub/goodlink")
	return tmpdir
}

func TestCallingMain(t *testing.T) {
	if os.Getenv(SubCmdFlags) != "" {
		// we are the subprocess, we run main which the parent captures
		args := strings.Split(os.Getenv(SubCmdFlags), " ")
		os.Args = append([]string{os.Args[0]}, args...)
		main()
		os.Exit(0)
	}

	dir := setupTmpdir(t)
	t.Run("sri", func(t *testing.T) {
		// obtained via `nix hash path /tmp/...` of the above test dir
		expected := "sha256-mSK98sSotWk8aILCQH37XeS0UsZtSI/lss2NoXuhvlg="
		cmd := runMain(t.Name(), []string{"--sri", dir})

		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err, cmd.ProcessState.ExitCode())
		}
		actual := string(out)
		actual = actual[:len(actual)-1]

		if expected != actual {
			t.Fatal("Hash value not matched", actual)
		}
	})

	t.Run("nar", func(t *testing.T) {
		// obtained via `nix-store --dump /tmp/... | sha256sum` of the above test dir
		expected := "9922bdf2c4a8b5693c6882c2407dfb5de4b452c66d488fe5b2cd8da17ba1be58"
		cmd := runMain(t.Name(), []string{dir})
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err, cmd.ProcessState.ExitCode())
		}
		h := sha256.New()
		h.Write(out)
		hash := fmt.Sprintf("%x", h.Sum(nil))
		if expected != hash {
			t.Fatal("sha256sum of nar not matched", hash, expected)
		}
	})
}

// runMain allows testing main() directly
// adapted from the source https://github.com/kohirens/tmplpress/blob/46e536e50eaf55ea60d66bf815c9a4d6f51d27b9/main_test.go#L86 licensed MIT
func runMain(testFunc string, args []string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run", testFunc)
	subEnvVar := SubCmdFlags + "=" + strings.Join(args, " ")
	cmd.Env = append(os.Environ(), subEnvVar)
	return cmd
}
