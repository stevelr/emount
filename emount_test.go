package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"testing"
)

const testDirPrefix = "gotest_emount_"

// gocryptConfig is config file - we check for this to confirm vol was created
const gocryptConfig = "/gocryptfs.conf"

func TestInitCryptVolEmptyPw(t *testing.T) {

	// empty password attempts to prompt for new password
	err := initCryptVol("", "")
	// expect "Input error: inappropriate ioctl for device"
	if err == nil {
		t.Errorf("expected prompt for password, err == nil\n")
	}
	if !strings.Contains(err.Error(), "Input error") {
		t.Errorf("unexpected error for empty password: %v", err)
	}
}

func TestInitCryptVol(t *testing.T) {

	// generate random password of about 31-32 hex chars
	password := fmt.Sprintf("%x%x", rand.Int63(), rand.Int63())
	os.Setenv("EMOUNT_PASSWORD", password)

	folder, err := ioutil.TempDir("", testDirPrefix)
	okf(t, err)
	defer func() {
		_ = os.RemoveAll(folder)
	}()
	err = initCryptVol(folder, "")
	okf(t, err)

	fs, err := os.Stat(folder)
	okf(t, err)
	if fs.Mode()&0777 != 0700 {
		t.Errorf("mode mismatch expected %o got %o", 0700, fs.Mode()&0777)
	}

	fs, err = os.Stat(folder + gocryptConfig)
	okf(t, err)
	assert(t, fs.Size() > 0, "crypt config", fs.Size())
}

type testFileInfo struct {
	dirPath  string
	name     string
	contents []byte
	perm     os.FileMode
}

func getTestFileInfo() []testFileInfo {
	return []testFileInfo{
		// the names and file contents in this array are opaque to the test,
		// since we iterate through the array, however there is an assumption
		// in TestDecryptAndRun that the [0]th item is a file in the root directory.
		{
			dirPath:  "",
			name:     "/abc.txt",
			contents: []byte("hello_world"),
			perm:     0600,
		},
		{
			dirPath:  "/sub",
			name:     "/xyz.txt",
			contents: []byte("1234"),
			perm:     0640,
		},
		{
			dirPath:  "/bin",
			name:     "/prog.exe",
			contents: []byte{0, 1, 2, 3},
			perm:     0755,
		},
	}
}

func createTestData() (string, error) {

	folder, err := ioutil.TempDir("", testDirPrefix)
	if err != nil {
		return "", err
	}

	for _, tf := range getTestFileInfo() {
		if tf.dirPath != "" {
			if err = os.Mkdir(folder+tf.dirPath, 0700); err != nil {
				return "", fmt.Errorf("creating tmp file %v", err)
			}
		}
		if err = ioutil.WriteFile(folder+tf.name, tf.contents, tf.perm); err != nil {
			return "", err
		}
	}
	return folder, nil
}

func confirmTestData(t *testing.T, path string) error {
	t.Helper()

	fs, err := os.Stat(path)
	ok(t, err)
	assertf(t, fs.IsDir(), "tet folder is dir", fs.IsDir())

	for _, tf := range getTestFileInfo() {
		fs, err = os.Stat(path + tf.name)
		ok(t, err)
		mode := fs.Mode() & 0777
		assert(t, tf.perm == mode,
			fmt.Sprintf("mode mismatch %s expected %o", tf.name, tf.perm), mode)
		data, err := ioutil.ReadFile(path + tf.name)
		if err != nil {
			t.Errorf("reading file %s: %v", tf.name, err)
		}
		if !bytes.Equal(data, tf.contents) {
			t.Logf("expected: %v\nactual:%v\n", tf.contents, data)
			t.Errorf("file contents mismatch")
		}
	}
	return nil
}

func TestInitCryptVolCopy(t *testing.T) {

	// generate random password of about 31-32 hex chars
	password := fmt.Sprintf("%x%x", rand.Int63(), rand.Int63())
	os.Setenv("EMOUNT_PASSWORD", password)

	newCrypt, err := ioutil.TempDir("", testDirPrefix)
	okf(t, err)
	defer func() {
		_ = os.RemoveAll(newCrypt)
	}()

	// create test data
	dataFolder, err := createTestData()
	okf(t, err)
	defer func() {
		_ = os.RemoveAll(dataFolder)
	}()

	err = initCryptVol(newCrypt, dataFolder)
	okf(t, err)

	// mount volume and confirm
	// when second param is empty it creates temp mount, so we need to remove later
	mp, err := mountCrypt(newCrypt, "", password)
	okf(t, err)
	err = confirmTestData(t, mp)
	ok(t, err)
	unmountVol(mp)
	ok(t, os.RemoveAll(mp))
}

func TestDecryptAndRun(t *testing.T) {

	sav := flag.CommandLine
	defer func() {
		flag.CommandLine = sav
	}()

	// generate random password of about 31-32 hex chars
	password := fmt.Sprintf("%x%x", rand.Int63(), rand.Int63())
	os.Setenv("EMOUNT_PASSWORD", password)

	newCrypt, err := ioutil.TempDir("", testDirPrefix)
	okf(t, err)
	defer func() {
		_ = os.RemoveAll(newCrypt)
	}()

	// create test data
	dataFolder, err := createTestData()
	okf(t, err)
	defer func() {
		_ = os.RemoveAll(dataFolder)
	}()

	// init & copy from test data
	err = initCryptVol(newCrypt, dataFolder)
	okf(t, err)

	mountPoint, err := ioutil.TempDir("", testDirPrefix)
	okf(t, err)
	defer func() {
		_ = os.RemoveAll(mountPoint)
	}()

	destDir, err := ioutil.TempDir("", testDirPrefix)
	okf(t, err)
	defer func() {
		_ = os.RemoveAll(destDir)
	}()

	// use /bin/cp to copy file. Need to use abs path because path lookup is in parseArgs
	// (path lookup was already tested with bash in TestParseArgs)
	// use the 0th file in the test array, a file at the volume root
	// so we don't need to do mkdir
	tf := getTestFileInfo()[0]
	assert(t, tf.dirPath == "", "test file[0] should be at root", tf.dirPath)

	flag.CommandLine = flag.NewFlagSet("prog", flag.ExitOnError)
	os.Args = []string{"prog", "-r", newCrypt, "-m", mountPoint,
		"/bin/cp", mountPoint + tf.name, destDir + tf.name}
	main()

	// confirm that copy occurred
	data, err := ioutil.ReadFile(destDir + tf.name)
	ok(t, err)
	assert(t, bytes.Equal(data, tf.contents), "cp", data)

	// confirm that it's been unmounted
	err = checkEmptyDir(mountPoint)
	ok(t, err)
}

func TestInvalidPath(t *testing.T) {
	_, err := mountCrypt("/usr/bin", "", "abc")
	if err == nil || !strings.Contains(err.Error(), "no such file or directory") {
		t.Errorf("expect 'no such file or directory', got %v", err)
	}
}

func TestInvalidPassword(t *testing.T) {

	// generate random password of about 31-32 hex chars
	password := fmt.Sprintf("%x%x", rand.Int63(), rand.Int63())
	os.Setenv("EMOUNT_PASSWORD", password)

	newCrypt, err := ioutil.TempDir("", testDirPrefix)
	okf(t, err)
	defer func() {
		_ = os.RemoveAll(newCrypt)
	}()
	err = initCryptVol(newCrypt, "")
	ok(t, err)

	_, err = mountCrypt(newCrypt, "", "abc")
	if err == nil || !strings.Contains(err.Error(), "Invalid password") {
		t.Errorf("expected Invalid password error, got: %v", err)
	}
}

func TestUnmountWarning(t *testing.T) {
	fmt.Printf("Ignore the following warning about unmount failure:  ")
	printUnmountWarning("")
}

func TestCheckEmptyDir(t *testing.T) {

	path, err := ioutil.TempDir("", testDirPrefix)
	okf(t, err)
	defer func() {
		_ = os.RemoveAll(path)
	}()

	err = checkEmptyDir(path)
	ok(t, err)

	err = ioutil.WriteFile(path+"/abc.txt", []byte("hello"), 0600)
	okf(t, err)
	err = checkEmptyDir(path)
	if err == nil {
		t.Errorf("expected checkEmptyDir to fail with file")
	}

	// check invalid path
	err = checkEmptyDir(path)
	if err == nil {
		t.Errorf("expected error for invalid path")
	}

	// check file but not dir
	tmpf, err := ioutil.TempFile("", testDirPrefix)
	okf(t, err)

	err = checkEmptyDir(tmpf.Name())
	if err == nil {
		t.Errorf("checkEmptyDir should have failed with file param")
	}

	ok(t, tmpf.Close())
	ok(t, os.Remove(tmpf.Name()))
}

func TestCheckFolder(t *testing.T) {

	cf := checkFolder("")
	if cf != invalidPath {
		t.Errorf("empty path expected invalidPath, got %v", cf)
	}

	cf = checkFolder(os.TempDir())
	if cf != isDir {
		t.Errorf("expected isDir for %s, got %v", os.TempDir(), cf)
	}

	tmpf, err := ioutil.TempFile("", testDirPrefix)
	okf(t, err)

	fname := tmpf.Name()
	cf = checkFolder(fname)
	if cf != isNotDir {
		t.Errorf("expected notDir for temp file, got %v", cf)
	}

	// test invalid path
	ok(t, tmpf.Close())
	ok(t, os.Remove(fname))

	cf = checkFolder(fname)
	if cf != invalidPath {
		t.Errorf("invalid fname, got %v", cf)
	}
}

func TestParseArgs(t *testing.T) {

	// we overwrite CommandLine a few times so we'll restore it in case
	// the test framework depends on it
	sav := flag.CommandLine
	defer func() {
		flag.CommandLine = sav
	}()

	dir, err := ioutil.TempDir("", testDirPrefix)
	okf(t, err)
	defer func() {
		_ = os.RemoveAll(dir)
	}()

	// -i dir valid
	t.Logf("-i dir (valid)")
	os.Args = []string{"prog", "-i", dir}
	opt := &options{}
	flag.CommandLine = flag.NewFlagSet("prog", flag.ExitOnError)
	err = parseArgs(opt)
	ok(t, err)
	if opt.init != dir {
		t.Errorf("init flag")
	}

	file, err := ioutil.TempFile("", testDirPrefix)
	okf(t, err)
	fname := file.Name()
	defer func() {
		_ = file.Close()
		_ = os.Remove(fname)
	}()

	// -i dir (invalid dir)
	t.Logf("-i dir (invalid dir)")
	os.Args = []string{"prog", "-i", fname}
	opt = &options{}
	flag.CommandLine = flag.NewFlagSet("prog", flag.ExitOnError)
	err = parseArgs(opt)
	if err == nil {
		t.Errorf("expected arg fail with invalid dir")
	}

	// -i dir non-existent
	t.Logf("-i dir (non-existent)")
	os.Args = []string{"prog", "-i", dir + "/a/b/c"}
	opt = &options{}
	flag.CommandLine = flag.NewFlagSet("prog", flag.ExitOnError)
	err = parseArgs(opt)
	ok(t, err)
	_ = os.RemoveAll(dir + "/a")

	// -i dir -r dir (error: both)
	t.Logf("-i dir -r dir")
	os.Args = []string{"prog", "-i", "folder", "-r", "other"}
	opt = &options{}
	flag.CommandLine = flag.NewFlagSet("prog", flag.ExitOnError)
	err = parseArgs(opt)
	if err == nil {
		t.Errorf("expected error for both -i and -r")
	}

	// -i dir non-empty
	t.Logf("-i dir (non-empty)")
	os.Args = []string{"prog", "-i", dir}
	err = ioutil.WriteFile(dir+"/file.x", []byte{1}, 0600)
	okf(t, err)
	opt = &options{}
	flag.CommandLine = flag.NewFlagSet("prog", flag.ExitOnError)
	err = parseArgs(opt)
	if err == nil {
		t.Errorf("init non-empty dir expected error")
	}
	_ = os.Remove(dir + "/file.x")

	// -r dir
	t.Logf("-r dir bash echo hello")
	os.Args = []string{"prog", "-r", dir, "bash", "echo", "hello"}
	opt = &options{}
	flag.CommandLine = flag.NewFlagSet("prog", flag.ExitOnError)
	err = parseArgs(opt)
	ok(t, err)
	if opt.run != dir {
		t.Errorf("run arg missing")
	}
	// this also tests the PATH lookup
	// string.Contains should work for any of /bin/bash /usr/bin/bash, /usr/local/bin/bash
	if !strings.Contains(opt.runCmd[0], "/bin/bash") {
		t.Errorf("opt path search failed")
	}
	if opt.runCmd[1] != "echo" || opt.runCmd[2] != "hello" || len(opt.runCmd) != 3 {
		t.Errorf("runCmd not packed correctly: %v", opt.runCmd)
	}

	// -r dir -m mountpoint
	t.Logf("-r dir -m mountpoint")
	mp, err := ioutil.TempDir("", testDirPrefix)
	okf(t, err)
	defer func() {
		_ = os.RemoveAll(mp)
	}()
	os.Args = []string{"prog", "-r", dir, "-m", mp, "bash", "echo", "hello"}
	opt = &options{}
	flag.CommandLine = flag.NewFlagSet("prog", flag.ExitOnError)
	err = parseArgs(opt)
	okf(t, err)
	if opt.mountPoint != mp {
		t.Errorf("mountpoint arg")
	}

	// -r dir != -m mp
	t.Logf("-r dir != mp")
	os.Args = []string{"prog", "-r", dir, "-m", dir, "bash", "echo", "hello"}
	opt = &options{}
	flag.CommandLine = flag.NewFlagSet("prog", flag.ExitOnError)
	err = parseArgs(opt)
	if err == nil {
		t.Errorf("expected err for run dir == mountpoint")
	}
	// -r dir no cmd
	t.Logf("-r dir no cmd")
	os.Args = []string{"prog", "-r", dir}
	opt = &options{}
	flag.CommandLine = flag.NewFlagSet("prog", flag.ExitOnError)
	err = parseArgs(opt)
	if err == nil {
		t.Errorf("expected err for run with no command")
	}

	// -r dir cmd non-executable
	t.Logf("-r dir cmd non-executable")
	os.Args = []string{"prog", "-r", dir, mp}
	opt = &options{}
	flag.CommandLine = flag.NewFlagSet("prog", flag.ExitOnError)
	err = parseArgs(opt)
	if err == nil {
		t.Errorf("expected err for run with non-executable cmd")
	}
}

func TestShowUsage(t *testing.T) {
	t.Logf("Ignore the following usage statement: ")
	showUsage()

}

func TestUsageError(t *testing.T) {
	e := newUsageErr("hello")

	assert(t, e.Error() == "hello", "Error()", e.Error())

	assert(t, isUsageErr(e), "usage", e)
}
