package computer

import (
	"encoding/binary"
	"errors"
)

func encodeDirEntry(name string, inum uint32) ([64]byte, error) {
	var buffer [64]byte

	if len(name) > 55 {
		return buffer, errors.New("name too long")
	}

	if inum == 0 {
		return buffer, errors.New("invalid inum")
	}

	nameLen := uint8(len(name)) // in bytes

	// Indexing is [inclusive]:[exclusive]
	binary.LittleEndian.PutUint32(buffer[0:4], inum)
	buffer[4] = nameLen

	// bytes 5, 6 and 7 are padding

	copy(buffer[8:], name)

	// the array is zeroed to automatically pads with null-terminators

	return buffer, nil
}

func decodeDirEntries(block [BLOCKSIZE]byte) []DirEntry {
	var dirEntries []DirEntry

	for i := 0; i < BLOCKSIZE; i += 64 {
		entry := block[i:i+64] // inclusive:exclusive

		// check if the inum is 0 

		inum := binary.LittleEndian.Uint32(entry[0:4])

		if inum == 0 {
			continue // skit that entry, its blank!
		}

		nameLen := entry[4]
		name := string(entry[8:nameLen+8]) // go strings dont need no null terminator, they store 
										   // the length unlike C, this took a while for me to wrap
										   // my head around! I just realized!!!!!! BRR

		dirEntry := DirEntry{Inum: inum, Name: name}

		dirEntries = append(dirEntries, dirEntry)
	}

	return dirEntries
}
