// modules/agb.go - Game Boy Advance ROM decoder
// MobCat (2026)

package modules

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"bytes" //noodles around in your file, by hand, one byte at a time. nomnomnom
)

// ============================================================
// GBA ROM header layout constants
// ============================================================
const (
	agbEntryPoint    = 0x000 // 4 bytes   - ARM branch instruction
	agbNintendoLogo  = 0x004 // 156 bytes - Nintendo logo bitmap
	agbGameTitle     = 0x0A0 // 12 bytes  - uppercase ASCII, null-padded
	agbGameCode      = 0x0AC // 4 bytes   - uppercase ASCII
	agbMakerCode     = 0x0B0 // 2 bytes   - uppercase ASCII
	agbFixedValue    = 0x0B2 // 1 byte    - must be 0x96
	agbUnitCode      = 0x0B3 // 1 byte    - 0x00 for GBA
	agbDeviceType    = 0x0B4 // 1 byte    - usually 0x00
	agbSoftwareVer   = 0x0BC // 1 byte    - usually 0x00
	agbChecksum      = 0x0BD // 1 byte    - header complement checksum
	agbUnknowOp      = 0x100 // 8 bytes   - 
	agbAltTitle      = 0x108 // 32 bytes  - some games like pokemon have alt titles here.
)

// ============================================================
// GBA ROM json output layout
// ============================================================
type AGBROMInfo struct {
	Filename      string            `json:"filename"`
	RomDecoder    string            `json:"decoder"`
	RomID         string            `json:"rom_id"`
	InternalName  string            `json:"internal_name"`
	AltTitle      string            `json:"alt_title,omitempty"`
	UnitCode      int               `json:"unit_code"`
	Prefix        string            `json:"prefix"`
	Region        string            `json:"region"`
	Serial        string            `json:"serial"`
	RomVersion    string            `json:"rom_version"`
	CRC32         string            `json:"crc32"`
	PublisherCode string            `json:"publisher_code"`
	Publisher     string            `json:"publisher"`
	DACSCode      string            `json:"dacs_code"`
	DACSInfo      string            `json:"dacs_info"`
	CartSizeBytes uint64            `json:"rom_size_bytes"`
	CartSize      string            `json:"rom_size"`
	Exported      map[string]string `json:"exported,omitempty"`
	Warning       string            `json:"warning,omitempty"`
	Error         string            `json:"error,omitempty"`
}

func (r *AGBROMInfo) GetRomID() string { return r.RomID }

// AppendMessage sets the warning or error field on the struct.
// If a message already exists the new one is appended with "; ".
func (r *AGBROMInfo) AppendMessage(level, msg string) {
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

// GBA maker codes are 2 ASCII characters stored directly in the header,
// eg hex 30 31 = chars "01" = Nintendo, hex 37 4A = chars "7J" = ZOO Digital.
var agbPublisher = map[string]string{
	"00": "Unlicensed Homebrew", //unofficial ofc. But if homebrew build there rom correctly, we should return valid data correctly.
	//"02": "", /Castle Master (World) (Unl).gba
	"01": "Nintendo",
	"08": "Capcom",
	"09": "Hot-B",
	"0A": "Jaleco",
	"0B": "Coconuts Japan",
	"0C": "Elite Systems",
	"0H": "Starfish",
	"0M": "ESP Software",
	"0N": "NowPro", //Unconfirmed
	"0P": "Namco",
	"0Q": "Imagineer",
	"13": "EA (Electronic Arts)",
	"18": "Hudsonsoft",
	"19": "ITC Entertainment",
	"1A": "Yanoman",
	"1D": "Japan Clary",
	"1F": "Virgin Interactive",
	"1G": "Shogakukan Production Co., Ltd. ",
	"1Q": "TDK Core",
	"20": "Destination Software Inc.",
	"24": "PCM Complete",
	"25": "San-X",
	"28": "Kotobuki Systems",
	"29": "Seta",
	"2H": "Ubisoft", //Japan only?
	"2K": "NEC Interchannel",
	"2L": "Tamsoft",
	"2N": "Rocket Company", //Weird.. needs more checks.. (4)Rocket Company (1)Smilesoft
	"2M": "jondan co ltd", //Unconfried
	"2Q": "MediaKite",
	"30": "Infogrames",
	"31": "Nintendo",
	"32": "Bandai",
	"34": "Konami",
	"35": "HectorSoft",
	"36": "Codemasters",
	"38": "Capcom",
	"39": "Banpresto",
	"3C": ".Entertainment i",
	"3E": "Gremlin",
	"41": "Ubisoft", //Konami?
	"42": "Atlus",
	"44": "Malibu",
	"46": "Angel",
	"47": "Spectrum Holoby",
	"49": "Irem",
	"4A": "Virgin Interactive",
	"4D": "Malibu",
	"4F": "Eidos Interactive",
	"4Q": "Disney Interactive Studios",
	"4Z": "Crave Entertainment",
	"50": "Absolute",
	"51": "Acclaim",
	"52": "Activision",
	"53": "American Sammy",
	"54": "Take-Two Interactive",
	"55": "Park Place",
	"56": "LJN",
	"57": "Matchbox",
	"59": "Milton Bradley",
	"5A": "Mindscape",
	"5B": "Romstar",
	"5C": "Naxat Soft",
	"5D": "Midway",
	"5G": "Majesco Entertainment",
	"5H": "3DO", //The 3DO Company.
	"5M": "Telegames",
	"5N": "Metro3D",
	"5L": "Vivendi Universal Games", //?
	"5Q": "Lego Software", // Lego Interactive (formerly Lego Media and later Lego Software) 
	"5S": "XICAT Entertainment",
	"5T": "DreamCatcher Interactive",
	"5X": "Microids", //please valadate
	"5Z": "Conspiracy Entertainment",
	"60": "Titus",
	"61": "Virgin Interactive",
	"64": "LucasArts",
	"67": "Ocean Interactive",
	"69": "EA (Electronic Arts)",
	"6E": "Elite Systems",
	"6F": "Electro Brain",
	"6H": "BBC Multimedia",
	"6L": "BAM! Entertainment",
	"6M": "Studio 3 Interactive Entertainment Ltd.",
	"6S": "TDK Mediactive",
	"6U": "DreamCatcher Interactive",
	"6V": "JoWooD",
	"6W": "SEGA", //USA only?
	"6X": "Wanadoo",
	"6Y": "Hip Games", //again? or LSP?
	"6Z": "ITE Media",
	"70": "Infogrames",
	"71": "Interplay",
	"72": "Broderbund",
	"73": "Sculptered Soft",
	"75": "The Sales Curve",
	"77": "Pocket Pulp LLC.", //unofficial aftermarket publisher. However they set the data corecly, and it dosent conflict yet. so its in.
	"78": "THQ",
	"79": "Accolade",
	"7A": "Triffix Entertainment",
	"7C": "Microprose",
	"7D": "Vivendi Universal Games", //or? 	Universal Interactive Studios. depends on who owns crash bandicoot at the time.
	"7F": "Kemco",
	"7G": "RAGE", //"4040 Entertainment", "Majesco", //??? dev vs publisher. needs more checks.
	"7H": "Encore Software",
	"7J": "ZOO Digital Production Ltd.",
	"7K": "Kiddinx",
	"7L": "Hypnotix",
	"7Q": "Jester Interactive",
	"7S": "Rockstar Games",
	"7T": "Scholastic, Inc.",
	"7U": "Ignition Entertainment Ltd.",
	"7V": "Summitsoft Entertainment",
	"7W": "Stadlbauer",
	"7N": "Empire Interactive",
	"7M": "Ignition Entertainment Ltd.", //USA only?
	"80": "Misawa Entertainment",
	"83": "Lozc",
	"86": "Tokuma Shoten Intermedia",
	"8B": "Bullet-Proof Software",
	"8C": "Vic Tokai",
	"8E": "Ape",
	"8F": "I'Max",
	"8J": "Kadokawa Shoten",
	"8N": "Success",
	"8P": "THQ Europe", //? Sometimes sega?
	"91": "Chunksoft Co.",
	"92": "Video System",
	"93": "Tsubaraya Productions Co.",
	"95": "Varie Corporation",
	"96": "Yonezawa/S'Pal",
	"97": "Kaneko",
	"99": "Arc",
	"9A": "Nihon Bussan",
	"9B": "Tecmo",
	"9C": "Imagineer",
	"9D": "Banpresto",
	"9F": "Nova",
	"9G": "Gotham Games", //DBOE is Global Star aka T2 so needs more checking.
	"9N": "Marvelous Entertainment", //Dev not publisher, found on ALHJ needs more testing.
	"9P": "KEYNET",
	"9Q": "Hands-On Entertainment",
	"A0": "Nippon Telenet",
	"A1": "Hori Electric",
	"A2": "Bandai",
	"A4": "Konami",
	"A6": "Kawada",
	"A7": "Takara",
	"A9": "Technos Japan",
	"AA": "Broderbund",
	"AC": "Toei Animation",
	"AD": "Toho",
	"AF": "Namco",
	"AG": "Media Rings",
	"AH": "J-Wing",
	"AQ": "Ludic",
	"B0": "Acclaim",
	"B1": "ASCII or Nexsoft",
	"B2": "Bandai",
	"B4": "Square Enix",
	"B6": "HAL Laboratory",
	"B7": "SNK",
	"B9": "Pony Canyon",
	"BA": "Culture Brain",
	"BB": "Sunsoft",
	"BD": "Sony Imagesoft",
	"BF": "Sammy",
	"BL": "MTO Inc.",
	"BJ": "Compile", //unconfrimed
	"BM": "Bitmap Bureau", //unofficial aftermarket publisher(dev). However they set the data corecly, and it dosent conflict yet. so its in.
	"BN": "Sunrise Interactive",
	"BP": "Global A Entertainment Inc.", //might also be RAGE jpn? Denki is the dev
	"BQ": "Fuuki Co, Ltd.", 
	"C0": "Taito",
	"C2": "Kemco",
	"C3": "Squaresoft",
	"C4": "Tokuma Shoten Intermedia",
	"C5": "Data East",
	"C6": "Tonkinhouse",
	"C8": "Koei",
	"C9": "UFL",
	"CA": "Ultra",
	"CB": "Vap",
	"CC": "Use Corporation",
	"CD": "Meldac",
	"CE": "Pony Canyon",
	"CF": "Angel",
	"CM": "Konami", //japan only?
	"CN": "Omega Micott", //NEC Interchannel?
	"CP": "Enterbrain",
	"D0": "Taito",
	"D1": "Sofel",
	"D2": "Quest",
	"D3": "Sigma Enterprises",
	"D4": "ASK Kodansha Co.",
	"D6": "Naxat Soft", // //Atari SA //Naxat Soft //? not sure if dev not pub
	"D7": "Copya System",
	"D9": "Banpresto Co., Ltd.", //Atari SA in USA? 
	"DA": "Tomy",
	"DB": "LJN",
	"DD": "NCS",
	"DE": "Human",
	"DF": "Altron",
	"DL": "Digital Kids",
	"E0": "Jaleco",
	"E1": "Towa Chiki",
	"E2": "Yutaka",
	"E3": "Varie",
	"E5": "Epoch",
	"E7": "Athena",
	"E8": "Asmik",
	"E9": "Natsume",
	"EA": "King Records",
	"EB": "Atlus",
	"EC": "Epic/Sony Records",
	"EE": "IGS",
	"EL": "Spike Co., Ltd.",
	"EM": "Konami", //Japan only
	"EP": "Sting",
	"ES": "Disney Interactive Studios",
	"F0": "A Wave",
	"F3": "Extreme Entertainment",
	"FF": "LJN",
	"FK": "InterActive Vision", //Unconfirmed / unknown. europe only. self published?
	"FJ": "Virtual Toys",
	"FL": "Hip Games",
	"FM": "Aspyr",
	"FN": "Pocket Direct, L.L.C.",
	"FP": "Mastiff",
	"FR": "dtp young entertainment GmbH & Co. KR", //Needs valadating.
	"FS": "XS Games",
	"FT": "Daiwon C&A",
	"FQ": "iQue China", //? Needs more testing
	"G0": "Alpha Unit",
	"G1": "Pacific Century Cyber Works",
	"G2": "Yuke's Co. Ltd.",
	"G4": "Mig Entertainment",
	"G5": "Atmark",
	"G6": "SIMS Co., Ltd.",
	"G7": "BROCCOLI Co., Ltd.",
	"G8": "AVEX MODE",
	"G9": "D3Publisher",
	"GB": "Konami", //Konami again? maybe just japaenese only Konami?
	"GD": "Square Enix",
	"GE": "Kids Station",
	"GG": "O3 Entertainment",
	"GF": "Micott & Basara",
	"GH": "Orbital Media, Inc.",
	"GT": "505 Games",
	"GU": "Guidance Interactive Healthcare",
	"GN": "Acclaim",
	"GY": "The Game Factory",
	"H1": "Treasure",
	"H2": "Aruze Corp.",
	"H3": "Ertain",
	"H4": "SNK Playmore",
	"IN": "In-Cubus Inc. (Unlicensed Homebrew)", //AGB-ADVN
	"KM": "Koch Media",
	"KR": "Krea Medie",
	"LH": "East Entertainment Media",
	"MA": "MAKOTO", //dev of Hoshi no Kirby - Kagami no Daimeikyuu Kiosk demo
	"MN": "Mindscape", //netherlands game with little to know info on english web (BQTX) (BQVX)
	"NK": "Neko Entertainment",
	"NR": "Bold Games",
	//"T1": "Edge Interact", //dev not pub?
	"RM": "Remute", //unofficial aftermarket dev (music). However they set the data corecly, and it dosent conflict yet. so its in.
	"S5": "SouthPeak Interactive",
	"TK": "Tasuke",
	"VN": "Valcon Games",
	"WR": "Warner Bros. Interactive",
	//"XX": "Metro3D prototype == 5N"
}

// GBA region is encoded in the 4th character of the game code,
var agbRegionMap = map[byte]string{
	'0': "UNL", // unlicened but the bit was set corecly to 0 not null
	//'A' "###" (B85A:### B85P:EUR B85E:USA so might be JPN)
	//'B': "###", // B for beta? All-Star Baseball 2004 ASNB
	'C': "CHN", // unconfirmed iQue china
	'J': "JPN",
	'K': "KOR", // ageing cart is K but is actualy 1 so needs more tests.
	'E': "USA",
	'H': "HOL", //Netherlands/Holland BDZH 
	'P': "EUR", //        
	'Q': "DAN", //BQ9Q(DAN)
	'D': "GER",
	'F': "FRE",
	'I': "ITA",
	'S': "SPA",
	'X': "EUR",
	'U': "AUS",
	//all X, Y and Z have been moved to the below dict. its a mess and alawys difrent.  
}

//Not a fan of datasets like this, we should just be abale to dig around deaper into the rom to find this info but okie.
var agbAdvRegionCheck = map[string]string{
	"B85A": "JPN",
	"ADVN": "EUR", //Adventures of Mr. Bean demo hack leak thingy.
	"BE8P": "EUR",
	"ABTP": "UKV",
	"AN3P": "UKV",
	"ALXP": "UKV",
	"AZIP": "UKV",
	"BFEP": "UKV",
	"BB2P": "UKV",
	"BDZP": "UKV",
	"AMXP": "UKV",
	"BCAP": "AUS",
	"BFWP": "UKV",
	"BFWX": "FAH", 
	"AE7X": "EUR",
	"BZIX": "EUR", 
	"AZIX": "EUR",
	"BINY": "ITA", //EUU?
	"BFWY": "GER", //NOE? Just EUU may be right for this
	"BCAY": "EUR",
	"BM7Y": "EUU",
	"ACTY": "EUU",
	"B82Y": "EUU",
	"AZIY": "EUU", //Gerrman cart
	"AE7Y": "EUR",
	"AMXY": "EUU",
	"BZIY": "FRA", //UKV?
	"ABYY": "EUU",
	"BINZ": "SPA",
	"BFWZ": "EUR",
	"BCAZ": "SCN",
}


//AGBJ may be a debug flag, not a real region.
//SV3D is an unlicened game. 
//but I really dont wanna but a struct for this. we should just firgerprint and procuess other parts of the rom to figuer out debug or unliced and fix the region code then.

// AFAIK this is always 0, but if its not.
// then I have already setup a map for you future MobCat
// your welcome <3

//Thanks past mobcat. we found one.
//0x00 is nomral
//0x01 Moorhuhn Jagd (Europe) (Proto).gba
var agbUnitMap = map[byte]string{
	0: "AGB",
}

// ============================================================
// Entry point
// ============================================================

// DecodeAGB decodes a GBA ROM header.
// hdr is the already-read 512-byte header. f and romPath are passed through
// for CRC and any future export needs; f is not read again here.
func DecodeAGB(hdr []byte, f *os.File, romPath string, opts DecodeOptions) (*AGBROMInfo, error) {

	// Validate fixed byte. Must be 0x96
	if hdr[agbFixedValue] != 0x96 {
		return nil, fmt.Errorf("fixed value byte at 0xB2 is 0x%02X, expected 0x96. This may not be a retail ROM, Run NitroValadator to check and fix this.", hdr[agbFixedValue])
	}

	// Validate header checksum
	//TODO: Build a list of these games and see if we can decode the header anyways. turn this into a warning not an error.
	//cos a lot of prototypes just have it calulated wrong? this could be we are doing the math wrong or somehting else?
	if computed, stored, ok := agbVerifyChecksum(hdr); !ok {
		return nil, fmt.Errorf("header checksum failed. 0x%02X != 0x%02X. Run NitroValadator to check and fix this ROM", computed, stored)
	}

	gameTitle   := nullTermASCII(hdr[agbGameTitle : agbGameTitle+12])
	altTitle    := nullTermASCII(hdr[agbAltTitle  : agbAltTitle+32])
	gameCode    := string(hdr[agbGameCode  : agbGameCode+4])
	makerCode   := string(hdr[agbMakerCode : agbMakerCode+2]) // 2 ASCII chars eg "01", "7J"
	softwareVer := hdr[agbSoftwareVer]

	// Debug DACS check
	// Debugging And Communication System
	// Check for extra flash above the rom data.
	DACSdebug := "Retail"
	//DACSdebug := "Retail/No extra flash"
	DACScode  := fmt.Sprintf("%02X", hdr[0xB4])
	if hdr[0xB4] != 0x00 {
	    switch hdr[0xB4] {
	    case 0x80:
	        DACSdebug = "0x9FE2000/1MBIT"
	    case 0x04:
	        DACSdebug = "0x9FFC000/8MBIT"
	    case 0xA5:
	        DACSdebug = "FIQ/Undefined handler unlocked"
	    default:
	        DACSdebug = "Unknown DACS flag"
	    }
	}

	//valadator checks. check if 0xBE and BF are nulled. if not, coudl be homebrew of some kind or a flashcart tag.


	// GBA ROM size is not stored in the header. Use actual file size
	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("could not stat ROM file: %w", err)
	}
	romSize := uint64(fi.Size())

	// CRC over the whole file (GBA has no "used size" field)
	crc := agbComputeCRC32(romPath, romSize)


	//TODO: check gameCode and makerCode for \u0000
	//This works. but we need to somehow keep the nulled code in the rom_id but dont just set it to 0000
	//as other roms use ascii 0000 as a "valid" id.
	if gameCode == "\u0000\u0000\u0000\u0000" {
		gameCode = "nul#"
	} 
	
	var region string
	var ok bool
	regionChar := gameCode[3]

	region, ok = agbAdvRegionCheck[gameCode]
	if !ok {
	    region, ok = agbRegionMap[regionChar]
	}

	var regionWarning string
	if !ok {
		if regionChar == '#' { //This check is fucked. anthing thats lowercase is invalied, but we are clashing into our null check.
			regionWarning = fmt.Sprintf("Invalid region code. rom_id may be nulled.")
		} else if regionChar >= 'a' && regionChar <= 'z' {
			regionWarning = fmt.Sprintf("Invalid region code. ROM may be Unlicensed or Homebrew.")
		} else {
			regionWarning = fmt.Sprintf("Unknown region code %c", regionChar)
		}
		
		region = "###"
	}

	prefix, ok := agbUnitMap[hdr[agbUnitCode]]
	var prefixWarning string
	if !ok {
		prefix = "###"
		prefixWarning = fmt.Sprintf("Unknown unit code 0x%02X", hdr[agbUnitCode])
	}

	makerName, ok := agbPublisher[makerCode]
	var publisherWarning string
	if !ok {
		makerName = "###"
		publisherWarning = fmt.Sprintf("Unknown publisher code %s", makerCode)
	}
	if makerCode == "\u0000\u0000" {
		makerCode = "n#"
	}

	//Hotfix
	//Kinda dumb to make a check for one game. but gonna just dump this here for now to close the warning report
	if gameCode == "XXXX" && gameTitle == "AEROXXXXXXXX" {
		makerName = "Metro3D (Prototype)"
	}

	//TODO: build that check for alt tiles found in pokemon games.
	//if altTitle does not contain ascii. null altTitle so omitempty works.
	//altTitle    := nullTermASCII(hdr[agbAltTitle  : agbAltTitle+32])
	//This check is kinda trash and really we should check for a string in the hole header
	//and or check for thoes op codes befor the title.
	for _, b := range altTitle {
	    if b < ' ' || b > '~' {
	        altTitle = ""
	        break
	    }
	}
	if len(altTitle) <= 6 {
		altTitle = ""
	}

	//Unlicned and or homebrew rom checks
	//TODO: move this to a func so we hit a check and return early. we dont need a stupid if else fall down stairs check.
	//A lot of unl games copy or edit a legit gba header so the publsiher codes are normally set to nintendio. with is valid, but not corect.
	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	//YJencrypted roms
	//'All of Sintax's GBA bootlegs are encrypted with a tool called "YJencryption", which made them extremely hard to dump and emulate'
	//One way to dump them was to just run a overdump to a full 32MB to get around the weird bank switching.
	//https://bootleggames.fandom.com/wiki/Sintax
	if gameCode == "nul#" && romSize == 33554432 && gameTitle == "YJencrypted" && makerCode == "01" {
		region = "UNL"
		makerName = "Sintax Technology Co., Ltd"
	}

	//Invalied or nulled data check.
	//TODO: This is flaging for detail games like action replay. so maybe we could do more checks here and fix makerName for detal?
	if gameCode == "nul#" && region == "###" && makerCode == "01" {
		region = "UNL"
		makerName = "###"
	}

	// Double-branch signature at 0xC0: indicates DevKitAdvance gba.ld artifact?
	// Startup code placed at 0xE4 with a reserved as null 28-byte vector cave [0xC8:0xE3].
	// Seen in: licensed games from devs / pub codes (Sirius/GY, InterActive Vision/FK, Summitsoft Entertainment/7V), and a few homebrew and unlicensed ROMs.
	// for eg Anguna Warriors of Virtue.
	// Root cause is likely a specific DevKitAdvance linker script version or a leaked custom sdk from the above devs.
	// Compiler fingerprint: arm-agb-elf-gcc (DevKitAdvance) vs arm-none-eabi-gcc (devkitARM)
	// So the UNL tag is not 100% corect here, hence also checking for a valid lookup for pub code.
	// More analysis will be in NitroValidator as to not bloat NitroDecoder.
	if bytes.Equal(hdr[0xC0:0xC4], []byte{0x06, 0x00, 0x00, 0xEA}) && makerName == "###" {
		 if bytes.Equal(hdr[0xC4:0xDF], bytes.Repeat([]byte{0x00}, 0xDF-0xC4)) {
		 	region = "UNL"
		 	makerName = "Unlicensed Homebrew"
		 	//romCompiler = "arm-agb-elf-gcc"

		 }
	}
	//Summitsoft Entertainment dev'ed for InterActive Vision (BBHE)
	//(BCGE) (BCGP) ?

	//TODO: it be neat if we can do some sort of 'if detal check' for the action replay carts.

	//TODO: I wonder if we can do a if dumped from Virtual Console check. but if its a valid gba cart then probs outside of the scope for NitroValadator

	//TODO: Dog Trainer and Dog Trainer 2 DS Cheat Cartridges from detal contain a double branch
	//But contain some other junk we can fingerprint off
	//if bytes.Equal(hdr[0xBD:0xBF], []byte{0xF0, 0x00, 0x00}) &&                  //complement check
	//   bytes.Equal(hdr[0xC0:0xC4], []byte{0x36, 0x00, 0xEA}) &&                  //Double branch
	//   bytes.Equal(hdr[0x190:0x19B], bytes.Repeat([]byte{0x00}, 0x190-0x19B)) && //null check
	//   bytes.Equal(hdr[0x19c:0x19f], []byte{0xF0, 0xFF, 0x00, 0x00})             //secend complent check?
	//       makerName = "Detal"

	//TODO: GBAMP flash cart or hack of some kind?
	//"GBAMP (GBA Movie Player v2 from movieadvance), flashed with Chishm's firmware v2."
	//https://www.gamebrew.org/wiki/GBAMP_Multiboot
	//GBA Movie Player - 2nd Version (World) (Unl) [b].gba
	//if 0xA0:0xAF == 47 42 41 4D 50 20 48 61 63 6B 00 00 50 41 53 53 (GBAMP Hack\u0000\u0000PASS)
	//The source of this is
	//GBA Movie Player - 2nd Version (World) (V2.00) (Unl).gba
	//if 0xA0:0xAF == 53 55 50 45 52 20 4D 41 52 49 4F 41 41 4D 41 4A (SUPER MARIOAAMAJ)
	//In both cases
	//0xC0:0xCF == D2 00 A0 E3 00 F0 21 E1 64 D0 9F E5 D3 00 A0 E3
	//The roms dont really differ untill 0x1300 but we can check the instructon at 0x120 befor the branch at 0x124 to 0x164
	//if 0x120 == 24 D0 9F E5 is real (ldr sp, [pc, #0x24]) Load Register
	//if 0x120 == 00 00 A0 E1 is hack (mov r0, r0)          Move r0 to its self?
	//
	//if 0x118 == 50 00 A0 E3 is real (mov r0, #0x50)
	//if 0x118 == 5F 00 A0 E3 is hack (mov r0, #0x5f)

	//TODO: GB-A TV Tuner PAL (China) (v1.3) (Unl).gba
	//"serial": "AGB-tvap-###",
	//No idea who makes this thingy yet.
	//difrent one, same idea.
	//"filename": "GBA AV Adapter (China) (Unl).gba",
	//"rom_id": "AGBJ",
    //"internal_name": "AV 7111 ",

    //TODO:
    //"filename": "GBA Personal Organizer (USA) (Unl).gba",
    //"rom_id": "PDA1",
    //"internal_name": "PDA4AGB",

    //TODO:
    //"filename": "Gu Huo Lang 4 (Taiwan) (Unl).gba",
    //"internal_name": "Crash Bandic",
    //This might be Gu Huo Lang 3 not 4.
    //https://bootleggames.fandom.com/wiki/Sintax


	//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	// GBA cart size isn't in the header but the entrypoint word tells us
	// nothing useful about capacity, so we just report the file size.
	// Real cart sizes are always a power-of-2 (256 KiB, 512 KiB, 1/2/4/8/16/32 MB).
	// A file whose size isn't a power-of-2 is either trimmed or otherwise a bad dump.
	cartSize := formatSize(romSize)
	var sizeWarning string
	const minLicensedSize = 1024 * 1024 // 1 MB
	if romSize > 0 && romSize < minLicensedSize {
		// Legitimate licensed GBA games are normally 1 MB or larger.
		// Anything smaller is probably unlicensed, homebrew, or a bad dump.
		sizeWarning = fmt.Sprintf("ROM size %d bytes is unusually small, may be unlicensed, homebrew or a bad dump.", romSize)
	} else if romSize == 0 || (romSize&(romSize-1)) != 0 {
		// Round up to the next power-of-2 so the message is actionable.
		next := uint64(1)
		for next < romSize {
			next <<= 1
		}
		sizeWarning = fmt.Sprintf("ROM size %d bytes is not a valid cart size, expected %d bytes (%s). Run NitroValadator to check and or fix this.", romSize, next, formatSize(next))
	}



	// Read the 4-byte entrypoint for display (it's an ARM branch instruction)
	entryPoint := binary.LittleEndian.Uint32(hdr[agbEntryPoint:])

	_ = entryPoint // reserved for future JSON field if wanted

	info := &AGBROMInfo{
		Filename:      filepath.Base(romPath),
		RomDecoder:    "AGB",
		RomID:         gameCode,
		InternalName:  gameTitle,
		AltTitle:      altTitle,
		UnitCode:      int(hdr[agbUnitCode]),
		Prefix:        prefix,
		Region:        region,
		Serial:        fmt.Sprintf("%s-%s-%s", prefix, gameCode, region),
		RomVersion:    fmt.Sprintf("%02X", softwareVer),
		CRC32:         crc,
		PublisherCode: makerCode,
		Publisher:     makerName,
		DACSCode:      DACScode,
		DACSInfo:      DACSdebug,
		CartSizeBytes: romSize,
		CartSize:      cartSize,
	}
	//This is kinda dumb. it should just be if warning != ""
	//but we would have to like build and unpack and array or some shit and I dont wanna right now.
	//TODO: Regon code and pub code, so warning only formed after fixes not befor.
	if regionWarning != "" {
		FmtError(info, "warning", regionWarning)
	}
	if prefixWarning != "" {
		FmtError(info, "warning", prefixWarning)
	}
	if publisherWarning != "" {
		FmtError(info, "warning", publisherWarning)
	}
	if sizeWarning != "" {
		FmtError(info, "warning", sizeWarning)
	}

	return info, nil
}

// ============================================================
// Helpers
// ============================================================

// agbVerifyChecksum computes the complement checksum over bytes 0xA0-0xBC
// and compares it to the stored value at 0xBD.
// Formula from the GBA header spec:
//
//	checksum = (-( 0x19 + sum(0xA0..0xBC) )) & 0xFF
//
// Returns (computed, stored, ok). computed is what we calculated; stored is
// what the ROM header claims. ok is true when they match.
func agbVerifyChecksum(hdr []byte) (computed, stored uint8, ok bool) {
	var sum uint8
	for off := 0xA0; off <= 0xBC; off++ {
		sum += hdr[off]
	}
	computed = uint8((-int(0x19+sum)) & 0xFF)
	stored   = hdr[agbChecksum]
	return computed, stored, computed == stored
}

// agbComputeCRC32 streams the entire file through CRC32/Castagnoli.
// GBA has no "used ROM size" field so we always hash the full file.
func agbComputeCRC32(romPath string, size uint64) string {
	cf, err := os.Open(romPath)
	if err != nil {
		return "????????"
	}
	defer cf.Close()

	// reuse the same streaming approach as the NDS decoder
	// import hash/crc32 is handled in nds.go within the same package
	return computeCRC32(romPath, uint32(size))
}

//More comprehencive check cos GBA cant be fingerprinted easly in the first 16 bytes becxause of the ARM entry point code
func isAGB(fp string, hdr []byte) string {
		//TODO: move this code to agb.go module?
		// If we hit a match, exit early, otherwise fall down to the next tier.
    // Tier 1: standard ARM entry point for all legit games programed with Nintendos sdk?
    if bytes.Equal(hdr[0x00:0x04], []byte{0x2E, 0x00, 0x00, 0xEA}) {
        return "AGB"
    }

    // Tier 2: Legit Nintendo logo header (first 12 bytes of it)
    // If the rom contains theses bytes excaly then we assume they have the legit Nintendo logo in the rom and we can move on
    nintendoLogo := []byte{0x24, 0xFF, 0xAE, 0x51, 0x69, 0x9A, 0xA2, 0x21, 0x3D, 0x84, 0x82, 0x0A}
    if bytes.Equal(hdr[0x04:0x10], nintendoLogo) {
        return "AGB"
    }

    // Tier 3: Need to pass more data from the rom header
    // fingerprint only loads 16 bytes but f.Read(hdr) loads 512 bytes for this resion / advanced checks like this. 0x00 to 0x200
    //so we just have to manualy offset around in hdr for the bytes we want.
    if hdr[0x03] == 0xEA && hdr[0xB2] == 0x96 && hdr[0xB3] == 0x00 && hdr[0xB4] == 0x00 {
        // 0xB0:0xB2 should be printable ASCII publisher code. see 'var agbPublisher = map[string]string' for more info
        if hdr[0xB0] >= ' ' && hdr[0xB0] <= '`' && hdr[0xB1] >= ' ' && hdr[0xB1] <= '`' {
        	  //if we get here, we have the right rom data to load in a gba, and a publisher code was set, but other data may not be legit like logos.
            return "AGBL"
        } else {
        	//1. not a legit Nintendo ARM entry point
        	//2. not a legit Nintendo logo
        	//3. publsiher code has not been set corecly
        	//However, 0x03 == EA, 0xB2 == 0x96, 0xB3 == 00 and 0xB4 == 00
        	//So it will load? but will need even more checks with NitroValidater
        	return "AGBF"
        }
    }

    return "###"
}