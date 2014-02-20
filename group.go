package main

// #include <grp.h>
import "C"

import (
	"errors"
)

func getGroupId(name string) (int, error) {
	group := C.getgrnam(C.CString(name))

	if group == nil {
		return 0, errors.New("Call to C function `getgrnam` returned null pointer")
	}
	return int(group.gr_gid), nil
}
