package computer

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type InodeType uint8

const (
	S_IFREG = 0
	S_IFDIR = 1
)

type FD interface {
	Open() error
	Read(buf []byte) (int, error)
	Write(data []byte) (int, error)
	Close() error

	InodeNum() uint32
	Offset() uint64
	SetOffset(uint64)
	Flags() int
}


type InodeOperations interface {
	CreateFD(kernel *Kernel, inodeNum uint32, path string, flags int) FD
	ReadEntries(kernel *Kernel) ([]DirEntry, error)
}

type DirEntry struct {
	Inum uint32
	Name string
}

type inode struct {
	dirty bool
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
	sind   uint32     // second first-indirect
	tind   uint32     // third first-indirect
	// since its a small fs we dont need second and third indirect

	// sind and tind are not unix style recurisve indirection just 1024 + 1024 + 1024. simple

	createdAt  uint64
	modifiedAt uint64

	// runtime only
	num uint32
	ops InodeOperations
}

func (fs *FileSystem) readInode(inode *inode, idx uint32) error {
	// offset
	inode.num = idx
	inodeTableOffset := int(fs.superBlk.inodeTableStartBlock) * BLOCKSIZE
	startIdx := inodeTableOffset + int(idx)*INODESIZE
	inodeBuf := make([]byte, INODESIZE) // 128

	// Read from disk
	n, err := fs.disk.ReadAt(inodeBuf, int64(startIdx))
	if err != nil {
		return err
	}
	if n != INODESIZE {
		return io.ErrUnexpectedEOF
	}

	// Deserialize from buffer into inode struct
	inode.size = binary.LittleEndian.Uint32(inodeBuf[0:4])
	inode.fType = InodeType(inodeBuf[4])
	inode.refs = binary.LittleEndian.Uint16(inodeBuf[5:7])

	for idx, val := range inodeBuf[7:21] {
		inode.owner[idx] = val
	}

	if inodeBuf[21] == 1 {
		inode.setuid = true
	} else {
		inode.setuid = false
	}

	inode.ownerMode = inodeBuf[22]
	inode.otherMode = inodeBuf[23]

	for idx := range inode.direct {
		off := idx * 4
		inode.direct[idx] = binary.LittleEndian.Uint32(inodeBuf[24+off : 28+off])
	}

	inode.find = binary.LittleEndian.Uint32(inodeBuf[72:76])
	inode.sind = binary.LittleEndian.Uint32(inodeBuf[76:80])
	inode.tind = binary.LittleEndian.Uint32(inodeBuf[80:84])

	inode.createdAt = binary.LittleEndian.Uint64(inodeBuf[84:92])
	inode.modifiedAt = binary.LittleEndian.Uint64(inodeBuf[92:100])

	return nil
}

func (fs *FileSystem) writeInode(inode *inode, idx uint32) error {
	// doesnt care about the free bitmap shit.
	// is a pure savage peak at binary serlaiztion

	inodeTableOffset := int(fs.superBlk.inodeTableStartBlock) * BLOCKSIZE
	startIdx := inodeTableOffset + int(idx)*INODESIZE
	inodeBuf := make([]byte, INODESIZE) // 128

	binary.LittleEndian.PutUint32(inodeBuf[0:4], inode.size)
	inodeBuf[4] = byte(inode.fType)
	binary.LittleEndian.PutUint16(inodeBuf[5:7], inode.refs)

	for idx, val := range inode.owner {
		inodeBuf[7+idx] = val
	}

	if inode.setuid {
		inodeBuf[21] = 1
	} else {
		inodeBuf[21] = 0
	}

	inodeBuf[22] = byte(inode.ownerMode)
	inodeBuf[23] = byte(inode.otherMode)

	for idx, val := range inode.direct {
		off := idx * 4
		binary.LittleEndian.PutUint32(inodeBuf[24+off:28+off], val)
	}

	binary.LittleEndian.PutUint32(inodeBuf[72:76], inode.find)
	binary.LittleEndian.PutUint32(inodeBuf[76:80], inode.sind)
	binary.LittleEndian.PutUint32(inodeBuf[80:84], inode.tind)

	binary.LittleEndian.PutUint64(inodeBuf[84:92], inode.createdAt)
	binary.LittleEndian.PutUint64(inodeBuf[92:100], inode.modifiedAt)

	// ik the buffer is already zero'd but im simulating C kinda. Just keeping tidy and verbose.
	for i := 0; i < 28; i++ {
		inodeBuf[100+i] = 0
	}

	n, err := fs.disk.WriteAt(inodeBuf, int64(startIdx))
	if err != nil {
		return err
	}
	if n != INODESIZE {
		fmt.Println("WTF HAPPENED!!!! GRR")
		return io.ErrShortWrite
	}

	return nil
}

// iterBlocks calls fn for each allocated data block in the inode, in order.
// fn receives the physical block number and a copy of its contents.
// Return stop=true from fn to break early.
func (fs *FileSystem) iterBlocks(in *inode, fn func(blockNum uint32, data [BLOCKSIZE]byte) (stop bool, err error)) error {
	blocksToRead := (in.size + BLOCKSIZE - 1) / BLOCKSIZE

	var findBuf, sindBuf, tindBuf []byte

	resolve := func(ptr uint32, buf *[]byte, relAddr uint32) (uint32, error) {
		if *buf == nil {
			blk, err := fs.readDataBlock(ptr)
			if err != nil {
				return 0, err
			}
			*buf = blk[:]
		}
		return binary.LittleEndian.Uint32((*buf)[relAddr*4 : relAddr*4+4]), nil
	}

	for i := uint32(0); i < blocksToRead; i++ {
		relAddr, place := virtualToPlaceRelative(i)

		var blockNum uint32
		var err error
		switch place {
		case 0:
			blockNum = in.direct[relAddr]
		case 1:
			blockNum, err = resolve(in.find, &findBuf, relAddr)
		case 2:
			blockNum, err = resolve(in.sind, &sindBuf, relAddr)
		case 3:
			blockNum, err = resolve(in.tind, &tindBuf, relAddr)
		}
		if err != nil {
			return err
		}

		data, err := fs.readDataBlock(blockNum)
		if err != nil {
			return err
		}

		stop, err := fn(blockNum, data)
		if err != nil || stop {
			return err
		}
	}
	return nil
}

// think about concurreny later TODO
// maybe we aquire locks at a higher kernel level rather than these low level function calls

func (fs *FileSystem) AllocInode() (uint32, error) {
	inodeBitmapBuf := make([]byte, INODES/8) // number of inodes in bytes
	inodeBitmapByteOffset := fs.superBlk.inodeBitmapStartBlock * BLOCKSIZE

	fs.inodeBitmapMu.Lock()
	defer fs.inodeBitmapMu.Unlock()

	_, err := fs.disk.ReadAt(inodeBitmapBuf, int64(inodeBitmapByteOffset))
	if err != nil {
		return 0, err
	}

	idx, status := findFreeBit(inodeBitmapBuf)

	if !status {
		return 0, errors.New("no space left bro")
	}

	setBit(inodeBitmapBuf, idx)
	_, err = fs.disk.WriteAt(inodeBitmapBuf, int64(inodeBitmapByteOffset))
	if err != nil { // verbose but idgaf
		return 0, err
	}

	return idx, nil
}

func (fs *FileSystem) FreeInodeFromBitmap(inum uint32) error {
	if inum >= INODES {
		return fmt.Errorf("FreeInodeFromBitmap: inum %d out of range", inum)
	}


	inodeBitmapBuf := make([]byte, INODES/8) // number of inodes in bytes
	inodeBitmapByteOffset := fs.superBlk.inodeBitmapStartBlock * BLOCKSIZE

	fs.inodeBitmapMu.Lock()
	defer fs.inodeBitmapMu.Unlock()

	_, err := fs.disk.ReadAt(inodeBitmapBuf, int64(inodeBitmapByteOffset))
	if err != nil {
		return err
	}

	if inodeBitmapBuf[inum/8]&(1<<(inum%8)) == 0 {
		return fmt.Errorf("FreeInodeFromBitmap: inum %d already free", inum)
	}

	clearBit(inodeBitmapBuf, inum)

	_, err = fs.disk.WriteAt(inodeBitmapBuf, int64(inodeBitmapByteOffset))

	return err
}
