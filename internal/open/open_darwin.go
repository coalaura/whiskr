//go:build desktop && darwin

package open

import "os/exec"

func OpenFile(name string) error {
	return exec.Command("open", "-t", name).Start()
}
