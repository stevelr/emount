## Setup _emount_ for Joplin and Joplin-desktop (linux/mac)

(This file is part of the [emount](https://github.com/stevelr/emount) project)

Here's an example setup for [Joplin](https://joplinapp.org/), an awesome markdown editor that runs on linux, macos, windows, and mobile apps. Joplin already encrypts notes before transmitting them to a server, but they are not encrypted on disk on your local machine. The one-line script below uses _emount_ to add local file encryption to Joplin.

Follow the instructions below for "joplin", if you use the ncurses cli version, or "joplin-desktop" if you use the electron-based gui app. If you use both, do both, since they store data in different locations. 

### 1. one-time setup: move existing data to a new encrypted volume

  Shut down Joplin/Joplin-desktop before you do this, to ensure all current edits are saved.
  
  __Joplin__ (cli) stores runtime data in a single folder `$HOME/.config/joplin`. Encrypted data will be moved to joplin.enc in the same folder, and the original folder path will be the emount mountpoint. When emount+joplin are not running, the joplin data folder will be empty.

  ```sh
  cd $HOME/.config
  emount --init joplin.enc --from joplin
  # move the data to a backup location. You can delete the joplin.sav folder later,
  # after you've confirmed that the volume was created correctly, and that you remember the password.
  mv joplin joplin.sav
  mkdir joplin
  ```
  
  __Joplin-desktop__ (the electron-based gui app) requires an extra step because it stores runtime data in two folders, `$HOME/.config/Joplin` and `$HOME/.config/joplin-desktop`. We will put them both into a single encrypted volume, and use symlinks so the joplin-desktop app can find them.

  ```sh
  cd $HOME/.config
  mkdir joplin-desktop-data
  mv Joplin joplin-desktop joplin-desktop-data
  ln -s joplin-desktop-data/Joplin Joplin
  ln -s joplin-desktop-data/joplin-desktop joplin-desktop
  emount -i joplin-desktop-data.enc -f joplin-desktop-data
  # move the data to a backup location. You can delete the .sav folder later,
  # after you've confirmed that the volume was created correctly, and that you remember the password.
  mv joplin-desktop-data joplin-desktop-data.sav
  # create new empty mount-point
  mkdir joplin-desktop-data
  ```

### 2. Create one-line launch script

Instead of running joplin or joplin-desktop, you will now be running a one-line script that invokes _emount_, which mounts the decrypted volume, runs joplin/joplin-desktop, and, after the app exits, the decrypted volume is unmounted, leaving only encrypted data on disk.

  __joplin__
  Place the script in `$HOME/bin/joplin`

  ```sh
  emount --run $HOME/.config/joplin.enc \
        --mount $HOME/.config/joplin \
        /usr/bin/joplin
  ```

  __joplin-desktop (linux)__
  Place the script in `$HOME/bin/joplin-desktop`

  ```sh
  emount --run $HOME/.config/joplin-desktop-data.enc \
        --mount $HOME/.config/joplin-desktop-data \
         /usr/bin/joplin-desktop
  ```

  __joplin-desktop (macos)__
  Place the script in `$HOME/bin/joplin-desktop`

  ```sh
  emount --run $HOME/.config/joplin-desktop-data.enc \
       --mount $HOME/.config/joplin-desktop-data \
         /Applications/Joplin.app/Contents/MacOS/Joplin
  ```

Ensure the script is executable, and in your path before /usr/bin, so that the script always runs.

### Comments

- Tip: For the greatest safety against hackers, malware, and potential data loss, don't keep joplin or joplin-desktop running all the time. During the time it's running, unencrypted data is present on your machine in $HOME/.config/joplin-desktop (a private folder), and could be read by someone with access to your physical machine or if they can access your account over a network. Risk of exposure is minimized if you get into the habit of closing the app when you aren't using it.
