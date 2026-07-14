package computer

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type MemDisk struct {
	buf []byte
}

func (m *MemDisk) ReadAt(p []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, errors.New("Negative offset")
	}
	if off >= int64(len(m.buf)) {
		return 0, io.EOF
	}
	n = copy(p, m.buf[off:])
	if n < len(p) {
		err = io.EOF
	}
	return n, err
}

func (m *MemDisk) WriteAt(b []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, errors.New("Negative offset")
	}
	if off >= int64(len(m.buf)) {
		if len(b) == 0 {
			return 0, nil
		}
		return 0, io.ErrShortWrite
	}

	n = copy(m.buf[off:], b)

	if n < len(b) {
		return n, io.ErrShortWrite
	}

	return len(b), nil
}

func (m *MemDisk) Truncate(size int64) error {
	if size < 0 {
		return errors.New("Negative size")
	}
	if size < int64(len(m.buf)) {
		m.buf = m.buf[:size]
	} else if size > int64(len(m.buf)) {
		extention := make([]byte, size-int64(len(m.buf)))
		m.buf = append(m.buf, extention...)
	}
	return nil
}

func (m *MemDisk) Close() error {
	return nil
}

type Disk interface {
	ReadAt(p []byte, off int64) (n int, err error)
	WriteAt(b []byte, off int64) (n int, err error)
	Truncate(size int64) error
	Close() error
}

// PLANNING
// if string too short ["p", "i", "c", "s", "\0","0","0","0"] // the extra 0's are padding

const LATEST_VERSION = 10

const (
	INODESIZE   = 0x80   // 128 in bytes
	BLOCKSIZE   = 0x1000 // 4096 in bytes
	DISKSIZE    = 67_108_864
	INODES      = 8064
	BLOCKS      = 8064 * 2
	MAGICLEN    = 8
	TOTALBLOCKS = 16384
)

type dataBlockType int

const (
	REG      = 0 // not a directory data block
	DIRBLOCK = 1
)

type dataBlock struct {
	blockType dataBlockType
	data      [BLOCKSIZE]byte // key-value pair type data block if it's a directory data block.
}

type FileSystem struct {
	disk     Disk
	superBlk SuperBlock // cached
	mu       sync.Mutex

	// maybe later cache the bitmaps for extra SPEED.
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

	if err := disk.Truncate(DISKSIZE); err != nil {
		panic(err)
	}

	// now check for the superblk header being correct and up to date.

	headerBuf := make([]byte, BLOCKSIZE)
	var headerSuperBlk SuperBlock
	_, err = io.ReadFull(disk, headerBuf)
	if err != nil {
		panic(err)
	}

	// Only get the header data to validate.

	copy(headerSuperBlk.magic[:], headerBuf[0:MAGICLEN])
	headerSuperBlk.version = binary.LittleEndian.Uint32(headerBuf[8:12])

	// check if the header is valid
	if string(headerSuperBlk.magic[:MAGICLEN]) != "BS-EXTFS" && isInitialized == true {
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

		superBuf := make([]byte, BLOCKSIZE)
		superBlk := SuperBlock{
			magic:   [MAGICLEN]byte{'B', 'S', '-', 'E', 'X', 'T', 'F', 'S'},
			version: LATEST_VERSION,

			blockSize: BLOCKSIZE,
			inodeSize: INODESIZE,

			inodeCount:     uint32(INODES),
			dataBlockCount: uint32(BLOCKS),

			inodeBitmapStartBlock: 1,
			inodeTableStartBlock:  2,

			dataBitmapStartBlock: 254,
			dataBlocksStartBlock: 255,

			totalBlocks: TOTALBLOCKS,
		}

		writeSuprBlktoSuprBuf(superBuf, superBlk)

		_, _ = disk.WriteAt(superBuf, 0)

		cachedSuperBlock = superBlk

		// next we need to format the inode bitmap and maybe cache it.

		inodeBitmapBuf := make([]byte, BLOCKSIZE)

		inodeBitmapBuf[0] = 0b00000111 // the 0 1 2 inodes are taken, 2 is root.

		// so the bytes are left to right, but in a byte its right to left. odd ik but yea.

		// write to disk

		_, _ = disk.WriteAt(inodeBitmapBuf, BLOCKSIZE*int64(superBlk.inodeBitmapStartBlock))

		// inodes into disk

		inodeTableBuf := make([]byte, BLOCKSIZE*252)
		// inodeTableBuf[0] = 0b11111111
		// inodeTableBuf[252*DATABLOCKSIZE-1] = 0b11111111

		_, _ = disk.WriteAt(inodeTableBuf, BLOCKSIZE*int64(superBlk.inodeTableStartBlock))

		// data bitmap

		dataBitmapBuf := make([]byte, BLOCKSIZE)
		// dataBitmapBuf[0] = 0b10101010
		// dataBitmapBuf[BLOCKSIZE-1] = 0b10101010
		disk.WriteAt(dataBitmapBuf, int64(superBlk.dataBitmapStartBlock)*BLOCKSIZE)

		// data blocks
		dataBlocksBuf := make([]byte, BLOCKSIZE*superBlk.dataBlockCount)
		// dataBlocksBuf[0] = 0b10101010
		// dataBlocksBuf[(DATABLOCKSIZE*superBlk.dataBlockCount)-1] = 0b10101010
		disk.WriteAt(dataBlocksBuf, int64(superBlk.dataBlocksStartBlock)*BLOCKSIZE)

		// padding

		paddingBuf := make([]byte, BLOCKSIZE)
		for idx := range paddingBuf {
			paddingBuf[idx] = 103 // 0x67
		}
		disk.WriteAt(paddingBuf, int64(superBlk.dataBlocksStartBlock+BLOCKS)*BLOCKSIZE)

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

	fs := &FileSystem{
		superBlk: cachedSuperBlock,
		disk:     disk,
	}

	if !isInitialized {
		// create the root inode

		// passing a pointer cuz less memory, ik it doesnt need to mutate.
		fs.writeInode(&inode{
			size:  0,
			fType: S_IFDIR,
			refs:  0,
			owner: [14]byte{'r', 'o', 'o', 't'},

			setuid:     false,
			ownerMode:  0b111, // 7
			otherMode:  0b101, // 5
			direct:     [12]uint32{},
			find:       0,
			sind:       0,
			tind:       0,
			createdAt:  uint64(time.Now().Unix()),
			modifiedAt: uint64(time.Now().Unix()),
		}, 2)
	}

	return fs

	// falloc and write to inode
}

func (fs *FileSystem) Shutdown() {
	// close the file
	// this function is called on engine shutdown

	fs.disk.Close()
	return
}
