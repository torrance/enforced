package main

import (
	"testing"
)

var testFolderList []*folder = []*folder{
	&folder{
		Path:     "/var",
		User:     "www-data",
		DirPerms: "775",
	},
	&folder{
		Path:  "/var/site",
		Group: "www-editors",
	},
	&folder{
		Path:      "/lib/folder2",
		User:      "adm",
		FilePerms: "664",
	},
}

var testFolderConfig *folder = &folder{
	Path: "/",
	Children: map[string]*folder{
		"var": &folder{
			Path:     "/var",
			User:     "www-data",
			DirPerms: "775",
			DirMode:  0775,
			Children: map[string]*folder{
				"site": &folder{
					Path:  "/var/site",
					Group: "www-editors",
				},
			},
		},
		"lib": &folder{
			Path: "/lib",
			Children: map[string]*folder{
				"folder2": &folder{
					Path:      "/lib/folder2",
					User:      "adm",
					FilePerms: "664",
					FileMode:  0664,
				},
			},
		},
	},
}

var testFolderConfigCopy *folder = &folder{
	Path: "/",
	Children: map[string]*folder{
		"var": &folder{
			Path:     "/var",
			User:     "www-data",
			DirPerms: "775",
			DirMode:  0775,
			Children: map[string]*folder{
				"site": &folder{
					Path:  "/var/site",
					Group: "www-editors",
				},
			},
		},
		"lib": &folder{
			Path: "/lib",
			Children: map[string]*folder{
				"folder2": &folder{
					Path:      "/lib/folder2",
					User:      "adm",
					FilePerms: "664",
					FileMode:  0664,
				},
			},
		},
	},
}

var testFolderConfigNonCopy *folder = &folder{
	Path: "/",
	Children: map[string]*folder{
		"var": &folder{
			Path:     "/var",
			User:     "www-data",
			DirPerms: "775",
			DirMode:  0775,
			Children: map[string]*folder{
				"site": &folder{
					Path:  "/var/site",
					Group: "admin",
				},
			},
		},
		"lib": &folder{
			Path: "/lib",
			Children: map[string]*folder{
				"folder2": &folder{
					Path:      "/lib/folder2",
					User:      "adm",
					FilePerms: "664",
					FileMode:  0664,
				},
			},
		},
	},
}

func TestFolderIsEqual(t *testing.T) {
	if !testFolderConfig.isEqual(testFolderConfigCopy) {
		t.Error("isEqual returned false, expected true [1]")
	}
	if testFolderConfig.isEqual(testFolderConfigNonCopy) {
		t.Error("isEqual returned true, expected false [2]")
	}
}

func TestLoadConfig(t *testing.T) {
	rootFolder := loadConfig(testFolderList, true)
	if !rootFolder.isEqual(testFolderConfig) {
		t.Errorf("Loaded folder config incorrect, got %v", rootFolder)
	}
}

func TestGetBaseFolders(t *testing.T) {
	bf := getBaseFolders(testFolderConfig)

	if len(bf) != 2 {
		t.Fatalf("Incorrect number of base folders: expected 2, got %d", len(bf))
	}

	if !(bf[0] == "/var" && bf[1] == "/lib/folder2") && !(bf[0] == "/lib/folder2" && bf[1] == "/var") {
		t.Errorf("Incorrect base folders, got: %v", bf)
	}
}

func TestHasConfig(t *testing.T) {
	f := testFolderConfig

	if f.hasConfig() {
		t.Error("hasConfig returned true, expected false [1]")
	}
	if !f.Children["var"].hasConfig() {
		t.Error("hasConfig returned false, expected true [2]")
	}
	if !f.Children["var"].Children["site"].hasConfig() {
		t.Error("hasConfig returned false, expected true [3]")
	}
	if f.Children["lib"].hasConfig() {
		t.Error("hasConfig returned true, expected false [4]")
	}
	if !f.Children["lib"].Children["folder2"].hasConfig() {
		t.Error("hasConfig returned false, expected true [5]")
	}
}

func TestMergeConfig(t *testing.T) {
	folder1 := &folder{
		Path:      "test",
		User:      "me",
		FilePerms: "0655",
		FileMode:  0655,
	}
	folder2 := &folder{
		Path:  "test/more",
		Group: "myself",
		User:  "root",
		Uid:   0,
	}
	folder3 := &folder{
		Path:      "test/more",
		User:      "root",
		Uid:       0,
		Group:     "myself",
		FilePerms: "0655",
		FileMode:  0655,
	}

	c := &folder{}
	c.mergeConfig(folder1)

	if !c.isEqual(folder1) {
		t.Errorf("Merge incorrect: got %v", c)
	}

	c.mergeConfig(folder2)
	if !c.isEqual(folder3) {
		t.Errorf("Merge incorrect: got %v", c)
	}
}

func TestGetConfig(t *testing.T) {
	c1 := &folder{
		Path:     "/var",
		User:     "www-data",
		DirPerms: "775",
		DirMode:  0775,
	}
	c2 := &folder{
		Path:     "/var/site",
		User:     "www-data",
		Group:    "www-editors",
		DirPerms: "775",
		DirMode:  0775,
	}
	c3 := &folder{
		Path:      "/lib/folder2",
		User:      "adm",
		FilePerms: "664",
		FileMode:  0664,
	}

	c := &folder{}
	getConfig([]string{"var", "my", "own", "path"}, testFolderConfig, c)
	if !c.isEqual(c1) {
		t.Errorf("getConfig incorrect, got: %v", c)
	}

	c = &folder{}
	getConfig([]string{"var", "site", "path"}, testFolderConfig, c)
	if !c.isEqual(c2) {
		t.Errorf("getConfig incorrect, got: %v", c)
	}

	c = &folder{}
	getConfig([]string{"lib", "folder2", "path", "again"}, testFolderConfig, c)
	if !c.isEqual(c3) {
		t.Errorf("getConfig incorrect, got: %v", c)
	}
}
