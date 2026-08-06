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

func (d *DirectoryOps) addDentry(kernel *Kernel, dentry DirEntry) error {
	// this is when I kinda discovered Linux calls the dentries and I found it really nice so my code
	// kind of has a split between DirEntries and Dentries but i vibe with it

	// stuff I got to do here
	// look at all of the data blocks in the inode for space, for a 64 chunk of space
	// if i find that space
	// encode that dentry there, and write to disk.
	// else
	// falloc inode.size + 64 and then encode it into there and write to disk.

	dirInode := inode{}
	err := kernel.computer.fs.readInode(&dirInode, d.inodeNum) // TODO migrate this to kernel.getInode later
	if err != nil {
		return err
	}

	return nil
}

// func (d *DirectoryOps) removeDentry(kernel *Kernel, dentry DirEntry) error {

//	dirInode := kernel.getInode(d.inodeNum)

//}

func (d *DirectoryOps) ReadEntries(kernel *Kernel) ([]DirEntry, error) {
	dirEntries := make([]DirEntry, 0, 8) // my guess is like max 8 entries per folder on the average 
	                                     // case, i just don't want to have to reallocate
	
	// get the inode, later we migrate to kernel.getInode 
	dirInode := &inode{}
	err := kernel.computer.fs.readInode(dirInode, d.inodeNum)
	if err != nil {
		return nil, err
	}

	err = kernel.computer.fs.iterBlocks(dirInode, func(_ uint32, data [BLOCKSIZE]byte) (stop bool, err error) {
		newDirEntries := decodeDirEntries(data)
		dirEntries = append(dirEntries, newDirEntries...)
		return false, nil
	})

	if err != nil {
		return nil, err
	}
	return dirEntries, nil
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
	// setup the stuff needed for the fd to be good right?

	d.offset = 0

	return nil
}

func (d *DirectoryFD) Read(buf []byte) (int, error) {
	return 0, nil
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
