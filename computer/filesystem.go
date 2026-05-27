package computer

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
)

// PLANNING
// inode file: meta data (file-size, type) + block list + FIND, SIND, TIND
// data file: raw bytes (fixed size)
// directory data block entry (fixed size)
// if string too short ["p", "i", "c", "s", "\0","0","0","0"] // the extra 0's are padding
// metadata: freeBlocks + root
// write: take free block -> store data -> update inode
// read: inode -> blocks -> combine
// delete: free blocks -> remove inode
// freeBlocks empty = disk full (FAHH)

type InodeType uint8

const (
	S_IFREG = 0
	S_IFDIR = 1
)

const LATEST_VERSION = 4

const (
	INODESIZE     = 0x80   // 128 in bytes
	DATABLOCKSIZE = 0x1000 // 4096 in bytes
	DISKSIZE      = 67_108_864
	INODES        = 8064
	BLOCKS        = 8064*2
	MAGICLEN      = 8
	TOTALBLOCKS = 16384
)

type inode struct {
	size  uint32 // file-size in bytes
	fType InodeType
	refs  uint16

	owner [14]byte // the owner of the file/folder, // TODO cap the username len to 13 in the kernel
	// string of the username of the creator,
	// for system files it is root, for other stuff /home/user it is that user.

	setuid bool // true means the user who runs that program can run it in the permissions of the owner

	ownerMode uint8
	otherMode uint8

	//    rwx     // read  write  execute permissions
	// 0: 000
	// 1: 001
	// 2: 010
	// 3: 011
	// 4: 100
	// 5: 101
	// 6: 110
	// 7: 111

	// BLOCK LISTS

	direct [12]uint32 // 12 x4 = 48
	find   uint32     // first-indirect
	sind   uint32     // second-indirect
	tind   uint32     // third-indirect

	createdAt  uint64
	modifiedAt uint64
}

type dataBlockType int

const (
	REG      = 0 // not a directory data block
	DIRBLOCK = 1
)

type dataBlock struct {
	blockType dataBlockType
	data      [DATABLOCKSIZE]byte // key-value pair type data block if it's a directory data block.
}

type FileSystem struct {
	disk     *os.File
	superBlk SuperBlock // cached

	// maybe later cache the bitmaps for extra SPEED.
}

type SuperBlock struct {
	magic   [8]byte // BS-EXTFS
	version uint32  // 1

	blockSize uint32 // 4096

	inodeCount uint32 // ~8192
	inodeSize  uint32 // 128

	inodeBitmapStartBlock uint32 // 1
	inodeTableStartBlock  uint32 // 2

	dataBlockCount uint32 // 16384

	dataBlocksStartBlock uint32 // 255
	dataBitmapStartBlock uint32 // 254

	totalBlocks uint32 // 16384

	// later maybe a dirty bit

	// total: 48
}

// bitmap structure: [inodeCount]uint64

// disk structure

// SUPER BLOCK
// INODE BITMAP
// INODE TABLE
// DATA BITMAP
// DATA BLOCKS

func NewFileSystem(basePath string) *FileSystem {
	var cachedSuperBlock SuperBlock

	os.MkdirAll(basePath, 0o755)
	diskPath := filepath.Join(basePath, "disk.img")

	// Check formatting
	isInitialized := true
	if _, err := os.Stat(diskPath); errors.Is(err, os.ErrNotExist) {
		log.Println("Disk not created yet.")
		isInitialized = false
	}
	disk, err := os.OpenFile(
		diskPath,
		os.O_CREATE|os.O_RDWR,
		0o644,
	)
	if err != nil {
		panic(err)
	}

	disk.Truncate(DISKSIZE)

	// now check for the superblk header being correct and up to date.

	disk.Seek(0, io.SeekStart)

	headerBuf := make([]byte, DATABLOCKSIZE)
	var headerSuperBlk SuperBlock
	_, err = io.ReadFull(disk, headerBuf)
	if err != nil {
		panic(err)
	}

	// Only get the header data to validate.

	copy(headerSuperBlk.magic[:], headerBuf[0:MAGICLEN])
	headerSuperBlk.version = binary.LittleEndian.Uint32(headerBuf[8:12])

	// check if the header is valid
	if string(headerSuperBlk.magic[:MAGICLEN]) != "BS-EXTFS" && isInitialized == true{
		log.Println("Invalid magic: expected BS-EXTFS, got %s", headerSuperBlk.magic)
		isInitialized = false
	}
	if headerSuperBlk.version != LATEST_VERSION && isInitialized == true {
		log.Println("Invalid version: expected %d, got %d", LATEST_VERSION, headerSuperBlk.version)
		isInitialized = false
	}

	if !isInitialized {
		// format the fs

		// If its not valid we need to create and initialize a new disk.img

		superBuf := make([]byte, DATABLOCKSIZE)
		superBlk := SuperBlock{
			magic:                 [MAGICLEN]byte{'B', 'S', '-', 'E', 'X', 'T', 'F', 'S'},
			version:               LATEST_VERSION,

			blockSize:             DATABLOCKSIZE,
			inodeSize:             INODESIZE,

			inodeCount:            uint32(INODES),
			dataBlockCount:        uint32(BLOCKS),

			inodeBitmapStartBlock: 1,
			inodeTableStartBlock:  2,

			dataBitmapStartBlock:  254,
			dataBlocksStartBlock:  255,

			totalBlocks:           TOTALBLOCKS,
		}


		writeSuprBlktoSuprBuf(superBuf, superBlk)

		_, _ = disk.WriteAt(superBuf, 0)

		cachedSuperBlock = superBlk

		// next we need to format the inode bitmap and maybe cache it.

		inodeBitmapBuf := make([]byte, DATABLOCKSIZE)

		inodeBitmapBuf[0] = 0b00000111 // the 0 1 2 inodes are taken, 2 is root.

		// so the bytes are left to right, but in a byte its right to left. odd ik but yea.

		// write to disk

		_, _ = disk.WriteAt(inodeBitmapBuf, DATABLOCKSIZE * int64(superBlk.inodeBitmapStartBlock))

		// inodes into disk

		inodeTableBuf := make([]byte, DATABLOCKSIZE * 252)
		inodeTableBuf[0] = 0b11111111
		inodeTableBuf[252*DATABLOCKSIZE-1] = 0b11111111

		_, _ = disk.WriteAt(inodeTableBuf, DATABLOCKSIZE * int64(superBlk.inodeTableStartBlock))




	} else {
		// Copy from buffer into go struct data structure

		headerSuperBlk.blockSize = binary.LittleEndian.Uint32(headerBuf[12:16])
		headerSuperBlk.inodeCount = binary.LittleEndian.Uint32(headerBuf[16:20])
		headerSuperBlk.inodeSize = binary.LittleEndian.Uint32(headerBuf[20:24])
		headerSuperBlk.inodeTableStartBlock = binary.LittleEndian.Uint32(headerBuf[24:28])
		headerSuperBlk.inodeBitmapStartBlock = binary.LittleEndian.Uint32(headerBuf[28:32])
		headerSuperBlk.dataBlockCount = binary.LittleEndian.Uint32(headerBuf[32:36])
		headerSuperBlk.dataBlocksStartBlock = binary.LittleEndian.Uint32(headerBuf[36:40])
		headerSuperBlk.dataBitmapStartBlock = binary.LittleEndian.Uint32(headerBuf[40:44])
		headerSuperBlk.totalBlocks = binary.LittleEndian.Uint32(headerBuf[44:48])

		cachedSuperBlock = headerSuperBlk
	}

	return &FileSystem{
		superBlk: cachedSuperBlock,
		disk:     disk,
	}
}

func (fs *FileSystem) Shutdown() {
	// close the file
	// this function is called on engine shutdown

	fs.disk.Close()
	return
}

func writeSuprBlktoSuprBuf(suprBuf []byte, suprBlk SuperBlock) {
	// Indexing is [inclusive]:[exclusive]
	copy(suprBuf[0:8], suprBlk.magic[:])
	binary.LittleEndian.PutUint32(suprBuf[8:12], suprBlk.version)
	binary.LittleEndian.PutUint32(suprBuf[12:16], suprBlk.blockSize)
	binary.LittleEndian.PutUint32(suprBuf[16:20], suprBlk.inodeCount)
	binary.LittleEndian.PutUint32(suprBuf[20:24], suprBlk.inodeSize)
	binary.LittleEndian.PutUint32(suprBuf[24:28], suprBlk.inodeTableStartBlock)
	binary.LittleEndian.PutUint32(suprBuf[28:32], suprBlk.inodeBitmapStartBlock)
	binary.LittleEndian.PutUint32(suprBuf[32:36], suprBlk.dataBlockCount)
	binary.LittleEndian.PutUint32(suprBuf[36:40], suprBlk.dataBlocksStartBlock)
	binary.LittleEndian.PutUint32(suprBuf[40:44], suprBlk.dataBitmapStartBlock)
	binary.LittleEndian.PutUint32(suprBuf[44:48], suprBlk.totalBlocks)
}
