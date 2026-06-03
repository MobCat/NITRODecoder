// NitroDecoder - Nintendo DS and DSi ROM metadata extractor
// MobCat (2026)

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"NitroDecoder/modules"
)

// ============================================================
// Arg helpers
// ============================================================

// extractMultiFlag scans args for -name / --name and collects all following
// non-flag tokens as its values. Returns a non-nil slice (possibly empty) when
// the flag is found, nil when absent. Consumed tokens are removed from rest.
//
//	-export           found=true, vals=[]
//	-export png gif   found=true, vals=["png","gif"]
func extractMultiFlag(args []string, name string) (vals []string, rest []string, found bool) {
	for i, a := range args {
		if a == "-"+name || a == "--"+name {
			rest = append(args[:i:i], args[i+1:]...)
			j := i
			for j < len(rest) && len(rest[j]) > 0 && rest[j][0] != '-' {
				vals = append(vals, strings.ToLower(rest[j]))
				rest = append(rest[:j:j], rest[j+1:]...)
			}
			return vals, rest, true
		}
	}
	return nil, args, false
}

// extractBoolFlag scans args for -name / --name (no value). Returns true and
// removes it from args when found.
func extractBoolFlag(args []string, name string) (bool, []string) {
	for i, a := range args {
		if a == "-"+name || a == "--"+name {
			return true, append(args[:i:i], args[i+1:]...)
		}
	}
	return false, args
}

// extractValueFlag scans args for -name value or --name value (single token value).
// Returns the value pointer (nil if absent) and remaining args.
func extractValueFlag(args []string, name string) (*string, []string) {
	for i, a := range args {
		if a == "-"+name || a == "--"+name {
			rest := append(args[:i:i], args[i+1:]...)
			if i < len(rest) && len(rest[i]) > 0 && rest[i][0] != '-' {
				val := rest[i]
				rest = append(rest[:i:i], rest[i+1:]...)
				return &val, rest
			}
			val := ""
			return &val, rest
		}
		for _, prefix := range []string{"-" + name + "=", "--" + name + "="} {
			if strings.HasPrefix(a, prefix) {
				val := a[len(prefix):]
				rest := append(args[:i:i], args[i+1:]...)
				return &val, rest
			}
		}
	}
	return nil, args
}

// ============================================================
// Main
// ============================================================
func main() {
	rawArgs := os.Args[1:]
	if len(rawArgs) == 0 || rawArgs[0] == "-h" || rawArgs[0] == "--help" {
		fmt.Fprintln(os.Stderr, `NitroDecoder Help
This tool can be used to extract metadata like rom ids and publisher info from nintendio game roms.
It can also be used to extract rom icons for the roms that support this.
Extracted metadata is in json format for easy decoding with or piping to other tools and apps.

CLI Usage:     NitroDecoder rom -flags
CLI Usage:     NitroDecoder "path/to/rom/file.ext" -flags
Normal people: Just drag'n'drop a ROM file to NitroDecoder and it will open it, no cli needed.

Flags:
  -dir <path>
      Output directory for exported files.
      Default: same directory as the ROM.

  -export [png] [bmp] [gif] [json]
      Exports images and data in the chosen format/s.
      if not export formats spesifyed, always exports PNG;
      also exports GIF if the ROM is DSi and has more then 1 animation frame.

      png  = transparent PNG of the static icon
      bmp  = indexed BMP(s) with raw palette
             DSi ROMs: also exports each animation frame as ID-N.BMP
      gif  = animated GIF (single-frame + warning if not DSi)
      json = write JSON to file instead of stdout

      (just use -export png for eg. dont include the [] or <> in path)

  -cli
      Suppress the "Press enter to close" prompt when NitroDecoder has finished.
      Use when piping output to another program. Otherwise excluding this helps
      with drag'n'drop mode so the app doesn't open and then just close before you can read it.

Supported ROMs:
	NitroDecoder curently supports the following consoles and ROM Types

	Console          | Extension    | Unit/Prefix Code | Note
	Gameboy Advanced | .gba         | AGB              | No save type decode yet.
	DS               | .nds or .ids | NTR              | Decrypted and Encrypted
	DSi              | .nds         | TWI              | Decrypted and Encrypted
`)
		waitForEnter()
		return
	}
//Unsported ROMs.
//Play-Yan: AGS-006. The common avaible romsets only have the mpeg files for the sd card, not a dump of the main gba cart.
//3ds: Dont have any plans for this because idk what to do with the 3d banner files.
//nsp: Lol no. outside of the encryption, im not gonna go there.
//xci: Doubble nope. Thats a mess for a difrent reson I dont wanna poke at.
//zip: Dude, extract your rom files. yes really.
//Limitations.
//Decrypted VS Encrypted DS roms: The game contence is encrypted, not the header we are reading.
//and only 0x4000 to 0x47FF (only 0x800 or 2 kiB) is encrypted???? needs more testing and data points.
//Gameboy Advanced video: While NitroDecoder does support these large rom files, as we are only reading the
//Header we cant tell outside of the large rom size if its a gba video or a game.

////////////////////////////////////////////////////////////////////////////////////////////////////////////

	romPath  := rawArgs[0]
	flagArgs := rawArgs[1:]

	// Parse flags
	exportVals, flagArgs, doExport := extractMultiFlag(flagArgs, "export")
	dirVal, flagArgs := extractValueFlag(flagArgs, "dir")
	cliMode, _ := extractBoolFlag(flagArgs, "cli")

	// Determine output directory
	var outDir string
	if dirVal != nil && *dirVal != "" {
		outDir = *dirVal
	} else {
		outDir = filepath.Dir(romPath)
	}

	// Build export options to pass to the decoder module
	opts := modules.DecodeOptions{
		OutDir:     outDir,
		DoExport:   doExport,
		ExportVals: exportVals,
	}

	// --- Stage 1: read the first 512 bytes ---
	f, err := os.Open(romPath)
	if err != nil {
		outputError(fmt.Sprintf("error opening ROM: %v", err))
		os.Exit(1)
	}
	defer f.Close()

	hdr := make([]byte, 0x200) // 512 bytes
	if n, err := f.Read(hdr); err != nil || n < 0x200 {
		outputError("file too small to be a valid ROM")
		os.Exit(1)
	}

	// --- Stage 2: fingerprint and dispatch ---
	signals := modules.CollectSignals(hdr)
	ROMType := modules.DetectROMType(signals, hdr)

	switch ROMType {
	case "DSFW":
		outputError("NitroDecoder can not decode firmware files... yet..")

	case "AGBFW":
		outputError(fmt.Sprintf("NitroDecoder can not decode firmware files... yet.."))

	case "NDS":
		info, err := modules.DecodeNDS(hdr, f, romPath, opts)
		if err != nil {
			outputError(fmt.Sprintf("error decoding NDS ROM: %v", err))
			os.Exit(1)
		}
		outputJSON(info, opts, cliMode)

	case "AGB", "AGBL", "AGBF":
		info, err := modules.DecodeAGB(hdr, f, romPath, opts)
		if err != nil {
			outputError(fmt.Sprintf("error decoding AGB ROM: %v", err))
			os.Exit(1)
		}
		//AGBL: Bad logo, but other data is at least set
		//AGBF: More bad data, only contains the bear miniuem to load in a gba. defently a hack or homebrew of some kind.
		outputJSON(info, opts, cliMode)

	case "CGB", "DMG":
		outputError(fmt.Sprintf("%s ROMs are not yet supported", ROMType))

	default:
		//as this is the "default" this will cause false postives where you think its a rom but its just a random file
		//inlcuding romtupe here is just a check to make sure I dident mist a case match
		outputError(fmt.Sprintf("Unknown file (fingerprint: %s)(ROMType: %s)", signals.FP, ROMType))

	}

	if !cliMode {
		waitForEnter()
	}
}

//////////////////////////////////////////////////////////////////////////////////////////////////

// outputError prints a minimal JSON error object to stdout.
// This keeps stdout valid JSON even on fatal errors, so downstream pipes don't break.
func outputError(msg string) {
	fmt.Printf("{\n  \"error\": %q\n}\n", msg)
}

// outputJSON marshals info to JSON and either writes it to a file, prints it
// to stdout, or both. Matching the original behaviour.
func outputJSON(info modules.ROMInfo, opts modules.DecodeOptions, cliMode bool) {
	exportJSON := modules.Contains(opts.ExportVals, "json")
	exportPNG  := modules.Contains(opts.ExportVals, "png")
	exportBMP  := modules.Contains(opts.ExportVals, "bmp")
	exportGIF  := modules.Contains(opts.ExportVals, "gif")

	buf := &strings.Builder{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(info); err != nil {
		outputError(fmt.Sprintf("error marshalling JSON: %v", err))
		os.Exit(1)
	}
	jsonStr := strings.TrimRight(buf.String(), "\n")

	if exportJSON {
		jsonPath := filepath.Join(opts.OutDir, info.GetRomID()+".json")
		if err := os.WriteFile(jsonPath, []byte(jsonStr), 0644); err != nil {
			outputError(fmt.Sprintf("error writing JSON file: %v", err))
			os.Exit(1)
		}
		// Print to stdout too if other exports were also requested
		if exportPNG || exportBMP || exportGIF {
			fmt.Println(jsonStr)
		}
	} else {
		fmt.Println(jsonStr)
	}
}


// waitForEnter prints a closing prompt and blocks until the user presses enter.
func waitForEnter() {
	fmt.Println("\nNitroDecoder - Metadata extractor for ROMs from nintendio handhelds\nMobCat - 20260517\nPress enter to close.")
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
}
