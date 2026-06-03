// modules/common.go - shared types and utility functions
// MobCat (2026)

package modules

import "fmt"
import "math"

// ============================================================
// Shared types
// ============================================================

// DecodeOptions carries the export settings from main into any decoder module.
type DecodeOptions struct {
	OutDir     string   // output directory for exported files
	DoExport   bool     // true if -export flag was present at all
	ExportVals []string // token list eg ["png", "gif"]; empty = smart default
}

// ROMInfo is implemented by each module's decoded ROM struct.
// main.go uses this interface so outputJSON works with any decoder's output
// without needing to know the concrete type.
type ROMInfo interface {
	GetRomID() string
	AppendMessage(level, msg string)
}

// ============================================================
// Error / warning helpers
// ============================================================

// FmtError formats a message string (printf-style) and appends it to a
// ROMInfo struct as a "warning" or "error" JSON field.
// Use this instead of fmt.Fprintf(os.Stderr, ...) so the message ends up in
// the JSON output rather than breaking the pipe.
//
//	FmtError(info, "warning", "Invalid region code %s", regionChar)
//	FmtError(info, "error",   "could not stat ROM file: %w", err)
func FmtError(info ROMInfo, level string, format string, args ...any) {
	info.AppendMessage(level, fmt.Sprintf(format, args...))
}

// ============================================================
// Shared utility functions
// ============================================================

// Contains reports whether s appears in slice.
func Contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// nullTermASCII trims null bytes and returns a clean ASCII string.
func nullTermASCII(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// decodeUTF16LE converts a null-terminated UTF-16LE byte slice to a Go string.
func decodeUTF16LE(b []byte) string {
	u16 := make([]uint16, len(b)/2)
	for i := range u16 {
		u16[i] = uint16(b[i*2]) | uint16(b[i*2+1])<<8
	}
	runes := make([]rune, 0, len(u16))
	for _, c := range u16 {
		if c == 0 {
			break
		}
		runes = append(runes, rune(c))
	}
	return string(runes)
}

// formatSize returns a human readable size string.
func formatSize(size uint64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	f := float64(size)
	units := []string{"KB", "MB", "GB"}
	for _, u := range units {
		f /= 1024
		if f < 1024 {
			// %g strips trailing zeros: 4.00 -> "4", 3.96 -> "3.96"
			return fmt.Sprintf("%g %s", math.Round(f*100)/100, u)
		}
	}
	// Clamp anything absurdly large to GB. No we are testing should be TBs in size.
	return fmt.Sprintf("%g GB", math.Round(f*100)/100)
}

