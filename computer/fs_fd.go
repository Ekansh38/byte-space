package computer

import (
	"encoding/binary"
	"io"
)

// RegularFileOps is stateless — same for every regular file. the per-open
// state all lives in the FileFD it hands out.
type RegularFileOps struct{}

func (r *RegularFileOps) CreateFD(kernel *Kernel, inodeNum uint32, path string, flags int) FD {
	return &FileFD{
		kernel:   kernel,
		inodeNum: inodeNum,
		offset:   0,
		flags:    flags,
	}
}

func (r *RegularFileOps) ReadEntries(kernel *Kernel) ([]DirEntry, error) {
	return nil, nil // not a directory
}

type FileFD struct {
	kernel   *Kernel
	inodeNum uint32
	offset   uint64
	flags    int
}

// blockNumForVirtual resolves a virtual block index (0 = first block of the
// file) to a physical data block number by jumping into direct/find/sind/tind.
func (fs *FileSystem) blockNumForVirtual(in *inode, v uint32) (uint32, error) {
	relAddr, place := virtualToPlaceRelative(v)

	var indirectPtr uint32
	switch place {
	case 0:
		return in.direct[relAddr], nil
	case 1:
		indirectPtr = in.find
	case 2:
		indirectPtr = in.sind
	case 3:
		indirectPtr = in.tind
	}

	blk, err := fs.readDataBlock(indirectPtr)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(blk[relAddr*4 : relAddr*4+4]), nil
}

func (f *FileFD) Open() error {
	// nothing to validate for regular files
	return nil
}

func (f *FileFD) Read(buf []byte) (int, error) {
	in, err := f.kernel.getInode(f.inodeNum)
	if err != nil {
		return 0, err
	}

	if f.offset >= uint64(in.size) {
		return 0, io.EOF
	}

	toRead := uint64(len(buf))
	if remaining := uint64(in.size) - f.offset; toRead > remaining {
		toRead = remaining
	}

	var n uint64
	for n < toRead {
		v := uint32(f.offset / BLOCKSIZE)
		blockOff := f.offset % BLOCKSIZE

		blockNum, err := f.kernel.computer.fs.blockNumForVirtual(in, v)
		if err != nil {
			return int(n), err
		}
		data, err := f.kernel.computer.fs.readDataBlock(blockNum)
		if err != nil {
			return int(n), err
		}

		chunk := toRead - n
		if maxInBlock := BLOCKSIZE - blockOff; chunk > maxInBlock {
			chunk = maxInBlock
		}

		copy(buf[n:n+chunk], data[blockOff:blockOff+chunk])
		n += chunk
		f.offset += chunk
	}

	return int(n), nil
}

func (f *FileFD) Write(data []byte) (int, error) {
	in, err := f.kernel.getInode(f.inodeNum)
	if err != nil {
		return 0, err
	}

	// writing past the end? falloc first, then the blocks exist.
	newEnd := f.offset + uint64(len(data))
	if newEnd > uint64(in.size) {
		if err := f.kernel.computer.fs.Falloc(in, uint32(newEnd)); err != nil {
			return 0, err
		}
	}

	n := 0
	for n < len(data) {
		v := uint32(f.offset / BLOCKSIZE)
		blockOff := f.offset % BLOCKSIZE

		blockNum, err := f.kernel.computer.fs.blockNumForVirtual(in, v)
		if err != nil {
			return n, err
		}

		// read-modify-write so partial block writes dont clobber neighbours
		blk, err := f.kernel.computer.fs.readDataBlock(blockNum)
		if err != nil {
			return n, err
		}

		chunk := copy(blk[blockOff:], data[n:])
		if err := f.kernel.computer.fs.writeDataBlock(blockNum, blk[:]); err != nil {
			return n, err
		}

		n += chunk
		f.offset += uint64(chunk)
	}

	// persist the new size + any block pointers falloc handed out
	return n, f.kernel.computer.fs.writeInode(in, f.inodeNum)
}

func (f *FileFD) Close() error {
	return nil
}

func (f *FileFD) InodeNum() uint32 {
	return f.inodeNum
}

func (f *FileFD) Offset() uint64 {
	return f.offset
}

func (f *FileFD) SetOffset(offset uint64) {
	f.offset = offset
}

func (f *FileFD) Flags() int {
	return f.flags
}
