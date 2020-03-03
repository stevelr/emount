// +build !darwin

package main

// linux unmount that uses unmount flag MNT_DETACH to support
// lazy unmounting in case any files are still in use.
// If the syscall fails, attempts to call "fusermount -u path", which
// is installed with suid bit and seems to have a higher success rate.

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// #include <sys/mount.h>
import "C"

// unmountVol unmounts the volume. If it is still in use, returns an error.
// Do not use MNT_FORCE as it may lead to data corruption or loss.
func unmountVol(path string) error {

	// try the standard syscall first
	// This seems to fail often, with error "operation not permitted"
	e1 := syscall.Unmount(path, C.MNT_DETACH)
	if e1 != nil {
		// however, fusermount is installed with suid bit
		// try with fusermount -u
		env := os.Environ()[:]
		fusermount, err := exec.LookPath("fusermount")
		if err == nil {
			e2 := runCommand([]string{fusermount, "-u", path}, env)
			if e2 == nil {
				return nil
			}
			// both failed. what to return?
			return fmt.Errorf("unmount failed [1]:%v [2]:%v", e1, e2)
		}
		return e1
	}
	return nil
}
