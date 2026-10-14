//go:build !windows

package credstore

import (
	"fmt"
	"os"
	"path/filepath"
)

func enforceDirPermissions(dir string) error {
	return os.Chmod(dir, 0o700)
}

func enforceFilePermissions(path string) error {
	return os.Chmod(path, 0o600)
}

func checkVaultPermissions(vaultPath string) (ok bool, msg, hint string) {
	info, err := os.Stat(vaultPath)
	if err != nil {
		return false, err.Error(), ""
	}
	filePerm := info.Mode().Perm()
	dirPath := filepath.Dir(vaultPath)
	dirInfo, dErr := os.Stat(dirPath)
	if filePerm&0o077 != 0 {
		return false,
			fmt.Sprintf("credential vault %s has overly permissive mode %04o", vaultPath, filePerm),
			fmt.Sprintf("run: chmod 0600 %s", vaultPath)
	}
	if dErr == nil && dirInfo.Mode().Perm()&0o077 != 0 {
		return false,
			fmt.Sprintf("credential directory %s has overly permissive mode %04o", dirPath, dirInfo.Mode().Perm()),
			fmt.Sprintf("run: chmod 0700 %s", dirPath)
	}
	return true, fmt.Sprintf("credential vault %s permissions verified (0600)", vaultPath), ""
}
