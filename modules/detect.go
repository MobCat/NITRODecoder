// modules/detect.go - ROM type fingerprinting
// MobCat (2026)

package modules

import (
	"bytes"
	"strings"
)

// FingerprintHeader reads the first 16 bytes of the ROM header and returns a
// 16-character string where each character represents the "class" of that byte.
// This fingerprint is used for NDS/GBA/BIOS detection where header data lives
// in the first 16 bytes. GB/GBC headers are structured differently (data starts
// at 0x100) so they are NOT detected here — CollectSignals handles those.
func FingerprintHeader(hdr []byte) string {
	var temp []string
	for i, b := range hdr[0x0:0x10] {
		switch {
		case b >= ' ' && b <= '`':
			temp = append(temp, "A") // printable ASCII [0x20:0x60]
		case b == 0xFF:
			temp = append(temp, "G") // full byte [0xFF]
		case i == 0x03 && b == 0xEA:
			temp = append(temp, "B") // gBa fixed value
		case b == 0x00:
			temp = append(temp, "N") // null byte [0x00]
		default:
			temp = append(temp, "?") // unknown/not fingerprinted
		}
	}
	return strings.Join(temp, "")
}

// ROMSignals holds the pre-computed signals extracted from the header.
// FingerprintHeader covers the first 16 bytes (good for NDS/GBA/BIOS).
// The GB/GBC/DMG signals require looking past 0x100, so they live here
// separately rather than being jammed into the fingerprint string.
type ROMSignals struct {
	FP             string // 16-char fingerprint of bytes 0x00-0x0F
	HasGBABranch   bool   // hdr[0x03] == 0xEA is ARM branch opcode
	HasGBEntry     bool   // hdr[0x100:0x102] == 00 C3 is LR35902 NOP + JP
	HasGBCFlag     bool   // hdr[0x143] == 0x80 or 0xC0
	CGBFlag        byte   // raw value of hdr[0x143] for CGB/DMG distinction
	HasNDSGameCode bool   // hdr[0x0C:0x10] all A-Z uppercase ASCII
	HasNDSUnitCode bool   // hdr[0x12] in {0x00, 0x02, 0x03}
}

// CollectSignals builds a ROMSignals from the first 512 bytes of the ROM.
// This is the single place where we decide what kind of ROM this might be,
// before handing off to the more detailed per-format checks.
func CollectSignals(hdr []byte) ROMSignals {
	s := ROMSignals{}
	s.FP = FingerprintHeader(hdr)

	// GBA: ARM branch instruction has 0xEA at byte 3 (little-endian opcode)
	s.HasGBABranch = hdr[0x03] == 0xEA

	// GB/GBC: header lives at 0x100+, need at least 0x150 bytes
	if len(hdr) >= 0x150 {
		// Entry point: almost always 00 C3 xx xx (NOP + JP nn)
		// Some homebrew/hacks use C3 xx xx directly without the leading NOP,
		// but 00 C3 is the canonical form from Nintendo's SDK.
		s.HasGBEntry = hdr[0x100] == 0x00 && hdr[0x101] == 0xC3

		// CGB flag byte: 0x80 = runs on GBC (also works on DMG), 0xC0 = GBC only
		// On a DMG ROM this byte is the last char of the title field — ASCII or null.
		// 0x80 and 0xC0 are not valid ASCII title chars so this is a clean signal.
		s.CGBFlag    = hdr[0x143]
		s.HasGBCFlag = s.CGBFlag == 0x80 || s.CGBFlag == 0xC0
	}

	// NDS: game code at 0x0C-0x0F must be exactly 4 uppercase A-Z chars
	s.HasNDSGameCode = isUpperAlpha(hdr[0x0C:0x10])

	// NDS: unit code 0x12 — 0x00=DS, 0x02=DSi enhanced, 0x03=DSi only
	u := hdr[0x12]
	s.HasNDSUnitCode = u == 0x00 || u == 0x02 || u == 0x03

	return s
}

// DetectROMType maps collected signals + fingerprint to a known ROM type.
// Returns "###" if the type cannot be determined.
// Ordering matters: more specific checks go first to avoid false positives.
func DetectROMType(s ROMSignals, hdr []byte) string {
	// GB/GBC: must check before NDS because a GB entry point doesn't conflict
	// with NDS but we want to be explicit about precedence.
	if s.HasGBCFlag || s.HasGBEntry {
		return isCGB(s, hdr)
	}

	// DMG: valid GB entry point but no CGB flag (0x143 is end of title field)
	// Only reach here if HasGBEntry is false and HasGBCFlag is false,
	// but we still might have a DMG — check the entry point directly.
	if len(hdr) >= 0x150 && hdr[0x100] == 0x00 && hdr[0x101] == 0xC3 {
		return isDMG(hdr)
	}

	// GBA: ARM branch opcode at byte 3
	if s.HasGBABranch {
		return isAGB(s.FP, hdr)
	}

	// NDS/DSi: game code + unit code both look right
	if s.HasNDSGameCode && s.HasNDSUnitCode {
		return "NDS"
	}

	// Fall back to fingerprint switch for BIOS images and firmware
	return detectByFP(s.FP, hdr)
}

// isCGB handles GBC and GBC-compatible ROMs.
// Mirrors the tier pattern used in isAGB.
func isCGB(s ROMSignals, hdr []byte) string {
	// Tier 1: GBC-exclusive flag + canonical entry point — clean retail CGB
	if hdr[0x143] == 0xC0 && hdr[0x100] == 0x00 && hdr[0x101] == 0xC3 {
		return "CGB"
	}

	// Tier 2: GBC-compatible flag + entry point — runs on DMG too
	if hdr[0x143] == 0x80 && hdr[0x100] == 0x00 && hdr[0x101] == 0xC3 {
		return "CGBC" // compatible mode, boots on original DMG as well
	}

	// Tier 3: CGB flag is set but entry point is non-standard — homebrew/hack
	// The hardware will still try to boot it in GBC mode.
	if hdr[0x143] == 0xC0 || hdr[0x143] == 0x80 {
		return "CGBF" // flagged: CGB flag present but entry point is unusual
	}

	// Tier 4: entry point looks like GB but no CGB flag — could be DMG or misdetect
	if hdr[0x100] == 0x00 && hdr[0x101] == 0xC3 {
		return "CGBF"
	}

	return "###"
}

// isDMG handles original Game Boy ROMs (no CGB flag).
func isDMG(hdr []byte) string {
	// Tier 1: standard entry point + old-style licensee (pre-GBC era)
	// 0x14B != 0x33 means the single-byte publisher code is used directly.
	if hdr[0x14B] != 0x33 {
		return "DMG"
	}

	// Tier 2: standard entry point + new licensee code (0x14B == 0x33)
	// Later DMG games and some cross-gen titles used the updated format.
	if hdr[0x14B] == 0x33 {
		return "DMGN" // new licensee format
	}

	// Tier 3: entry point looks GB-ish but other fields are off — homebrew/hack
	return "DMGF"
}

// detectByFP handles BIOS images and firmware that are identified purely
// by their 16-byte fingerprint. These are all the cases that don't have a
// clean structural signal in the extended header.
func detectByFP(fp string, hdr []byte) string {
	switch fp {
	// DS/DSi firmware — identified by MAC address / known byte patterns
	case "?A?A????AAA?ANNA", // [BIOS] iQue DS and DS Lite Firmware
		"?A?A???AAAAAANNA",  // [BIOS] Nintendo DS Firmware
		"?A?A????AAAAANNA",  // [BIOS] Nintendo DS Firmware
		"?A?AA???AAAAANNA",  // [BIOS] Nintendo DS Firmware
		"?AAA????AAA?ANNA",  // [BIOS] Nintendo DS Lite Firmware
		"AAA???A?AAA?ANNA",  // [BIOS] Nintendo DS Lite Firmware
		"?A??N???AAA?ANNA",  // [BIOS] Nintendo DS Lite Firmware
		"?A?AA???AAA?ANNA",  // [BIOS] Nintendo DS Firmware (IS-NITRO-EMULATOR)
		"?A?A??AAAAA?ANNA",  // [BIOS] Nintendo DS Lite Firmware
		"?NN??NN?ANN??NN?",  // [BIOS] Nintendo DS ARM7TDMI (GBA Mode)
		"?NN?A?N???N???N?",  // [BIOS] Nintendo DS ARM7TDMI
		"ANN??NN??NN??NN?",  // [BIOS] Nintendo DS ARM946E-S
		"?NNB?NN??NN??NN?":  // [BIOS] Nintendo DSi ARM7 Boot ROM (World)
		return "DSFW"

	case "A?GA???N?N????N?",
		"?NNB?NN?ANN??NN?":
		return "AGBFW"  // GameBoy Advanced bios

	case "GNNNNNNNGNNNNNNN":
		return "DMG" // Original GameBoy (older fingerprint path, keep for compat)

	case "AA???N?N?NN??AAA",
		"AA???N?N?NN??AA?",
		"AA???N?N?NN??A??",
		"AA???N?N?NN?????",
		"AA???N?N?NN???AA",
		"AA???N?N?NN??A?A":
		return "ZIP - invalid"
	}

	// BUGBUG: Nintendo DSi ARM9 Boot ROM thinks it's a GBA ROM here.
	// Keeping this as the last-resort check after everything else fails.
	if len(fp) > 3 && fp[3] == 'B' {
		return isAGB(fp, hdr)
	}

	return "Not a valid Game ROM file"
}

// isUpperAlpha returns true if all bytes in b are uppercase A-Z.
func isUpperAlpha(b []byte) bool {
	for _, c := range b {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

// NDS fingerprint check. Kept for any caller that still uses the old string path.
// The main detection now goes through CollectSignals + DetectROMType.
func isNDSFingerprint(fp string) bool {
	switch fp {
	case "AANNNNNNNNNNAAAA",
		"AAANNNNNNNNNAAAA",
		"AAAANNNNNNNNAAAA",
		"AAAAANNNNNNNAAAA",
		"AAAAAANNNNNNAAAA",
		"AAAAAAANNNNNAAAA",
		"AAAAAAAANNNNAAAA",
		"AAAAAAAAANNNAAAA",
		"AAAAAAAAAANNAAAA",
		"AAAAAAAAAAANAAAA",
		"AAAAAAAAAAAAAAAA":
		return true
	}
	return false
}

// Ensure bytes is used (it's imported for potential future logo checks)
var _ = bytes.Equal
