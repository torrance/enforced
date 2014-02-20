package main

// #include <pwd.h>
import "C"

import (
	"errors"
)

func getUserId(name string) (int, error) {
	user := C.getpwnam(C.CString(name))

	if user == nil {
		return 0, errors.New("Call to C function `getpwnam` returned null pointer")
	}
	return int(user.pw_uid), nil
}
