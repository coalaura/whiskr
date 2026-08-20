//go:build desktop && windows

package open

import (
	"fmt"
	"os/exec"
)

func OpenFile(name string) error {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", name)

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("opening %s: %w", name, err)
	}

	return nil
}
