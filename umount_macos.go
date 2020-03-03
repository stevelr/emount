// +build darwin

package main

// this file applies to macos only

import "syscall"

//  unmountVol unmounts the volume. If it is still in use, returns an error.
// on macos, C.MNT_DETACH is not supported, so need to pass 0 for flags
// Do not use MNT_FORCE as it may lead to data corruption or loss.
func unmountVol(path string) error {
	return syscall.Unmount(path, 0)
}
