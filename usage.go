package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

type usageErr struct {
	message string
}

func (e *usageErr) Error() string {
	return e.message
}

func newUsageErr(msg string) error {
	return &usageErr{
		message: msg,
	}
}

func isUsageErr(e error) bool {
	_, ok := e.(*usageErr)
	return ok
}

func showUsage() {

	// prog is the name of the program invoked
	//prog := path.Base(os.Args[0])

	usage := `Usage:
emount --init FOLDER [--from srcFolder]
  Initialize a new encrypted volume at FOLDER. Either the path FOLDER must not exist
  or it must be an empty directory. If srcFolder is specified, the volume is
  populated with a recursive copy from the source folder.

  The user is prompted to enter a new password, and the password is rejected
  if it is too weak (according to the minEntropy setting in emount.go)

emount --run FOLDER [--mount mountpoint] command args...
  Run the command (with optional arguments), providing access to decrypted FOLDER
  mounted in a temporary location. When the command completes, the decrypted volume
  is unmounted. The 'command' term should be a program in your PATH
  or an absolute path to an executable.

  The default mount point is a dynamically-created temporary folder (inside TMPDIR),
  owned by the calling user with permission mode 0700. The dynamic folder name
  is passed to the command executable through the environment variable EMOUNT_FOLDER.

  The default mount point can be overridden by the --mount/-m flag.
  
For automation or to avoid interactive prompting for password, the encryption
password can be provided via the environment variable EMOUNT_PASSWORD.
`
	fmt.Println(usage)
}

func parseArgs(opt *options) error {

	flag.StringVar(&opt.run, "run", "", "run command")
	flag.StringVar(&opt.run, "r", "", "run command (shorthand)")
	flag.StringVar(&opt.init, "init", "", "initialize a new folder")
	flag.StringVar(&opt.init, "i", "", "initialize a new folder (shorthand)")
	flag.StringVar(&opt.srcFolder, "from", "", "folder to copy from")
	flag.StringVar(&opt.srcFolder, "f", "", "folder to copy from (shorthand)")
	flag.StringVar(&opt.mountPoint, "mount", "",
		"mount point for decrypted content")
	flag.StringVar(&opt.mountPoint, "m", "",
		"mount point for decrypted content (shorthand)")
	flag.Parse()

	// exactly one of run or init
	if (opt.run != "" && opt.init != "") || (opt.run == "" && opt.init == "") {
		return newUsageErr(
			"One of the flags (--run/-r) or (--init/-i) must be specified.")
	}

	if opt.init != "" {
		cf := checkFolder(opt.init)
		if cf == isNotDir {
			return fmt.Errorf("invalid init path: not a directory %v", opt.init)
		}
		if cf == isDir {
			if err := checkEmptyDir(opt.init); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(opt.init, 0700); err != nil {
				return fmt.Errorf("creating init path %s: %v", opt.init, err)
			}
		}
		if opt.srcFolder != "" {
			if cf := checkFolder(opt.srcFolder); cf != isDir {
				return fmt.Errorf("init from srcFolder %s invalid", opt.srcFolder)
			}
		}
		if opt.mountPoint != "" {
			return fmt.Errorf("mountPoint arg is not used with init")
		}
	}

	// run command validation
	if opt.run != "" {

		if checkFolder(opt.run) != isDir {
			return fmt.Errorf("invalid run folder %s", opt.run)
		}
		if opt.srcFolder != "" {
			return fmt.Errorf("the -f srcFolder is not used with --run")
		}

		// if mount point specified, it should already exist and be empty
		if opt.mountPoint != "" {
			if err := checkEmptyDir(opt.mountPoint); err != nil {
				return fmt.Errorf("Mountpoint %s error: %v", opt.mountPoint, err)
			}
		}

		if opt.mountPoint == opt.run {
			return fmt.Errorf("mountPoint may not be same as run folder")
		}

		opt.runCmd = flag.Args()
		if len(opt.runCmd) == 0 {
			// check that command[0] is a valid binary, or in the PATH
			return newUsageErr("Command required for -run")
		}
		runExePath := opt.runCmd[0]
		reInfo, err := os.Stat(runExePath)
		if err != nil {
			// see if we can find it with path lookup
			foundPath, err := exec.LookPath(runExePath)
			if err == nil && foundPath != runExePath {
				opt.runCmd[0] = foundPath
				return nil
			}
			return fmt.Errorf("run program %s not found: %v", runExePath, err)
		}
		mode := reInfo.Mode()
		if !mode.IsRegular() || ((mode & 0111) == 0) {
			return fmt.Errorf(
				"run program %s permission error or not executable: %v",
				runExePath, err)
		}
	}
	return nil
}
