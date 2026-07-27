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
		entry := block[i : i+64] // inclusive:exclusive

		// check if the inum is 0

		inum := binary.LittleEndian.Uint32(entry[0:4])

		if inum == 0 {
			continue // skip that entry, its blank!
		}

		nameLen := entry[4]
		name := string(entry[8 : nameLen+8]) // go strings dont need no null terminator, they store
		// the length unlike C, this took a while for me to wrap
		// my head around! I just realized!!!!!! BRR

		dirEntry := DirEntry{Inum: inum, Name: name}

		dirEntries = append(dirEntries, dirEntry)
	}

	return dirEntries
}

type DirectoryOps struct {
	inodeNum uint32
}

func (d *DirectoryOps) CreateFD(kernel *Kernel, inodeNum uint32, path string, flags int) FD {
	return &DirectoryFD{
		kernel:   kernel,
		offset:   0,
		ops:      d,
		inodeNum: inodeNum,
		flags:    flags,
	}
}

func (d *DirectoryOps) ReadEntries(kernel *Kernel) ([]DirEntry, error) {
	dirInode := &inode{}
	err := kernel.computer.fs.readInode(dirInode, d.inodeNum)
	if err != nil {
		return nil, err
	}

	// based on .size lets figure out how many blocks we need to read.

	blocksToRead := (dirInode.size + BLOCKSIZE - 1) / BLOCKSIZE
	// same: ceil(float(dirInode.size) / float(BLOCKSIZE))

	var findBuf []byte = nil
	var sindBuf []byte = nil
	var tindBuf []byte = nil

	var placeRelativeAddress uint32
	var place int

	resolve := func(indirectPtr uint32, dindBuf *[]byte) uint32 {
		if *dindBuf == nil {
			*dindBuf, _ = kernel.computer.fs.readDataBlock(indirectPtr)
		}

		return binary.LittleEndian.Uint32((*dindBuf)[placeRelativeAddress*4 : placeRelativeAddress*4+4])
	}

	var i uint32
	for i = 0; i < blocksToRead; i++ {
		var blockNumber uint32
		placeRelativeAddress, place = virtualToPlaceRelative(i)

		if place == 0 {
			blockNumber = dirInode.direct[placeRelativeAddress] // the actual datablock number
		} else if place == 1 {
			blockNumber = resolve(dirInode.find, &findBuf)
		} else if place == 2 {
			blockNumber = resolve(dirInode.sind, &sindBuf)
		} else if place == 3 {
			blockNumber = resolve(dirInode.tind, &tindBuf)
		}

		// get the data block content
		data, err := kernel.computer.fs.readDataBlock(blockNumber)
		if err != nil {
			return nil, err
		}




	}
}

type DirectoryFD struct {
	kernel   *Kernel
	inodeNum uint32
	offset   uint64
	flags    int // flags are like READONLY_O and stuff
	ops      InodeOperations
	cached   []byte
}

func (d *DirectoryFD) Open() error {
	return nil
}

func (d *DirectoryFD) Read(buf []byte) (int, error) {
	return 0, nil
	// lowkey just copy from the plan and maybe write in a more verbose manner
}

func (d *DirectoryFD) Write(data []byte) (int, error) {
	return 0, nil
}

func (d *DirectoryFD) Close() error {
	return nil
}

func (d *DirectoryFD) InodeNum() uint32 {
	return d.inodeNum
}

func (d *DirectoryFD) Offset() uint64 {
	return d.offset
}

func (d *DirectoryFD) SetOffset(offset uint64) {
	d.offset = offset
}

func (d *DirectoryFD) Flags() int {
	return d.flags
}
