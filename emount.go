package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/otiai10/copy"
)

// options holds command-line options and configuration
type options struct {
	run        string   // run volume path
	init       string   // init volume path
	srcFolder  string   // folder to copy from during initialization
	mountPoint string   // path for mounting unencrypted data
	runCmd     []string // command to run that accesses unencrypted data
}

type dirCheckResponse int

const (
	invalidPath = dirCheckResponse(0)
	isDir       = dirCheckResponse(1)
	isNotDir    = dirCheckResponse(2)
)

const (
	// dirMode sets permissions for the tmp folder created for mountpoint.
	// Only used if -m option is not used.
	dirMode = 0700

	// minEntropy is the minimum password entropy (float)
	// This is a better metric for password strength than length
	// and number of symbols!
	// Adjust here to enforce security policy cryptographic controls.
	// Examples of pass phrases that are slightly above 24 include:
	//   "horse-table", "summurr" "ostrich/3", "factory8717"
	minEntropy = 24.0

	tmpFolderPattern = "emount_"
	envPasswordKey   = "EMOUNT_PASSWORD"
	envFolderKey     = "EMOUNT_FOLDER"
)

// initCryptVol initializes encrypted storage folder at path.
// @param path should be a path to a folder that will be created.
// User is prompted to enter a password and gocryptfs is used to initialize it.
func initCryptVol(path string, initFrom string) error {

	var err error
	encPass := os.Getenv(envPasswordKey)
	if encPass == "" {
		encPass, err = promptNewPassword("Enter encryption passphrase: ", minEntropy)
		if err != nil {
			return err
		}
	}
	if encPass == "" {
		return errors.New("Password may not be empty")
	}
	if err := os.MkdirAll(path, dirMode); err != nil {
		return err
	}

	var out bytes.Buffer
	cmd := exec.Command("gocryptfs", "-init", "-q", "--", path)
	cmd.Stdin = bytes.NewBufferString(encPass)
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("Initialization failed: %s (rc=%d)",
				string(out.Bytes()), ee.ProcessState.ExitCode())
		}
		return fmt.Errorf("Initialization failed: %v", err)
	}

	if initFrom != "" {
		if err := initialCopy(path, encPass, initFrom); err != nil {
			return fmt.Errorf("The vault was successfully created, but some "+
				"files were not copied into it: (%v). If you can fix these "+
				"errors, you may want to delete the crypt volume and try again.",
				err)
		}
	}

	return nil
}

// initialCopy performs copy into newly created crypt volume
// Mounts volume for the first time, performs recursive copy from another folder,
// then umounts it.
func initialCopy(cryptPath string, password string, initFrom string) error {

	// Most likely source of errors here is access to source files:
	// Mount is likely to succeed here because it was just created
	// and we know the password works. Similarly, we are using
	// an empty new temp folder for the destination, so we shouldn't
	// get permission errors during write, and there are no existing
	// files in the destination that might cause overwrite concerns.
	mountPoint, err := mountCrypt(cryptPath, "", password)
	if err != nil {
		return fmt.Errorf("failed to mount new volume: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(mountPoint)
	}()
	err = copy.Copy(initFrom, mountPoint)
	if err != nil {
		return err
	}
	err = unmountVol(mountPoint)
	if err != nil {
		printUnmountWarning(mountPoint)
	}
	return nil
}

// runCommand runs the command and waits for it to complete.
// - runCmd the command and args. The first array element must be an
//     absolute path to an executable or a program in the PATH
// - env the environment, an array of strings of the form name=value
//
// Stdin, Stdout, and Stderr are passed through to the command, so you should
// be able to pipe things in or out. If the comamnd is "/bin/bash", you get an
// interactive terminal, etc. The caller's environment including PATH, DISPLAY,
// etc. is also passed to the subcommand, with one addition
// for the EMOUNT_FOLDER.
func runCommand(runCmd []string, env []string) error {
	var err error

	cmd := exec.Cmd{
		Path:   runCmd[0],
		Args:   runCmd[:],
		Env:    env,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	err = cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("Command exited with error. [rc=%d]",
				ee.ProcessState.ExitCode())
		}
		return fmt.Errorf("Command error: %v", err)
	}
	return nil
}

func mountCrypt(cryptPath string, mountPoint string,
	encPass string) (string, error) {

	var cleanup bool
	var err error

	if mountPoint == "" {
		mountPoint, err = ioutil.TempDir("", tmpFolderPattern)
		//fmt.Printf("mountCrypt: created %s\n", mountPoint)
		if err != nil {
			return "", fmt.Errorf("Failed to create mount point: %v", err)
		}
		cleanup = true
	}
	defer func() {
		// if we created a temp folder and had to exit due to error,
		// remove the temp folder
		if cleanup {
			_ = os.RemoveAll(mountPoint)
		}
	}()

	var out bytes.Buffer
	cmd := exec.Command("gocryptfs", "-q", "--", cryptPath, mountPoint)
	cmd.Stdin = bytes.NewBufferString(encPass)
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc := ee.ProcessState.ExitCode()
			if rc == 12 {
				return "", fmt.Errorf("Invalid password")
			}
			return "", fmt.Errorf("Mount failed: %s (rc=%d)",
				string(out.Bytes()), rc)
		}
		return "", fmt.Errorf("Mount failed: %v", err)
	}
	//fmt.Println("mount succeeded!")
	cleanup = false
	return mountPoint, nil
}

func decryptAndRun(opt *options) error {

	mountPoint := opt.mountPoint

	var err error
	encPass := os.Getenv(envPasswordKey)
	if encPass == "" {
		encPass, err = terminalGetSecret("Enter encryption passphrase: ")
		if err != nil {
			return err
		}
	}
	if encPass == "" {
		return errors.New("Password may not be empty")
	}

	mountPoint, err = mountCrypt(opt.run, mountPoint, encPass)
	if err != nil {
		return fmt.Errorf("Failed to mount: %v", err)
	}

	// pass through caller's environment, with one additional var for folder
	env := append(os.Environ()[:],
		fmt.Sprintf("%s=%s", envFolderKey, mountPoint))

	if err = runCommand(opt.runCmd, env); err != nil {
		// print error but keep going
		fmt.Printf("Command execution error: %v\n", err)
	}
	//fmt.Printf("command completed, unmounting ...\n")

	// unmount
	err = unmountVol(mountPoint)
	if err != nil {
		printUnmountWarning(mountPoint)
		fmt.Printf("err=%v\n", err)
	} else {
		// The folder is unmounted. Attempt to remove the mount point if it was
		// just created
		if opt.mountPoint == "" {
			_ = os.Remove(mountPoint)
		}
	}
	return nil
}

func printUnmountWarning(path string) {
	fmt.Printf("WARNING: Unmount folder '%s' failed. Please ensure all files "+
		"on this volume are closed and try unmounting again. The program 'lsof' "+
		"may be useful for identifying open file handles. If necessary, "+
		"you may need to use the unmount -F flag to force unmounting.\n",
		path)
}

func main() {
	var opt options
	var err error
	flag.Usage = func() {
		showUsage()
	}
	flag.ErrHelp = newUsageErr("Syntax error")
	err = parseArgs(&opt)
	if err != nil {
		fmt.Printf("parseArgs returned %v", err)
		if isUsageErr(err) {
			fmt.Printf("ERROR: %s\n\n", err.Error())
			showUsage()
		} else {
			fmt.Printf("ERROR: %v\n", err)
		}
	} else {
		if opt.init != "" {
			if err = initCryptVol(opt.init, opt.srcFolder); err != nil {
				fmt.Printf("ERROR: %v\n", err)
			}
		}
		if opt.run != "" {
			if err = decryptAndRun(&opt); err != nil {
				fmt.Printf("ERROR: %v\n", err)
			}
		}
	}
}

// checkEmptyDir verifies the directory exists and is empty.
// Returns error if not.
func checkEmptyDir(path string) error {
	dh, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dh.Close()
	fs, err := dh.Stat()
	if err != nil {
		return err
	}
	if !fs.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	// to find out if it's empty, walking children should return EOF
	// before it finds any
	_, err = dh.Readdir(1)
	if err == nil {
		return fmt.Errorf("folder %s is not empty", path)
	}
	if err != io.EOF {
		return err // some other io error
	}
	// EOF error means nothing found
	return nil
}

// returns invalidPath, if no such path, isDir if dir exists, or isNotDir
func checkFolder(path string) dirCheckResponse {
	if path == "" {
		return invalidPath
	}
	fstat, err := os.Stat(path)
	if err != nil {
		return invalidPath
	}
	if !fstat.IsDir() {
		return isNotDir
	}
	return isDir
}
