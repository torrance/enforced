# Enforced

Enforced is a small daemon written in Go (golang) that forces file and folder attributes (eg. permissions, owner and group) to adhere to a given configuration.

## Configuration example

Folder configuration is defined in a yaml file according to the following format.

    folders:
        # We want all web editors the be able to edit
        # files and folders
        - path: "/var/www/site1"
          group: "www-editors"
          file_perms: "664"
          dir_perms: "775"

        # We want the uploads folder to be writable by
        # the webserver. This inherits the group, file and dir
        # permissions from its parent folder.
        - path "/var/www/site1/uploads"
          user: "www-data"

        # An entirely different folder
        - path: "/var/www/site2"
          dir_perms: "775"

## Options

Usage of enforced:

      -config /path/to/my/file: The location of config yaml file (required)
      -dry-run: Don't actually do anything
      -v: Output verbose logging
      -vv: Output highly verbose logging



