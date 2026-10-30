This is a putty and winscp like app.. combined. Written in golang, using wales GUI.

The top two left/right panes are local and remote file lists with all the appropriate buttons and folder paths.
The SSH CLI is below it, with a resizable vertical bar.

The remote file panes and SSH CLI are the same connection. If the user types su, they are prompted for a password
and (if successful) can move files around as root. The exit command drops back to the user. any cd commands are reflected
in the remote pane. If a user does something in the file area, the commands are listed in the CLI.
