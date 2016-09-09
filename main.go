package main

import (
	"fmt"
	"os"
	"path"

	"github.com/Sirupsen/logrus"
	flags "github.com/jessevdk/go-flags"
	"github.com/kr/pretty"
)

type GlobalOptions struct {
	Quiet   func() `short:"q" long:"quiet" description:"Show as little information as possible."`
	Verbose func() `short:"v" long:"verbose" description:"Show verbose debug information."`
	LogJSON func() `short:"j" long:"log-json" description:"Log in JSON format."`
}

var globalOptions GlobalOptions
var parser = flags.NewParser(&globalOptions, flags.Default)
var originalArgs []string

func main() {
	basename := path.Base(os.Args[0])

	if basename == "holen" || basename == "hln" {

		// configure logging
		logrus.SetLevel(logrus.InfoLevel)
		logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

		// options to change log level
		globalOptions.Quiet = func() {
			logrus.SetLevel(logrus.WarnLevel)
		}
		globalOptions.Verbose = func() {
			logrus.SetLevel(logrus.DebugLevel)
		}
		globalOptions.LogJSON = func() {
			logrus.SetFormatter(&logrus.JSONFormatter{})
		}
		originalArgs = os.Args
		if _, err := parser.Parse(); err != nil {
			os.Exit(1)
		}
	} else {

		s, err := NewSystem()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Printf("%# v\n", pretty.Formatter(s))

		RunUtility(s, basename)
	}
}
