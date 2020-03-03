## Introduction

_emount_ decrypts a data volume for the duration of a command. Using _emount_ as a wrapper, a program that does not have encryption built in can maintain its data encrypted on disk ("at rest"), and only decrypted when it needs to be read or written by its managing program. To use this, you would replace the program in your path with a script that invokes _emount_, passing the program as parameters (examples provided).

_emount_ is a wrapper around [gocryptfs](https://nuetzlich.net/gocryptfs/) ([git](https://github.com/rfjakob/gocryptfs)), which encrypts data with AES-256-GCM and makes unencrypted data available via a fuse-mounted volume. It runs entirely in userspace

## Usage

```sh
    emount --init FOLDER [--from srcFolder]
```

Initialize a new encrypted volume at FOLDER. Either the path FOLDER must not exist or it must be an empty directory. If __srcFolder__ is specified, the volume is populated with a recursive copy from the source folder.

The user is prompted to enter a new password, and the password is rejected if it is too weak (according to the `minEntropy` setting in emount.go)

```sh
    emount --run FOLDER [--mount mountpoint] command args...
```

Run the command (with optional arguments), providing access to decrypted FOLDER mounted in a temporary location. When the command completes, the decrypted volume is unmounted. The 'command' term should be a program in your PATH or an absolute path to an executable.

The default mount point is a dynamically-created temporary folder (inside TMPDIR), owned by the calling user with permission mode 0700. The dynamic folder name is passed to the command executable through the environment variable `EMOUNT_FOLDER`.

The default mount point can be overridden by the --mount/-m flag.

- Tip: When using the `--mount MOUNTPOINT` parameter, it is up to you to ensure that the mountpoint (where unencrypted data will be mounted) has _appropriate_ access permissions, and, for example, is not unintentionally shared across a network.

For automation or to avoid interactive prompting for password, the encryption password can be provided via the environment variable `EMOUNT_PASSWORD`.

## Current status

Tested on Linux (Arch, 5.4+ kernel) and macOS Catalina.

Please try it out and let me know what you find. This program should be considered alpha status and should not be used for critical data unless you are making frequent backups and verifying the backups. Feedback is welcome.

## Setup and Examples

### Installation

- Prerequisites: Install [gocryptfs](https://github.com/rfjakob/gocryptfs), which in turn requires fuse (mac:[osxfuse](https://osxfuse.github.io/)). After first-time installation of fuse, a reboot is recommended to ensure drivers are loaded.
- Install emount

    ```sh
    go get github.com/stevelr/emount
    ```

- ensure emount (possibly in $HOME/go/bin or $GOPATH/go/bin) is in your PATH

### Quick demo

Here are a few commands you can do to test the installation:

```sh
# create a volume with -i/--init. You will be prompted to enter a new password
emount -i /tmp/emtest
# each time you run a program you will be prompted for the password again.
# For this demo, we'll run an interactive bash session.
emount -r /tmp/emtest bash

        # Now you are running a subshell, with the decrypted folder mounted at:
        cd $EMOUNT_FOLDER
        ls
        # It's empty, since we just created the vault.
        # Print the directory path so we can check it later
        pwd
        # create a simple file
        echo hello > abc.txt
        ls -al
        exit

# if you look in /tmp/emtest now, you will see two gocryptfs files, and one more
# with an obscure name with about 22 random characters. That's abc.txt with
# an encrypted file name.
# If you look in the folder that was mounted as $EMOUNT_FOLDER,
# it will be empty, since it's not mounted anymore.

# Quickly decrypt and view the contents of abc.txt. You will be prompted for password
emount -r /tmp/emtest bash -c "cat \$EMOUNT_FOLDER/abc.txt"
# The command above decrypts the vault, mounts the folder,
# runs the bash command, and unmounts, effectively "sealing" the vault again.
```

- If you want to avoid having to re-type the password, you can set it as an environment variable "EMOUNT_PASSWORD".

### Joplin and Joplin-desktop (linux/mac)

An example setup for [Joplin](https://joplinapp.org/), an awesome markdown editor, is documented in [example-joplin.md](./example-joplin.md)

## Notes

### Backups

You can back up the encrypted folder using standard backup tools. It is recommended to run backups only when the owning program is not running (e.g, the decrypted volume is not mounted).

### How secure/private is this?

The algorithms used, [AES-256-GCM](https://en.wikipedia.org/wiki/Galois/Counter_Mode) for encryption, and [HKDF-SHA256](https://en.wikipedia.org/wiki/HKDF) for key derivation, are well regarded by many cryptography experts. File names are also encrypted. gocryptfs has published results of a [2017 external security audit](https://defuse.ca/audits/gocryptfs.htm). There is a lot of good material on [gocryptfs's wiki](https://nuetzlich.net/gocryptfs/) including discussion of algorithms used and thread model.

- Unsolicited security tips:
  - One of the weakest links is the choice of password used. Even though the password is salted and hashed, weak passwords are easier to crack. Choose a good one! There is a minimum entropy parameter that can be set if you want to ensure that any passwords used have a reasonable level of crack-resistance.
  - If you do make use of the environment variable EMOUNT_PASSWORD to set the password, don't initialize the variable from a text file that sits on the same drive as the encrypted volume - that defeats the purpose of data encrypted on disk.

### Command-line vs gui apps

Because _emount_ prompts the user for password, it needs to have a way to prompt the user and accept typed response. I've only used this app on the command line. It would certainly be possible to make a gui version of this to show a dialog, and hook it into the .desktop apps for linux. I'm open to discussing PRs if somebody wants to work on it.

## Acknowledgements

- [gocryptfs](https://github.com/rfjakob/gocryptfs) The [documentation](https://nuetzlich.net/gocryptfs/) has a description of cryptographic algorithms used and other background.
- [zxcvbn](https://github.com/nbutton23/zxcvbn-go) password strength checking [algorithm](https://github.com/dropbox/zxcvbn) implemented in go
