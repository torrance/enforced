package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/howeyc/fsnotify"
	logging "github.com/op/go-logging"
	"io/ioutil"
	"launchpad.net/goyaml"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type fileDescriptor struct {
	path *string
	info *os.FileInfo
}

var log *logging.Logger

func main() {
	// Load command line arguments
	configPath := flag.String("config", "", "The location of config yaml file.")
	dryRun := flag.Bool("dry-run", false, "Don't actually do anything")
	verbose := flag.Bool("v", false, "Output verbose logging")
	veryVerbose := flag.Bool("vv", false, "Output highly verbose logging")
	flag.Parse()

	// Set up logging
	log = logging.MustGetLogger("permsEnforce")
	switch {
	case *veryVerbose:
		logging.SetLevel(logging.DEBUG, "permsEnforce")
		log.Debug("Very verbose logging enabled")
	case *verbose:
		logging.SetLevel(logging.INFO, "permsEnforce")
		log.Info("Verbose logging enabled")
	default:
		logging.SetLevel(logging.ERROR, "permsEnforce")
	}

	log.Info("Config path: %s", *configPath)
	if dryRun {
		log.Info("Dry run enabled")
	}

	folderList, err := loadYAML(*configPath)
	if err != nil {
		log.Critical("Failed to load YAML config file: %s", err)
		return
	}

	rootFolder, err := loadConfig(folderList, false)
	if err != nil {
		log.Critical("Failed to process config: %s", err)
		return
	}

	baseFolders := getBaseFolders(rootFolder)
	if len(baseFolders) == 0 {
		log.Critical("No configuration rules found.")
		return
	}

	ch := make(chan fileDescriptor, 1000)
	go updateFile(rootFolder, ch, *dryRun)

	// Start watching for file changes
	// While this means we will redundantly check any files we change
	// whilst we walk the full stack, it means we catch any files that change during the walk.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Critical("Error occurred creating new file watcher: %s", err)
		return
	}
	// Fsnotify doesn't yet recursively watch. So we have to do this ourselves.
	for _, baseFolder := range baseFolders {
		err := filepath.Walk(baseFolder, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				err = watcher.Watch(path)
			}
			return err
		})
		if err != nil {
			log.Critical("Error occurred adding folders to watcher: %s", err)
			return
		}
	}

	// Let's make an initial walk across every single file and set it correctly.
	go func() {
		for _, baseFolder := range baseFolders {
			err := filepath.Walk(baseFolder, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				ch <- fileDescriptor{&path, &info}
				return nil
			})
			// Stop execution if we can't walk full file tree.
			if err != nil {
				log.Critical("Error occurred walking files: %s", err)
				os.Exit(1)
			}
		}
	}()

	// Now let's start handling file changes until we're told to quit.
	for {
		select {
		case ev := <-watcher.Event:
			log.Debug(ev.String())
			if !(ev.IsCreate() || ev.IsAttrib()) {
				// We don't care. These events don't affect file ownership or permissions.
				continue
			}

			fileInfo, err := os.Stat(ev.Name)
			if err != nil {
				log.Error("Failed to stat changed file: %s", err)
				continue
			}
			mode := fileInfo.Mode()

			// Add new folders to watcher
			if ev.IsCreate() && mode.IsDir() {
				err := watcher.Watch(ev.Name)
				if err != nil {
					log.Error("Failed to add new folder to watchlist: %s", err)
				} else {
					log.Debug("Added new directory to watcher: %s", ev.Name)
				}
			}

			// Send on the file to be processed
			ch <- fileDescriptor{&ev.Name, &fileInfo}

		case err := <-watcher.Error:
			log.Error("Watcher error: %s", err)
		}
	}
}

func loadYAML(path string) (folders []*folder, err error) {
	configFile, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}

	type config struct {
		Folders []*folder
	}

	c := new(config)
	if err = goyaml.Unmarshal(configFile, c); err != nil {
		return
	}
	return c.Folders, nil
}

func loadConfig(folderList []*folder, ignoreSystemErrors bool) (rootFolder *folder, err error) {
	rootFolder = &folder{Path: string(os.PathSeparator)}

	for _, f := range folderList {
		// Path must always exist.
		if len(f.Path) == 0 {
			err = errors.New("all folders must have a path attribute")
			return
		}

		// Path must be absolute.
		if !filepath.IsAbs(f.Path) {
			err = errors.New("all folders must be absolute (ie. preceded by '/')")
			return
		}

		// If user is set, grab system user id
		if f.User != "" {
			if uid, err := getUserId(f.User); err == nil {
				f.Uid = uid
			} else if !ignoreSystemErrors {
				// We failed to get the user id.
				err = errors.New(fmt.Sprintf("invalid user %s", f.User))
				return rootFolder, err
			}
		}

		// If the group is set, grab the system group id
		if f.Group != "" {
			if f.Gid, err = getGroupId(f.Group); err != nil && !ignoreSystemErrors {
				// We failed to get the group id
				err = errors.New(fmt.Sprintf("invalid group: %s.", f.Group))
				return
			}
		}

		// If file or dir perms are set, transform string to integer
		if f.FilePerms != "" {
			var fileMode uint64
			if fileMode, err = strconv.ParseUint(f.FilePerms, 8, 9); err == nil {
				f.FileMode = os.FileMode(fileMode)
			} else {
				err = errors.New(fmt.Sprintf("could not understand file perms: %s", f.FilePerms))
				return
			}
		}
		if f.DirPerms != "" {
			var dirMode uint64
			if dirMode, err = strconv.ParseUint(f.DirPerms, 8, 9); err == nil {
				f.DirMode = os.FileMode(dirMode)
			} else {
				err = errors.New(fmt.Sprintf("could not understand file perms: %s", f.DirPerms))
				return
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
	return
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

func updateFile(rootFolder *folder, ch chan fileDescriptor, dryRun bool) {
	for f := range ch {
		log.Debug("Processing file: %s", *f.path)

		// Extract file/folder information
		sys := (*f.info).Sys()
		if sys == nil {
			log.Error("Skipping file: sys interface is nil for %s", *f.path)
			return
		}
		uid := int(sys.(*syscall.Stat_t).Uid)
		gid := int(sys.(*syscall.Stat_t).Gid)
		mode := (*f.info).Mode()
		perms := mode.Perm()
		isDir := mode.IsDir()
		isRegular := mode.IsRegular()
		isSymlink := mode&os.ModeSymlink != 0

		// We only know how to handle regular files, directories and symlinks
		if !(isDir || isRegular || isSymlink) {
			log.Info("Skipping file: neither regular file, directory or symlink %s", *f.path)
		}

		// Explode path into compononents, and remove first component if it is empty.
		paths := strings.Split(*f.path, string(os.PathSeparator))
		if len(paths) > 0 && paths[0] == "" {
			paths = paths[1:]
		}

		// Get config for this file/folder
		c := &folder{}
		getConfig(paths, rootFolder, c)

		// If user/group is empty, then we want the file's owner/group to remain unchanged.
		if c.User == "" {
			c.Uid = uid
		}
		if c.Group == "" {
			c.Gid = gid
		}

		// Set permissions for files.
		if isDir && c.DirMode != 0 && perms != c.DirMode {
			log.Info("%s Changing permissions to %s\n", *f.path, c.DirMode)
			if !dryRun {
				if err := os.Chmod(*f.path, c.DirMode); err != nil {
					log.Error("%s", err)
				}
			}
		}
		// Set permissions for directories.
		if isRegular && c.FileMode != 0 && perms != c.FileMode {
			log.Info("%s Changing permissions to %s\n", *f.path, c.FileMode)
			if !dryRun {
				if err := os.Chmod(*f.path, c.FileMode); err != nil {
					log.Error("%s", err)
				}
			}
		}
		// Set ownership for files, directories and symlinks.
		if uid != c.Uid || gid != c.Gid {
			log.Info("%s Changing ownership to %s (%d) / %s (%d)\n", *f.path, c.User, c.Uid, c.Group, c.Gid)
			if !dryRun {
				if err := os.Chown(*f.path, c.Uid, c.Gid); err != nil {
					log.Error("%s", err)
				}
			}
		}
	}
}
