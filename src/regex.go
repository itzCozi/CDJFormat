package main

import "regexp"

var (
	diskIDRegex = regexp.MustCompile(`/dev/(disk\d+)`)
	sizeRegex   = regexp.MustCompile(`([\d.]+)\s*(GB|MB|TB|Bytes)`)
)
