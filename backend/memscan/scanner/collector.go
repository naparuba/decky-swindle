// Copyright (C) 2025 kayon <kayon.hu@gmail.com>

package scanner

// CollectorFunc
// return false to terminate scanning
type CollectorFunc func(offset int) bool
