package main

import (
	"fmt"
	"os"
)

type folder struct {
	Path      string
	User      string
	Uid       int
	Group     string
	Gid       int
	DirPerms  string `yaml:"dir_perms"`
	DirMode   os.FileMode
	FilePerms string `yaml:"file_perms"`
	FileMode  os.FileMode
	Children  folders
}

func (f *folder) hasConfig() bool {
	return f.User != "" || f.Group != "" || f.DirPerms != "" || f.FilePerms != ""
}

// Merge non-empty config from the folder passed in.
// This always merges path and never merges children.
func (c *folder) mergeConfig(f *folder) {
	c.Path = f.Path

	if f.User != "" {
		c.User = f.User
		c.Uid = f.Uid
	}
	if f.Group != "" {
		c.Group = f.Group
		c.Gid = f.Gid
	}
	if f.FilePerms != "" {
		c.FilePerms = f.FilePerms
		c.FileMode = f.FileMode
	}
	if f.DirPerms != "" {
		c.DirPerms = f.DirPerms
		c.DirMode = f.DirMode
	}
}

func (f1 *folder) isEqual(f2 *folder) bool {
	if f1.User != f2.User || f1.Uid != f1.Uid || f1.Group != f2.Group || f1.Gid != f2.Gid || f1.FilePerms != f2.FilePerms ||
		f1.FileMode != f2.FileMode || f1.DirPerms != f2.DirPerms || f1.DirMode != f2.DirMode || len(f1.Children) != len(f2.Children) {
		return false
	}

	for k, v1 := range f1.Children {
		if v2, ok := f2.Children[k]; ok {
			if !v1.isEqual(v2) {
				// Key exists, but isn't recursively equal
				return false
			}
		} else {
			// Key doesn't exist.
			return false
		}
	}

	return true
}

func (f *folder) String() string {
	return fmt.Sprintf("[%s :: User: %s Uid: %d Group: %s Gid: %d FileMode: %04o DirMode: %04o Children: %v]",
		f.Path, f.User, f.Uid, f.Group, f.Gid, f.FileMode, f.DirMode, f.Children)
}
