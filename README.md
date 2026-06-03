# NITRODecoder
Metadata extractor for ROMs from nintendio handhelds
(Yes, I spelt that wrong on perpuse)

# Info
This tool can be used to extract metadata like rom ids and publisher info from nintendio game roms.<br>
It can also be used to extract rom icons for the roms that support this.<br>
Extracted metadata is in json format for easy decoding with or piping to other tools and apps.

# Sample
### GameBoy Advanced
```json
{
  "filename": "POKEMON FIRE.gba",
  "decoder": "AGB",
  "rom_id": "BPRE",
  "internal_name": "POKEMON FIRE",
  "alt_title": "pokemon red version",
  "unit_code": 0,
  "prefix": "AGB",
  "region": "USA",
  "serial": "AGB-BPRE-USA",
  "rom_version": "00",
  "crc32": "EB0E24DA",
  "publisher_code": "01",
  "publisher": "Nintendo",
  "dacs_code": "00",
  "dacs_info": "Retail",
  "rom_size_bytes": 16777216,
  "rom_size": "16 MB"
}
```
### Nintendo DS
```json
{
  "filename": "Pokemon - Diamond Version (USA) (Rev 5).nds",
  "decoder": "NDS",
  "rom_id": "ADAE",
  "internal_name": "POKEMON D",
  "unit_code": 0,
  "prefix": "NTR",
  "region": "USA",
  "serial": "NTR-ADAE-USA",
  "rom_version": "05",
  "crc32": "A2A36934",
  "publisher_code": "01",
  "publisher": "Nintendo",
  "lock_code": "00",
  "region_lock": "None",
  "cart_code": "09",
  "cart_size_bytes": 67108864,
  "cart_size": "64 MB",
  "cart_info": "Common NDS cart",
  "titles": {
    "ENG": "Pokémon Diamond\nNintendo"
  },
  "exported": {
    "PNG": "ADAE.PNG"
  }
}
```
Exported icon<br>
ADAE.PNG
<img width="32" height="32" alt="ADAE" src="https://github.com/user-attachments/assets/2cbc881c-d3a6-4300-b80e-821548b596fd" /><br><br>
### Nintendo DSi
```json
{
  "filename": "tests\\NDSi\\Pokemon - Black Version 2 (USA, Europe) (NDSi Enhanced).nds",
  "decoder": "NDS",
  "rom_id": "IREO",
  "internal_name": "POKEMON B2",
  "unit_code": 2,
  "prefix": "TWL",
  "region": "USA",
  "serial": "TWL-IREO-USA",
  "rom_version": "00",
  "crc32": "149E2561",
  "publisher_code": "01",
  "publisher": "Nintendo",
  "lock_code": "40",
  "region_lock": "NTR Korea",
  "cart_code": "0C",
  "cart_size_bytes": 536870912,
  "cart_size": "512 MB",
  "cart_info": "NDS or DSi enhanced cart with a built-in Infrared port",
  "titles": {
    "ENG": "Pokémon\nBlack Version 2\nNintendo"
  },
  "exported": {
    "GIF": "tests\\NDSi\\IREO.GIF",
    "PNG": "tests\\NDSi\\IREO.PNG"
  }
}
```
Exported icons<br>
IREO.GIF
<img width="32" height="32" alt="IREO" src="https://github.com/user-attachments/assets/2cf6d43a-de0a-4c2c-98a0-84f6ab073f84" /><br>
IREO.PNG
<img width="32" height="32" alt="IREO" src="https://github.com/user-attachments/assets/d116d226-335a-49c7-8df7-9226c74d0a4c" /><br>

# Usage
You can either drag'n'drop rom files to the exe direcly to get info on them<br>
Or you can run NitroDecoder in a termail with the `-cli` flag to suppress anything that cant be encoded to json. This is also how you run the `-export` flag as this is not ran by default.

# Support
| Console          | Extension    | Unit/Prefix Code | Note |
| --- | --- | --- | --- |
| Gameboy Advanced | .gba         | AGB              | No save type decode yet |
| DS               | .nds or .ids | NTR              | Decrypted and Encrypted |
| DSi              | .nds         | TWI              | Decrypted and Encrypted |

//TODO: gbc and gb roms.
## Unsported
Play-Yan: AGS-006. The common available romsets only have the mpeg files for the sd card, not a dump of the main gba player cart. <br>
3ds: Don't have any plans for this because idk what to do with the 3d banner files.<br>
nsp: Lol no. Outside of the encryption, i'm not gonna go there.<br>
xci: Double nope. That's a mess for a different reason I don't wanna poke at.<br>
(TL;DR all your encryption is missing, makes it easy to read, not easy to archive and process as un-tampered with.)<br>
zip: Dude, extract your rom files. yes really.<br>
## Limitations
Decrypted VS Encrypted DS roms: The game contents is encrypted, not the header we are reading.<br>
and only 0x4000 to 0x47FF (only 0x800 or 2 kiB) is encrypted? needs more testing and data points.<br>
Gameboy Advanced video: While NitroDecoder does support these large rom files,<br>
as we are only reading the header we cant tell outside of the large rom size if its a gba video or a game.<br>
