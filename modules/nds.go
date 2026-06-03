// modules/nds.go - NDS and DSi ROM decoder
// MobCat (2026)

package modules

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
)

// ============================================================
// NDS ROM / Banner layout constants
// ============================================================
const (
	// ROM header offsets
	hdrGameTitle   = 0x000 // 12 bytes, ASCII
	hdrGameCode    = 0x00C // 4 bytes
	hdrMakerCode   = 0x010 // 2 bytes
	hdrUnitCode    = 0x012 // 1 byte
	hdrRegionLock  = 0x01D // 1 byte
	hdrRomVersion  = 0x01E // 1 byte
	hdrRomSizeCode = 0x014 // 1 byte
	hdrBannerOff   = 0x068 // 4 bytes, offset to icon/title banner

	// Banner offsets (relative to banner start)
	bannerVersion   = 0x000 // 2 bytes   = version flag
	bannerIconTiles = 0x020 // 512 bytes = 4bpp tile data (32x32, 8x8 tiles)
	bannerPalette   = 0x220 // 32 bytes  = 16 x BGR555
	bannerTitles    = 0x240 // 6 x 256 bytes UTF-16LE titles

	// DSi animated icon offsets (banner version >= 0x0103)
	// 8 frames of tile data + 8 palettes + 64 entry sequence table
	bannerAnimTiles  = 0x1240 // 8 × 512 bytes  (8 tile frames)
	bannerAnimPals   = 0x2240 // 8 × 32  bytes  (8 palettes)
	bannerAnimSeq    = 0x2340 // 64 × 2  bytes  (sequence table)
	bannerMinSizeDSi = 0x23C0 // end of anim block
)

// ============================================================
// NDS ROM json output layout
// ============================================================
type NDSROMInfo struct {
	Filename      string            `json:"filename"`
	RomDecoder    string            `json:"decoder"`
	RomID         string            `json:"rom_id"`
	InternalName  string            `json:"internal_name"`
	UnitCode      int               `json:"unit_code"`
	Prefix        string            `json:"prefix"`
	Region        string            `json:"region"`
	Serial        string            `json:"serial"`
	RomVersion    string            `json:"rom_version"`
	CRC32         string            `json:"crc32"`
	PublisherCode string            `json:"publisher_code"`
	Publisher     string            `json:"publisher"`
	LockCode      string            `json:"lock_code"`
	CartLock      string            `json:"region_lock"`
	CartCode      string            `json:"cart_code"`
	CartSizeBytes uint64            `json:"cart_size_bytes"`
	CartSize      string            `json:"cart_size"`
	CartInfo      string            `json:"cart_info"`
	Titles        map[string]string `json:"titles,omitempty"`
	Exported      map[string]string `json:"exported,omitempty"`
	Warning       string            `json:"warning,omitempty"`
	Error         string            `json:"error,omitempty"`
}

func (r *NDSROMInfo) GetRomID() string { return r.RomID }

// AppendMessage sets the warning or error field on the struct.
// If a message already exists the new one is appended with "; ".
func (r *NDSROMInfo) AppendMessage(level, msg string) {
	switch level {
	case "warning":
		if r.Warning == "" {
			r.Warning = msg
		} else {
			r.Warning += "; " + msg
		}
	case "error":
		if r.Error == "" {
			r.Error = msg
		} else {
			r.Error += "; " + msg
		}
	}
}

// ============================================================
// Decoder tables
// ============================================================

var ndsPublisher = map[string]string{
	"01": "Nintendo",
	"08": "CAPCOM",
	"18": "Hudson Entertainment, Inc.",
	"20": "Destination Software Inc.",
	"2L": "Agatsuma Entertainment Co.,Ltd.",
	"36": "Codemasters",
	"41": "Ubisoft Entertainment",
	"4F": "TT Games",
	"4Q": "Disney Interactive Studios",
	"4Z": "Crave Entertainment",
	"52": "Activision",
	"54": "2K Play",
	"5D": "Midway",
	"5G": "Majesco Entertainment",
	"5M": "Telegames, Inc",
	"5V": "agetec",
	"5Z": "Conspiracy Entertainment",
	"64": "LucasArts",
	"68": "Vir2L Studios LLC",
	"69": "Electronic Arts Inc.",
	"6K": "UFO Interactive Games",
	"6U": "DreamCatcher Interactive",
	"70": "Atari, Inc.",
	"78": "THQ Inc.",
	"7D": "Sierra Entertainment, Inc.",
	"7N": "Empire Interactive",
	"7T": "Scholastic Inc.",
	"7V": "Summitsoft Corporation",
	"7U": "Ignition Entertainment Ltd.",
	"8P": "SEGA",
	"99": "Rising Star Games",
	"9B": "TECMO",
	"A4": "Konami Digital Entertainment",
	"AF": "NAMCO LTD.",
	"B2": "BANDAI",
	"B7": "SNK PLAYMORE",
	"C8": "KOEI",
	"DA": "TOMY Corporation",
	"E9": "Natsume Inc.",
	"EB": "Atlus U.S.A., Inc.",
	"FH": "Easy Interactive",
	"FM": "Aspyr",
	"FP": "Mastiff, LLC.",
	"FS": "XS Games",
	"FX": "RED MILE ENTERTAINMENT",
	"FQ": "iQue China", //? Needs more testing
	"G0": "Alpha Unit",
	"G2": "FUTURE MEDIA CREATORS YUKE'S",
	"G9": "D3Publisher",
	"GD": "SQUARE ENIX",
	"GG": "O~3 Entertainment, Inc.",
	"GL": "gameloft",
	"GM": "Gamecock Media Group",
	"GN": "Oxygen Interactive Ltd.", //was Acclaim (Europe?)
	"GR": "Avanquest",
	"GT": "505 Games",
	"GU": "Bayer Diabetes HealthCare",
	"GY": "The Game Factory",
	"GV": "cdv Software Entertainment",
	"HF": "LEVELS",
	"HG": "Graffiti Entertainment",
	"HJ": "Genius Products, llc",
	"HS": "HES Interactive",
	"JH": "City Interactive",
	"JJ": "Deep Silver, Inc",
	"JZ": "505 Games, Inc.",
	"KA": "Alchemist",
	"LT": "Legacy Interactive",
	"M8": "PHP Research Institute", //?? No idea if this is right, only seen one one hard to google jpn game.
	"MH": "Mentor Interactive",
	"MR": "Minescale",
	"MT": "Mastertronic",
	"MJ": "MumboJumbo",
	"MV": "Marvelous Entertainment USA Inc.",
	"NR": "Destineer",
	"NS": "NIS America, Inc.",
	"NJ": "Enjoy Gaming ltd.",
	"PK": "Knowledge Adventure",
	"PL": "Playlogic/Engine Software",
	"PQ": "PopCap Games, Inc.",
	"PZ": "GameMill Entertainment",
	"QH": "Virtual Play Games",
	"RM": "Rondomedia",
	"RS": "Brash Entertainment",
	"RW": "RealArcade",
	"S5": "SOUTHPEAK GAMES",
	"SZ": "Storm City Games",
	"TR": "Tetris Online, Inc.",
	"VN": "Valcon Games",
	"VZ": "Little Orbit",
	"WR": "WB Games Inc.",
	"XE": "GMG Play",
	"XJ": "XSEED Games",
	"XS": "Aksys Games",
	"XZ": "qube Ltd",
	"Y0": "Talking Stick Games Inc.",
	"YC": "NECA",
	"YM": "Bergsala Lightweight LLC",
	"YG": "Maximum Family Games",
}

// Game region code or the game on the cart. eg NTR-ADAE-USA is E = USA
var ndsRegionMap = map[byte]string{
	'C': "CHN", //Chinese
	'D': "GER", //German. un-verified
	'E': "USA", //English (American)
	'F': "FRA", //French
	//'H': "HOL", //Dutch. un-verified
	'I': "ITA", //Italian
	'J': "JPN", //Japanese
	'K': "KOR", //Korean
	//'M': "###", //Swedish. un-verified
	//'N': "###", //Norwegian. un-verified
	'P': "EUR", //European (Multilingual)
	//'Q': "###", //Danish. un-verified
	//'R': "RUS", //Russian. un-verified
	//'S': "SPA", //Spanish. un-verified
	//'U': "AUS", //English (Australian). un-verified. because NTR-AMCE-AUS
	'X': "EUR",
	'Y': "USA",
	'V': "EUR", //English (British)
	'O': "USA",
}
// NOTE: Using the above game region code to get a packaging region code
// is not always excat. but its close.

// For region lock byte. eg cant use a Chinese iQue DS cart in a retail DS.
// TWL DSi is currently unknown.
var ndsLockMap = map[byte]string{
	0x00: "None",
	0x40: "NTR Korea",
	0x80: "NTR China",
}

var ndsInfoMap = map[byte]string{
	'A': "Common NDS cart", //NTR-005
	'B': "Common NDS cart", //NTR-005?
	'C': "Common NDS cart", //NTR-005
	'D': "DSi exclusive cart", //NTR-005(-02)?
	'H': "DSiWare (system utility)",
	'I': "NDS or DSi enhanced cart with a built-in Infrared port", //NTR-031
	'K': "DSiWare (game)",
	'N': "NDS japan nintendo channel demo",
	'T': "Less common NDS cart",
	'U': "NDS or DSi cart with uncommon extra hardware (eg. NAND, ram, microSD, TV, azimuth)",
	'V': "DSi enhanced cart",
	'Y': "Less common NDS cart", //NTR-005? late gen DS?
}
//U:
// UU: NTR-005(-04) Bluetooth receiver on the cartridge itself
// ??: NTR-016 Nintendo DS TV Reception Adapter (?? no sn on cart lable)
// UE: NTR-030 Azimuth? for Hoshizora Navi aka Starry Sky Navigator

var ndsUnitMap = map[byte]string{
	0: "NTR",
	// 1: "NTR+TWL", // Invalid? no dumped ROM has this flag
	2: "TWL",
}

// ============================================================
// Internal types
// ============================================================

// animToken holds the parsed fields of one DSi banner animation sequence entry.
type animToken struct {
	tileIdx  int
	palIdx   int
	duration int
	hFlip    bool
	vFlip    bool
}

// ============================================================
// Entry point
// ============================================================

// DecodeNDS is the main entry point for NDS/DSi ROMs.
// hdr is the already-read 512-byte header. f is the open file handle,
// seeked to an unspecified position — DecodeNDS will seek as needed.
func DecodeNDS(hdr []byte, f *os.File, romPath string, opts DecodeOptions) (*NDSROMInfo, error) {

	bannerOffset := binary.LittleEndian.Uint32(hdr[hdrBannerOff:])
	usedRomSize  := binary.LittleEndian.Uint32(hdr[0x080:])

	// Sanity: banner must be at 0x8000 or higher (spec: "8000h and up")
	if bannerOffset < 0x8000 {
		return nil, fmt.Errorf("invalid banner offset 0x%X", bannerOffset)
	}

	// --- Stage 2: read header + banner block ---
	readEnd := uint64(bannerOffset) + bannerMinSizeDSi
	if usedRomSize > 0 && uint64(usedRomSize) < readEnd {
		readEnd = uint64(usedRomSize)
	}

	if _, err := f.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("seek error: %w", err)
	}
	data := make([]byte, readEnd)
	if _, err := f.Read(data); err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	// --- CRC32 over the used ROM region ---
	crc := computeCRC32(romPath, usedRomSize)

	// --- Header fields ---
	gameTitle   := nullTermASCII(hdr[hdrGameTitle : hdrGameTitle+12])
	gameCode    := string(hdr[hdrGameCode : hdrGameCode+4])
	makerCode   := string(hdr[hdrMakerCode : hdrMakerCode+2])
	unitCode    := hdr[hdrUnitCode]
	lockCode    := hdr[hdrRegionLock]
	romVersion  := hdr[hdrRomVersion]
	romSizeCode := hdr[hdrRomSizeCode]

	regionChar := gameCode[3]
	region, ok := ndsRegionMap[regionChar]
	if !ok {
		region = "###"
	}
	prefix, ok := ndsUnitMap[unitCode]
	if !ok {
		prefix = "###"
	}
	regionLock, ok := ndsLockMap[lockCode]
	if !ok {
		regionLock = "###"
	}
	publisher, ok := ndsPublisher[makerCode]
	if !ok {
		publisher = "###"
	}

	cartSizeBytes := uint64(128*1024) << romSizeCode

	// --- Banner ---
	if int(bannerOffset)+bannerTitles > len(data) {
		return nil, fmt.Errorf("banner offset out of range")
	}
	banner := data[bannerOffset:]

	// --- Titles ---
	titles := decodeTitles(banner)

	// --- DSi animation ---
	dsi := isDSiBanner(banner)
	var animTokens []animToken
	var animLoop bool
	if dsi {
		animTokens, animLoop = parseDSiAnimTokens(banner)
	}

	// --- Exports ---
	exported, exportWarnings, err := runExports(gameCode, banner, dsi, animTokens, animLoop, opts)
	if err != nil {
		return nil, err
	}

	var exportedField map[string]string
	if len(exported) > 0 {
		exportedField = exported
	}

	info := &NDSROMInfo{
		Filename:      romPath,
		RomDecoder:    "NDS",
		RomID:         gameCode,
		InternalName:  gameTitle,
		UnitCode:      int(unitCode),
		Prefix:        prefix,
		Region:        region,
		Serial:        fmt.Sprintf("%s-%s-%s", prefix, gameCode, region),
		RomVersion:    fmt.Sprintf("%02X", romVersion),
		CRC32:         crc,
		PublisherCode: makerCode,
		Publisher:     publisher,
		LockCode:      fmt.Sprintf("%02X", lockCode),
		CartLock:      regionLock,
		CartCode:      fmt.Sprintf("%02X", romSizeCode),
		CartSizeBytes: cartSizeBytes,
		CartSize:      formatSize(cartSizeBytes),
		CartInfo:      ndsInfoMap[gameCode[0]],
		Titles:        titles,
		Exported:      exportedField,
	}
	for _, w := range exportWarnings {
		FmtError(info, "warning", w)
	}
	return info, nil
}

// ============================================================
// Export orchestration
// ============================================================

func runExports(
	gameCode string,
	banner []byte,
	dsi bool,
	animTokens []animToken,
	animLoop bool,
	opts DecodeOptions,
) (map[string]string, []string, error) {

	var warnings []string
	exported := map[string]string{}

	if !opts.DoExport {
		return exported, nil, nil
	}

	exportPNG  := false
	exportBMP  := false
	exportGIF  := false

	if len(opts.ExportVals) == 0 {
		// Smart default: always PNG; GIF only for animated DSi ROMs
		exportPNG = true
		if dsi && len(animTokens) > 1 {
			exportGIF = true
		}
	} else {
		exportPNG  = Contains(opts.ExportVals, "png")
		exportBMP  = Contains(opts.ExportVals, "bmp")
		exportGIF  = Contains(opts.ExportVals, "gif")
		// json is handled by main, not here
	}

	if exportPNG || exportBMP || exportGIF {
		if err := os.MkdirAll(opts.OutDir, 0755); err != nil {
			return nil, nil, fmt.Errorf("error creating output directory: %w", err)
		}
	}

	if exportPNG {
		path := filepath.Join(opts.OutDir, gameCode+".PNG")
		pal := decodePalette(banner, bannerPalette, true)
		img := decodeTiles(banner[bannerIconTiles:bannerIconTiles+512], pal)
		f, err := os.Create(path)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating PNG: %w", err)
		}
		encErr := png.Encode(f, img)
		f.Close()
		if encErr != nil {
			return nil, nil, fmt.Errorf("error encoding PNG: %w", encErr)
		}
		exported["PNG"] = path
	}

	if exportBMP {
		if dsi && len(animTokens) > 1 {
			padding := len(fmt.Sprintf("%d", len(animTokens)-1))
			for i, tok := range animTokens {
				tileOff := bannerAnimTiles + tok.tileIdx*512
				grid := untileToIndexGrid(banner[tileOff : tileOff+512])
				grid = applyFlips(grid, tok.hFlip, tok.vFlip)
				rawPal := banner[bannerAnimPals+tok.palIdx*32 : bannerAnimPals+tok.palIdx*32+32]
				path := filepath.Join(opts.OutDir, fmt.Sprintf("%s-%0*d.BMP", gameCode, padding, i))
				if err := encodeIndexedBMPFromGrid(path, grid, rawPal); err != nil {
					return nil, nil, fmt.Errorf("error writing BMP frame %d: %w", i, err)
				}
				exported[fmt.Sprintf("BMP-%0*d", padding, i)] = path
			}
		} else {
			path := filepath.Join(opts.OutDir, gameCode+".BMP")
			if err := encodeIndexedBMP(path, banner[bannerIconTiles:bannerIconTiles+512], banner, bannerPalette); err != nil {
				return nil, nil, fmt.Errorf("error writing BMP: %w", err)
			}
			exported["BMP"] = path
		}
	}

	if exportGIF {
		path := filepath.Join(opts.OutDir, gameCode+".GIF")
		if dsi {
			if err := encodeGIF(path, banner, animTokens, animLoop); err != nil {
				return nil, nil, fmt.Errorf("error writing GIF: %w", err)
			}
		} else {
			warnings = append(warnings, "not a DSi ROM, exporting static icon as single-frame GIF")
			if err := encodeStaticGIF(path, banner); err != nil {
				return nil, nil, fmt.Errorf("error writing GIF: %w", err)
			}
		}
		exported["GIF"] = path
	}

	return exported, warnings, nil
}

// ============================================================
// Title decoding
// ============================================================

func decodeTitles(banner []byte) map[string]string {
	langs := []string{"JPN", "ENG", "FRE", "GER", "ITA", "SPA"}
	titles := map[string]string{}
	for i, lang := range langs {
		off := bannerTitles + i*256
		if off+256 > len(banner) {
			break
		}
		titles[lang] = decodeUTF16LE(banner[off : off+256])
	}
	// Deduplicate: if all non-empty titles are identical, keep only ENG
	unique := map[string]bool{}
	for _, t := range titles {
		if t != "" {
			unique[t] = true
		}
	}
	if len(unique) == 1 {
		titles = map[string]string{"ENG": titles["ENG"]}
	}
	return titles
}

// ============================================================
// CRC32
// ============================================================

func computeCRC32(romPath string, usedRomSize uint32) string {
	cf, err := os.Open(romPath)
	if err != nil {
		return "????????"
	}
	defer cf.Close()

	var limit int64
	if usedRomSize > 0 {
		limit = int64(usedRomSize)
	} else {
		if fi, err := cf.Stat(); err == nil {
			limit = fi.Size()
		}
	}

	h := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	buf := make([]byte, 64*1024)
	var read int64
	for read < limit {
		toRead := int64(len(buf))
		if limit-read < toRead {
			toRead = limit - read
		}
		n, err := cf.Read(buf[:toRead])
		if n > 0 {
			h.Write(buf[:n])
			read += int64(n)
		}
		if err != nil {
			break
		}
	}
	return fmt.Sprintf("%08X", h.Sum32())
}

// ============================================================
// DSi banner helpers
// ============================================================

// isDSiBanner returns true when the banner has DSi animation data with at
// least one active frame.
func isDSiBanner(banner []byte) bool {
	if len(banner) < bannerMinSizeDSi {
		return false
	}
	version := binary.LittleEndian.Uint16(banner[bannerVersion:])
	if version < 0x0103 {
		return false
	}
	return banner[bannerAnimSeq] != 0 // duration==0 at slot 0 means no animation
}

// parseDSiAnimTokens reads the sequence table and returns the active token
// list plus whether the animation should loop.
func parseDSiAnimTokens(banner []byte) (tokens []animToken, loop bool) {
	loop = true
	for i := 0; i < 64; i++ {
		entry := binary.LittleEndian.Uint16(banner[bannerAnimSeq+i*2:])
		tileIdx, palIdx, duration, hFlip, vFlip := decodeAnimSeqEntry(entry)
		if duration == 0 {
			controlByte := uint8(entry >> 8)
			loop = controlByte != 1
			break
		}
		tokens = append(tokens, animToken{tileIdx, palIdx, duration, hFlip, vFlip})
	}
	return
}

// decodeAnimSeqEntry unpacks a 16-bit DSi animation sequence entry.
//
//	bits  0-7  : frame duration in 1/60 s units (0 = control/end frame)
//	bits  8-10 : tile frame index (0-7)
//	bits 11-13 : palette index (0-7)
//	bit  14    : flip horizontal
//	bit  15    : flip vertical
func decodeAnimSeqEntry(entry uint16) (tileIdx, palIdx, duration int, flipH, flipV bool) {
	duration = int(entry & 0xFF)
	tileIdx  = int((entry >> 8) & 0x07)
	palIdx   = int((entry >> 11) & 0x07)
	flipH    = entry&(1<<14) != 0
	flipV    = entry&(1<<15) != 0
	return
}

// ============================================================
// Image helpers
// ============================================================

// decodeBGR555 converts a 15-bit BGR555 word to RGBA.
// index 0 is always transparent when applyTransparency is true.
func decodeBGR555(c uint16, index int, applyTransparency bool) color.RGBA {
	r := uint8((c & 0x1F) << 3)
	g := uint8(((c >> 5) & 0x1F) << 3)
	b := uint8(((c >> 10) & 0x1F) << 3)
	a := uint8(255)
	if applyTransparency && index == 0 {
		a = 0
	}
	return color.RGBA{r, g, b, a}
}

// bgr555ToColor converts a BGR555 word to color.RGBA (index 0 → transparent).
func bgr555ToColor(c uint16, idx int) color.RGBA {
	return decodeBGR555(c, idx, true)
}

// decodePalette reads 16 BGR555 entries from data at offset.
func decodePalette(data []byte, offset int, applyTransparency bool) [16]color.RGBA {
	var pal [16]color.RGBA
	for i := 0; i < 16; i++ {
		c := binary.LittleEndian.Uint16(data[offset+i*2:])
		pal[i] = decodeBGR555(c, i, applyTransparency)
	}
	return pal
}

// decodeTiles renders a 32×32 RGBA image from 4bpp NDS tile data + palette.
func decodeTiles(tiles []byte, pal [16]color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for ty := 0; ty < 4; ty++ {
		for tx := 0; tx < 4; tx++ {
			base := (ty*4 + tx) * 32
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					b := tiles[base+y*4+x/2]
					var ci byte
					if x&1 == 1 {
						ci = b >> 4
					} else {
						ci = b & 0x0F
					}
					img.SetRGBA(tx*8+x, ty*8+y, pal[ci])
				}
			}
		}
	}
	return img
}

// untileToIndexGrid converts 512 bytes of 4bpp NDS tile data into a flat
// 32×32 index grid (one byte per pixel, values 0–15).
func untileToIndexGrid(tiles []byte) [32 * 32]byte {
	var grid [32 * 32]byte
	for ty := 0; ty < 4; ty++ {
		for tx := 0; tx < 4; tx++ {
			base := (ty*4 + tx) * 32
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					b := tiles[base+y*4+x/2]
					var ci byte
					if x&1 == 1 {
						ci = b >> 4
					} else {
						ci = b & 0x0F
					}
					grid[(ty*8+y)*32+(tx*8+x)] = ci
				}
			}
		}
	}
	return grid
}

// applyFlips returns a new index grid with horizontal and/or vertical flip applied.
func applyFlips(grid [32 * 32]byte, hFlip, vFlip bool) [32 * 32]byte {
	if !hFlip && !vFlip {
		return grid
	}
	var out [32 * 32]byte
	for y := 0; y < 32; y++ {
		srcY := y
		if vFlip {
			srcY = 31 - y
		}
		for x := 0; x < 32; x++ {
			srcX := x
			if hFlip {
				srcX = 31 - x
			}
			out[y*32+x] = grid[srcY*32+srcX]
		}
	}
	return out
}

// ============================================================
// GIF encoder
// ============================================================

func buildPalettedFrame(banner []byte, tok animToken) *image.Paletted {
	tileOff := bannerAnimTiles + tok.tileIdx*512
	grid := untileToIndexGrid(banner[tileOff : tileOff+512])
	grid = applyFlips(grid, tok.hFlip, tok.vFlip)

	off := bannerAnimPals + tok.palIdx*32
	pal := make(color.Palette, 16)
	for i := 0; i < 16; i++ {
		c := binary.LittleEndian.Uint16(banner[off+i*2:])
		pal[i] = bgr555ToColor(c, i)
	}

	img := image.NewPaletted(image.Rect(0, 0, 32, 32), pal)
	copy(img.Pix, grid[:])
	return img
}

func encodeGIF(path string, banner []byte, tokens []animToken, loop bool) error {
	g := &gif.GIF{}
	if loop {
		g.LoopCount = 0
	} else {
		g.LoopCount = 1
	}
	for _, tok := range tokens {
		frame := buildPalettedFrame(banner, tok)
		delayCentisec := (tok.duration*100 + 30) / 60
		if delayCentisec < 2 {
			delayCentisec = 2
		}
		g.Image    = append(g.Image, frame)
		g.Delay    = append(g.Delay, delayCentisec)
		g.Disposal = append(g.Disposal, gif.DisposalBackground)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return gif.EncodeAll(f, g)
}

func encodeStaticGIF(path string, banner []byte) error {
	pal := make(color.Palette, 16)
	for i := 0; i < 16; i++ {
		c := binary.LittleEndian.Uint16(banner[bannerPalette+i*2:])
		pal[i] = bgr555ToColor(c, i)
	}
	grid := untileToIndexGrid(banner[bannerIconTiles : bannerIconTiles+512])
	img := image.NewPaletted(image.Rect(0, 0, 32, 32), pal)
	copy(img.Pix, grid[:])

	g := &gif.GIF{
		Image:     []*image.Paletted{img},
		Delay:     []int{100},
		Disposal:  []byte{gif.DisposalBackground},
		LoopCount: 1,
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return gif.EncodeAll(f, g)
}

// ============================================================
// BMP encoder. 4bpp indexed, 32x32
//
// Format: BITMAPFILEHEADER (14) + BITMAPINFOHEADER (40) +
//         16-colour RGBQUAD table (64) + 4bpp pixel rows bottom-up.
// ============================================================

func encodeIndexedBMP(path string, tiles []byte, bannerData []byte, palOffset int) error {
	const (
		width    = 32
		height   = 32
		stride   = width / 2
		pixBytes = stride * height
		palBytes = 16 * 4
		infoSize = 40
		fileSize = 14 + infoSize + palBytes + pixBytes
		dataOff  = 14 + infoSize + palBytes
	)

	buf := make([]byte, fileSize)

	copy(buf[0:], []byte{'B', 'M'})
	binary.LittleEndian.PutUint32(buf[2:], uint32(fileSize))
	binary.LittleEndian.PutUint32(buf[10:], uint32(dataOff))

	bih := buf[14:]
	binary.LittleEndian.PutUint32(bih[0:], uint32(infoSize))
	binary.LittleEndian.PutUint32(bih[4:], uint32(int32(width)))
	binary.LittleEndian.PutUint32(bih[8:], uint32(int32(height)))
	binary.LittleEndian.PutUint16(bih[12:], 1)
	binary.LittleEndian.PutUint16(bih[14:], 4)
	binary.LittleEndian.PutUint32(bih[32:], 16)
	binary.LittleEndian.PutUint32(bih[36:], 16)

	palDst := buf[14+infoSize:]
	for i := 0; i < 16; i++ {
		c := binary.LittleEndian.Uint16(bannerData[palOffset+i*2:])
		r := uint8((c & 0x1F) << 3)
		g := uint8(((c >> 5) & 0x1F) << 3)
		b := uint8(((c >> 10) & 0x1F) << 3)
		palDst[i*4+0] = b
		palDst[i*4+1] = g
		palDst[i*4+2] = r
		palDst[i*4+3] = 0
	}

	linear := make([]byte, pixBytes)
	for ty := 0; ty < 4; ty++ {
		for tx := 0; tx < 4; tx++ {
			base := (ty*4 + tx) * 32
			for y := 0; y < 8; y++ {
				dstRow      := ty*8 + y
				dstByteBase := dstRow*stride + tx*4
				for x := 0; x < 4; x++ {
					nb := tiles[base+y*4+x]
					linear[dstByteBase+x] = (nb << 4) | (nb >> 4)
				}
			}
		}
	}

	pixDst := buf[dataOff:]
	for row := 0; row < height; row++ {
		srcRow := height - 1 - row
		copy(pixDst[row*stride:], linear[srcRow*stride:(srcRow+1)*stride])
	}

	return os.WriteFile(path, buf, 0644)
}

func encodeIndexedBMPFromGrid(path string, grid [32 * 32]byte, rawPal []byte) error {
	const (
		width    = 32
		height   = 32
		stride   = width / 2
		pixBytes = stride * height
		palBytes = 16 * 4
		infoSize = 40
		fileSize = 14 + infoSize + palBytes + pixBytes
		dataOff  = 14 + infoSize + palBytes
	)

	buf := make([]byte, fileSize)

	copy(buf[0:], []byte{'B', 'M'})
	binary.LittleEndian.PutUint32(buf[2:], uint32(fileSize))
	binary.LittleEndian.PutUint32(buf[10:], uint32(dataOff))

	bih := buf[14:]
	binary.LittleEndian.PutUint32(bih[0:], uint32(infoSize))
	binary.LittleEndian.PutUint32(bih[4:], uint32(int32(width)))
	binary.LittleEndian.PutUint32(bih[8:], uint32(int32(height)))
	binary.LittleEndian.PutUint16(bih[12:], 1)
	binary.LittleEndian.PutUint16(bih[14:], 4)
	binary.LittleEndian.PutUint32(bih[32:], 16)
	binary.LittleEndian.PutUint32(bih[36:], 16)

	palDst := buf[14+infoSize:]
	for i := 0; i < 16; i++ {
		c := binary.LittleEndian.Uint16(rawPal[i*2:])
		r := uint8((c & 0x1F) << 3)
		g := uint8(((c >> 5) & 0x1F) << 3)
		b := uint8(((c >> 10) & 0x1F) << 3)
		palDst[i*4+0] = b
		palDst[i*4+1] = g
		palDst[i*4+2] = r
		palDst[i*4+3] = 0
	}

	pixDst := buf[dataOff:]
	for row := 0; row < height; row++ {
		srcRow := height - 1 - row
		for col := 0; col < stride; col++ {
			lo := grid[srcRow*32+col*2]
			hi := grid[srcRow*32+col*2+1]
			pixDst[row*stride+col] = (lo << 4) | hi
		}
	}

	return os.WriteFile(path, buf, 0644)
}
