//go:build tools

// Package tools pins modules that only tools/genstatic.go (build tag
// "ignore") imports, so go mod tidy keeps them.
package tools

import (
	_ "github.com/shurcooL/httpfs/filter"
	_ "github.com/shurcooL/vfsgen"
)
