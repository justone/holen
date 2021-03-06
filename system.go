package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/justone/archiver"
	"github.com/pkg/errors"
)

type System interface {
	OS() string
	Arch() string
	UID() int
	GID() int
	FileExists(string) bool
	MakeExecutable(string) error
	Stderrf(string, ...interface{})
	Stdoutf(string, ...interface{})
	UnpackArchive(string, string) error
	Getenv(string) string
	DataPath() (string, error)
}

type DefaultSystem struct{}

func (ds DefaultSystem) OS() string {
	return runtime.GOOS
}

func (ds DefaultSystem) Arch() string {
	return runtime.GOARCH
}

func (ds DefaultSystem) UID() int {
	return os.Getuid()
}

func (ds DefaultSystem) GID() int {
	return os.Getgid()
}

func (ds DefaultSystem) FileExists(localPath string) bool {
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		return false
	}
	return true
}

func (ds DefaultSystem) MakeExecutable(localPath string) error {
	return os.Chmod(localPath, 0755)
}

func (ds DefaultSystem) Stderrf(message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, message, args...)
}

func (ds DefaultSystem) Stdoutf(message string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, message, args...)
}

func (ds DefaultSystem) UnpackArchive(archive, destPath string) error {
	unpackSuccess := false
	for _, format := range archiver.SupportedFormats {
		if format.Match(archive) {
			err := format.Open(archive, destPath)
			if err != nil {
				return errors.Wrap(err, "error unpacking archive")
			}
			unpackSuccess = true
			break
		}
	}

	if !unpackSuccess {
		return fmt.Errorf("archive format not supported")
	}

	return nil
}

func (ds DefaultSystem) Getenv(key string) string {
	return os.Getenv(key)
}

func (ds DefaultSystem) DataPath() (string, error) {
	var holenPath string
	if xdgDataHome := ds.Getenv("XDG_DATA_HOME"); len(xdgDataHome) > 0 {
		holenPath = filepath.Join(xdgDataHome, "holen")
	} else {
		var home string
		if home = ds.Getenv("HOME"); len(home) == 0 {
			return "", fmt.Errorf("$HOME environment variable not found")
		}
		holenPath = filepath.Join(home, ".local", "share", "holen")
	}
	os.MkdirAll(holenPath, 0755)

	return holenPath, nil
}

type Logger interface {
	Debugf(string, ...interface{})
	Infof(string, ...interface{})
	Warnf(string, ...interface{})
}

type LogrusLogger struct{}

func (ll LogrusLogger) Debugf(str string, args ...interface{}) {
	logrus.Debugf(str, args...)
}

func (ll LogrusLogger) Infof(str string, args ...interface{}) {
	logrus.Infof(str, args...)
}

func (ll LogrusLogger) Warnf(str string, args ...interface{}) {
	logrus.Warnf(str, args...)
}

type Downloader interface {
	DownloadFile(string, string) error
	PullDockerImage(string) error
}

type DefaultDownloader struct {
	Logger
	Runner
}

func (dd DefaultDownloader) DownloadFile(url, path string) error {
	dd.Debugf("Downloading file from %s to %s", url, path)

	res, err := http.Get(url)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("unable to download %s", url))
	}

	out, err := os.Create(path)
	defer out.Close()
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("unable to create file %s", path))
	}

	_, err = io.Copy(out, res.Body)
	if err != nil {
		return errors.Wrap(err, "unable to save downloaded file")
	}
	res.Body.Close()

	return nil
}

func (dd DefaultDownloader) PullDockerImage(image string) error {
	return dd.RunCommand("docker", []string{"pull", image})
}

type Runner interface {
	RunCommand(string, []string) error
	ExecCommand(string, []string) error
	ExecCommandWithEnv(string, []string, []string) error
	CheckCommand(string, []string) bool
	CommandOutputToFile(string, []string, string) error
}

type DefaultRunner struct {
	Logger
}

func (dr DefaultRunner) CheckCommand(command string, args []string) bool {
	dr.Debugf("Checking command %s with args %v", command, args)

	return exec.Command(command, args...).Run() == nil
}

func (dr DefaultRunner) RunCommand(command string, args []string) error {

	cmd := exec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (dr DefaultRunner) ExecCommand(command string, args []string) error {
	return dr.ExecCommandWithEnv(command, args, make([]string, 0))
}

func (dr DefaultRunner) ExecCommandWithEnv(command string, args []string, extraEnv []string) error {
	dr.Debugf("Running command %s with args %v and extra env %v", command, args, extraEnv)

	if runtime.GOOS == "windows" {
		if err := dr.RunCommand(command, args); err != nil {
			if eerr, ok := err.(*exec.ExitError); ok {
				os.Exit(eerr.Sys().(syscall.WaitStatus).ExitStatus())
			}
		}

		os.Exit(0)
		return nil
	} else {
		// adapted from https://gobyexample.com/execing-processes
		fullPath, err := exec.LookPath(command)
		if err != nil {
			return err
		}
		return syscall.Exec(fullPath, append([]string{filepath.Base(command)}, args...), append(os.Environ(), extraEnv...))
		// end adapted from
	}
}

func (dr DefaultRunner) CommandOutputToFile(command string, args []string, outputFile string) error {

	file, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	cmd := exec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = file
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
