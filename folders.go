package main

import (
	"strings"
)

type folders map[string]*folder

func (fs folders) String() string {
	s := make([]string, 0)
	for _, f := range fs {
		s = append(s, f.String())
	}
	return strings.Join(s, " ")
}
