package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitSourceUrl(t *testing.T) {
	assert := assert.New(t)

	var testCases = []struct {
		spec, url string
	}{
		{
			"test/repo",
			"https://github.com/test/repo.git",
		},
		{
			"/absolute/path/repo.git",
			"/absolute/path/repo.git",
		},
		{
			"bitbucket.org/test/repo",
			"https://bitbucket.org/test/repo.git",
		},
	}

	for _, test := range testCases {
		system := NewMemSystem()
		logger := &MemLogger{}
		runner := &MemRunner{}
		gs := GitSource{
			system,
			logger,
			runner,
			"test",
			test.spec,
		}

		assert.Equal(gs.Name(), "test")
		assert.Equal(gs.Spec(), test.spec)
		assert.Equal(gs.Info(), fmt.Sprintf("git source: %s", test.url))
	}
}

func TestGitSourceUpdate(t *testing.T) {
	assert := assert.New(t)

	baseDir := "/tmp"

	var testCases = []struct {
		mod func(*MemSystem)
		cmd string
	}{
		{
			func(ms *MemSystem) {
				return
			},
			fmt.Sprintf("git clone https://github.com/test/repo.git %s/test", baseDir),
		},
		{
			func(ms *MemSystem) {
				ms.Files[fmt.Sprintf("%s/test", baseDir)] = true
			},
			"git pull",
		},
	}

	for _, test := range testCases {
		system := NewMemSystem()
		logger := &MemLogger{}
		runner := &MemRunner{}
		gs := GitSource{
			system,
			logger,
			runner,
			"test",
			"test/repo",
		}

		test.mod(system)
		assert.Nil(gs.Update(baseDir))
		assert.Equal([]string{test.cmd}, runner.History)
	}
}

func TestGitSourceDelete(t *testing.T) {
	assert := assert.New(t)

	tempdir, _ := ioutil.TempDir("", "hash")
	defer os.RemoveAll(tempdir)

	gs := GitSource{
		NewMemSystem(),
		&MemLogger{},
		&MemRunner{},
		"test",
		"test/repo",
	}

	repoPath := filepath.Join(tempdir, "test")
	os.Mkdir(repoPath, 0755)
	assert.Nil(gs.Delete(tempdir))

	_, err := os.Stat(repoPath)
	assert.True(os.IsNotExist(err))
}