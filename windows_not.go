// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !windows

package fsnotify

import (
	"path/filepath"
	"strings"
)

// isHidden determines if a file/path is hidden if it starts with a dot
func isHidden(name string) bool {
	base := filepath.Base(name)
	return strings.HasPrefix(base, ".") && base != "." && base != ".."
}
