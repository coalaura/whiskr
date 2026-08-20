//go:build desktop && release

package main

import (
	"os"
	"path/filepath"
	rdebug "runtime/debug"

	"github.com/coalaura/whiskr/internal/open"
)

var (
	logFile *os.File
	logPath string
)

func SetupLogging() error {
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}

	cache = filepath.Join(cache, "whiskr")
	logPath = filepath.Join(cache, "whiskr.log")

	err = os.MkdirAll(cache, 0755)
	if err != nil {
		return err
	}

	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	log.SetTarget(logFile)

	return nil
}

func HandlePanic() {
	recovered := recover()
	if recovered != nil {
		log.Errorf("panic: %v\n%s\n", recovered, rdebug.Stack())
	}

	if logFile != nil {
		logFile.Close()
	}

	if recovered != nil && logPath != "" {
		open.OpenFile(logPath)
	}
}
