//go:build !desktop || !release

package main

func SetupLogging() error {
	return nil
}

func HandlePanic() {}
