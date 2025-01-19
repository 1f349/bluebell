package hook

import (
	"github.com/cyphar/filepath-securejoin"
	"os"
	"os/exec"
	"path/filepath"
)

type Hook struct {
	hookDir  string
	sitesDir string
}

func New(hookDir string, sitesDir string) *Hook {
	return &Hook{hookDir: hookDir, sitesDir: sitesDir}
}

func (h *Hook) Run(site, branch string) error {
	if h.sitesDir == "" || h.hookDir == "" {
		return nil
	}

	sitePath, err := securejoin.SecureJoin(h.sitesDir, site+"/work@"+branch)
	if err != nil {
		return err
	}
	scriptPath, err := securejoin.SecureJoin(h.hookDir, site)
	if err != nil {
		return err
	}

	// check if the script exists before failing to execute
	_, err = os.Stat(scriptPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	cmd := exec.Cmd{
		Path: scriptPath,
		Args: []string{filepath.Base(scriptPath)},
		Dir:  sitePath,
	}
	return cmd.Run()
}
