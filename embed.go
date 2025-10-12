package main

import (
	_ "embed"
)

//go:embed static/index.html
var indexHTML []byte
