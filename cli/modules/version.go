package modules

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"sync"
)

// Application info - centralized
const (
	AppName        = "CSD DevTrack"
	AppVersion     = "0.1.0"
	AppDescription = "Multi-project development tool"
)

// buildHash is computed once at runtime from the binary's properties
var (
	buildHash     string
	buildHashOnce sync.Once
)

// BuildHash returns the build hash (computed from binary at runtime)
// Format: YYMMDD-xxxxxxxx (mod date + 8-char hash of binary)
func BuildHash() string {
	buildHashOnce.Do(computeBuildHash)
	return buildHash
}

// computeBuildHash computes a unique hash for this binary
func computeBuildHash() {
	executable, err := os.Executable()
	if err != nil {
		buildHash = "000000-unknown0"
		return
	}

	info, err := os.Stat(executable)
	if err != nil {
		buildHash = "000000-unknown1"
		return
	}

	// Date part: YYMMDD from modification time
	modTime := info.ModTime()
	datePart := modTime.Format("060102")

	// Hash part: first 8 chars of SHA256 of binary
	f, err := os.Open(executable)
	if err != nil {
		buildHash = datePart + "-unknown2"
		return
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		buildHash = datePart + "-unknown3"
		return
	}

	hashHex := fmt.Sprintf("%x", h.Sum(nil))
	buildHash = datePart + "-" + hashHex[:8]
}
