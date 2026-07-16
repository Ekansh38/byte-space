package computer

import (
	"encoding/binary"
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


//type DirectoryOps struct {
//}
//
//func (d *DirectoryOps) CreateFD(kernel *Kernel, inodeNum uint32, path string, flags int) FD {
//	// TODO
//}
//

//func (d *DirectoryOps) ReadEntries(kernel *Kernel) []DirEntry {
//	// TODO
//}
//

type InodeOperations interface {
    CreateFD(kernel *Kernel, inodeNum uint32, path string, flags int) FD
    ReadEntries(kernel *Kernel) []DirEntry
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
