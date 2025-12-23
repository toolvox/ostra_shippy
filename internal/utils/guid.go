package utils

import (
	"crypto/rand"
	"fmt"
)

// GenerateGUID creates a new GUID in the format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
func GenerateGUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
