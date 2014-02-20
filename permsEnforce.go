package main

import (
	"io/ioutil"
	"launchpad.net/goyaml"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func main() {
	folderList := loadYAML("config.example.yml")
	rootFolder := loadConfig(folderList, false)
	baseFolders := getBaseFolders(rootFolder)

	filepath.Walk(baseFolders[0], func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Println(err)
			return err
		}

		// Extract file/folder information
		sys := info.Sys()
		if sys == nil {
			log.Printf("Skipping file: sys interface is nil for %s", path)
			return nil
		}
		uid := int(sys.(*syscall.Stat_t).Uid)
		gid := int(sys.(*syscall.Stat_t).Gid)
		perms := info.Mode().Perm()
		isDir := info.Mode().IsDir()
		isRegular := info.Mode().IsRegular()

		// We don't yet know what to do with files that are neither regular or directories
		if !(isDir || isRegular) {
			log.Printf("Skippoing file: neither regular file or directory %s", path)
		}

		// Explode path into compononents, and remove first component if it is empty.
		paths := strings.Split(path, string(os.PathSeparator))
		if len(paths) > 0 && paths[0] == "" {
			paths = paths[1:]
		}

		// Get config for this file/folder
		c := &folder{}
		getConfig(paths, rootFolder, c)
		c.Path = path

		// If user/group is empty, then we want the file's owner/group to remain unchanged.
		if c.User == "" {
			c.Uid = uid
		}
		if c.Group == "" {
			c.Gid = gid
		}

		if isDir && c.DirMode != 0 && perms != c.DirMode {
			log.Printf("%s Changing permissions to %s\n", path, c.DirMode)
			if err := os.Chmod(path, c.DirMode); err != nil {
				log.Println(err)
			}
		}
		if !isDir && c.FileMode != 0 && perms != c.FileMode {
			log.Printf("%s Changing permissions to %s\n", path, c.FileMode)
			if err := os.Chmod(path, c.FileMode); err != nil {
				log.Println(err)
			}
		}
		if uid != c.Uid || gid != c.Gid {
			log.Printf("%s Changing ownership to %s (%d) / %s (%d)\n", path, c.User, c.Uid, c.Group, c.Gid)
			if err := os.Chown(path, c.Uid, c.Gid); err != nil {
				log.Println(err)
			}
		}

		return nil
	})
}

func loadYAML(path string) []*folder {
	configFile, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalln(err)
	}

	type config struct {
		Folders []*folder
	}

	c := new(config)
	if err := goyaml.Unmarshal(configFile, c); err != nil {
		log.Fatalln(err)
	}
	return c.Folders
}

func loadConfig(folderList []*folder, ignoreSystemErrors bool) *folder {
	rootFolder := &folder{Path: string(os.PathSeparator)}

	for _, f := range folderList {
		// Path must always exist.
		if len(f.Path) == 0 {
			log.Fatalln("Config file error: all folders must have a path attribute")
		}

		// Path must be absolute.
		if !filepath.IsAbs(f.Path) {
			log.Fatalln("Config file error: all folders must be absolute (ie. preceded by '/')")
		}

		// If user is set, grab system user id
		if f.User != "" {
			if uid, err := getUserId(f.User); err == nil {
				f.Uid = uid
			} else if !ignoreSystemErrors {
				// We failed to get the user id.
				log.Fatalf("Invalid user: %s", f.User)
			}
		}

		// If the group is set, grab the system group id
		if f.Group != "" {
			if gid, err := getGroupId(f.Group); err == nil {
				f.Gid = gid
			} else if !ignoreSystemErrors {
				// We failed to get the group id
				log.Fatalf("Invalid group: %s.", f.Group)
			}
		}

		// If file or dir perms are set, transform string to integer
		if f.FilePerms != "" {
			if fileMode, err := strconv.ParseUint(f.FilePerms, 8, 9); err == nil {
				f.FileMode = os.FileMode(fileMode)
			} else {
				log.Fatalf("Could not understand file perms: %s", f.FilePerms)
			}
		}
		if f.DirPerms != "" {
			if dirMode, err := strconv.ParseUint(f.DirPerms, 8, 9); err == nil {
				f.DirMode = os.FileMode(dirMode)
			} else {
				log.Fatalf("Could not understand file perms: %s", f.DirPerms)
			}
		}

		paths := strings.Split(f.Path, string(os.PathSeparator))
		currentFolder := rootFolder
		currentPath := []string{""}
		for i, p := range paths {
			// Ignore empty path components.
			if p == "" {
				continue
			}

			currentPath = append(currentPath, p)

			// Initialise children maps as we traverse the tree.
			if currentFolder.Children == nil {
				currentFolder.Children = make(map[string]*folder)
			}

			if i == len(paths)-1 {
				// Add folder configuration if we're at the last path component.
				newFolder := f
				currentFolder.Children[p] = newFolder
				currentFolder = newFolder
			} else if f, ok := currentFolder.Children[p]; ok {
				// Folder already exists and may contain config. Don't overwrite.
				currentFolder = f
			} else {
				// No child exists. Create empty placeholder folder configuration.
				newFolder := &folder{Path: strings.Join(currentPath, string(os.PathSeparator))}
				currentFolder.Children[p] = newFolder
				currentFolder = newFolder
			}
		}
	}
	return rootFolder
}

func getBaseFolders(f *folder) []string {
	if f.hasConfig() {
		return []string{f.Path}
	}

	baseFolders := []string{}
	for _, f := range f.Children {
		baseFolders = append(baseFolders, getBaseFolders(f)...)
	}

	return baseFolders
}

func getConfig(paths []string, currentFolder *folder, config *folder) {
	config.mergeConfig(currentFolder)

	// Check if we've reached our final destination
	if len(paths) == 0 {
		return
	}

	// Attempt to find next child node
	if nextFolder, ok := currentFolder.Children[paths[0]]; ok {
		// Child folder config exists. Recurse.
		getConfig(paths[1:], nextFolder, config)
		return
	} else {
		// Otherwise this is as far as we can go. We have our config.
		return
	}
}
