package computer

import (
	"encoding/binary"
	"errors"
	"math/bits"
)

func clearBit(bitmap []byte, idx uint32) {
	bitmap[idx/8] &= ^(1 << (idx % 8))
}

func setBit(bitmap []byte, idx uint32) {
	bitmap[idx/8] |= 1 << (idx % 8)
}

// learn about how this works one day since fable wrote it.
func findFreeBit(bitmap []byte) (uint32, bool) {
	i := 0
	n := len(bitmap)

	// skip fully-occupied 64-bit words in big chunks
	for ; i+8 <= n; i += 8 {
		word := binary.LittleEndian.Uint64(bitmap[i : i+8])
		if word == ^uint64(0) {
			continue
		}
		free := bits.TrailingZeros64(^word)
		bitIdx := uint32(i)*8 + uint32(free)
		return bitIdx, true
	}

	// tail bytes
	for ; i < n; i++ {
		if bitmap[i] != 0xff {
			free := bits.TrailingZeros8(^bitmap[i])
			bitIdx := uint32(i)*8 + uint32(free)
			return bitIdx, true
		}
	}

	return 0, false
}

func (fs *FileSystem) Falloc(inode *inode, newSize uint32) error {
	inode.dirty = true

	// keep in mind this function does assume everything is perfect and correct about the inode.
	// its a very low level function, it does not perform any checks.
	// that is for higher level kernel/filesystem commands to enforce and perform on programs
	// making syscalls.

	totalBlocksNeeded := (newSize + BLOCKSIZE - 1) / BLOCKSIZE
	// same: ceil(float(newSize) / float(BLOCKSIZE))

	numOfCurrentBlocks := (inode.size + BLOCKSIZE - 1) / BLOCKSIZE
	// same: ceil(float(inode.size) / float(BLOCKSIZE))

	// if they have enough blocks, even if the size is higher. Eg. size = 10, falloc(20).
	// WE DONT GOTTA DO ANY WORK!!
	// they already have a 4096 block.

	if totalBlocksNeeded == numOfCurrentBlocks {
		return nil
	}

	var findBuf []byte = nil
	var sindBuf []byte = nil
	var tindBuf []byte = nil

	dataBitmapBuf := make([]byte, BLOCKSIZE) // 4096

	defer fs.mu.Unlock()
	fs.mu.Lock()

	fs.disk.ReadAt(dataBitmapBuf, int64(fs.superBlk.dataBitmapStartBlock)*BLOCKSIZE)

	if totalBlocksNeeded > numOfCurrentBlocks {
		// grow path

		const MaxBlocks = 12 + 1024 + 1024 + 1024
		if totalBlocksNeeded > MaxBlocks {
			return errors.New("not enough space to store all that fatass! please just store text and stuff, what are u tryna doo")
		}

		numOfNeededBlocks := totalBlocksNeeded - numOfCurrentBlocks

		if totalBlocksNeeded > 2060 && numOfCurrentBlocks <= 2060 {
			idx, status := findFreeBit(dataBitmapBuf)

			if status != true {
				panic("faahhhhh") // TODO
			}
			setBit(dataBitmapBuf, idx)

			inode.tind = idx
		}
		if totalBlocksNeeded > 1036 && numOfCurrentBlocks <= 1036 {
			idx, status := findFreeBit(dataBitmapBuf)

			if status != true {
				panic("faahhhhh") // TODO
			}
			setBit(dataBitmapBuf, idx)

			inode.sind = idx
		}
		if totalBlocksNeeded > 12 && numOfCurrentBlocks <= 12 {
			idx, status := findFreeBit(dataBitmapBuf)

			if status != true {
				panic("faahhhhh") // TODO
			}

			setBit(dataBitmapBuf, idx)

			inode.find = idx
		}

		for i := 1; i <= int(numOfNeededBlocks); i++ {
			place := 0 // 0 = direct, 1 = find, 2 = sind, 3 = tind

			v := i + int(numOfCurrentBlocks) - 1
			physicalAddress := v

			if v > 11 && v < 1036 {
				physicalAddress = v - 12
				place = 1
			} else if v >= 1036 && v < 2_060 {
				physicalAddress = v - 1036
				place = 2
			} else if v >= 2_060 {
				physicalAddress = v - 2_060
				place = 3
			}

			if place == 0 {
				idx, status := findFreeBit(dataBitmapBuf)
				if !status {
					panic("faahhhhh") // TODO
				}
				setBit(dataBitmapBuf, idx)

				inode.direct[physicalAddress] = idx

			} else if place == 1 {
				if findBuf == nil {
					findBuf = make([]byte, BLOCKSIZE)
					_, _ = fs.disk.ReadAt(findBuf, (int64(inode.find)*BLOCKSIZE)+int64(fs.superBlk.dataBlocksStartBlock*BLOCKSIZE))
				}

				idx, status := findFreeBit(dataBitmapBuf)
				if status != true {
					panic("faahhhhh") // TODO
				}
				setBit(dataBitmapBuf, idx)

				binary.LittleEndian.PutUint32(findBuf[physicalAddress*4:physicalAddress*4+4], idx)

			} else if place == 2 {
				if sindBuf == nil {
					sindBuf = make([]byte, BLOCKSIZE)
					_, _ = fs.disk.ReadAt(sindBuf, (int64(inode.sind)*BLOCKSIZE)+int64(fs.superBlk.dataBlocksStartBlock*BLOCKSIZE))
				}

				idx, status := findFreeBit(dataBitmapBuf)
				if status != true {
					panic("faahhhhh") // TODO
				}
				setBit(dataBitmapBuf, idx)

				binary.LittleEndian.PutUint32(sindBuf[physicalAddress*4:physicalAddress*4+4], idx)
			} else if place == 3 {
				if tindBuf == nil {
					tindBuf = make([]byte, BLOCKSIZE)
					_, _ = fs.disk.ReadAt(tindBuf, (int64(inode.tind)*BLOCKSIZE)+int64(fs.superBlk.dataBlocksStartBlock*BLOCKSIZE))
				}
				idx, status := findFreeBit(dataBitmapBuf)
				if status != true {
					panic("faahhhhh") // TODO
				}
				setBit(dataBitmapBuf, idx)

				binary.LittleEndian.PutUint32(tindBuf[physicalAddress*4:physicalAddress*4+4], idx)
			}

		}

	} else if totalBlocksNeeded < numOfCurrentBlocks {
		// shrink

		for i := totalBlocksNeeded; i < numOfCurrentBlocks; i++ {
			// i = just the virtual block number
			// we need to free all these blocks.

			// convert i to either. direct[x], find[x], sind[x], tind[x]. get that uint32, free it.
			// and 0 the value.

			physicalAddress := i
			place := 0 // 0 = direct, 1 = find, 2 = sind, 3 = tind

			if i > 11 && i < 1036 {
				physicalAddress = i - 12
				place = 1
			} else if i >= 1036 && i < 2_060 {
				physicalAddress = i - 1036
				place = 2
			} else if i >= 2_060 {
				physicalAddress = i - 2_060
				place = 3
			}

			if place == 0 {
				blkAddress := inode.direct[physicalAddress]

				// free that block address
				clearBit(dataBitmapBuf, blkAddress)

				// inode.direct[physicalAddress] = 0 // for visuals in the hex editor,
				// still a valid block number tho

			} else if place == 1 {
				if findBuf == nil {
					findBuf = make([]byte, BLOCKSIZE)
					_, _ = fs.disk.ReadAt(findBuf, (int64(inode.find)*BLOCKSIZE)+int64(fs.superBlk.dataBlocksStartBlock*BLOCKSIZE))
				}

				blkAddress := binary.LittleEndian.Uint32(findBuf[physicalAddress*4 : physicalAddress*4+4])

				// free that block address
				clearBit(dataBitmapBuf, blkAddress)

			} else if place == 2 {
				if sindBuf == nil {
					sindBuf = make([]byte, BLOCKSIZE)
					_, _ = fs.disk.ReadAt(sindBuf, (int64(inode.sind)*BLOCKSIZE)+int64(fs.superBlk.dataBlocksStartBlock*BLOCKSIZE))
				}

				blkAddress := binary.LittleEndian.Uint32(sindBuf[physicalAddress*4 : physicalAddress*4+4])

				// free that block address
				clearBit(dataBitmapBuf, blkAddress)
			} else if place == 3 {
				if tindBuf == nil {
					tindBuf = make([]byte, BLOCKSIZE)
					_, _ = fs.disk.ReadAt(tindBuf, (int64(inode.tind)*BLOCKSIZE)+int64(fs.superBlk.dataBlocksStartBlock*BLOCKSIZE))
				}

				blkAddress := binary.LittleEndian.Uint32(tindBuf[physicalAddress*4 : physicalAddress*4+4])

				// free that block address
				clearBit(dataBitmapBuf, blkAddress)
			}

		}

		// clear the blocks themselves if needed
		if numOfCurrentBlocks > 2060 && totalBlocksNeeded <= 2060 {
			clearBit(dataBitmapBuf, inode.tind)
		}
		if numOfCurrentBlocks > 1036 && totalBlocksNeeded <= 1036 {
			clearBit(dataBitmapBuf, inode.sind)
		}
		if numOfCurrentBlocks > 12 && totalBlocksNeeded <= 12 {
			clearBit(dataBitmapBuf, inode.find)
		}
	}

	// write to disk
	if findBuf != nil {
		fs.disk.WriteAt(findBuf, int64(inode.find)*BLOCKSIZE+int64(fs.superBlk.dataBlocksStartBlock)*BLOCKSIZE)
	}
	if sindBuf != nil {
		fs.disk.WriteAt(sindBuf, int64(inode.sind)*BLOCKSIZE+int64(fs.superBlk.dataBlocksStartBlock)*BLOCKSIZE)
	}
	if tindBuf != nil {
		fs.disk.WriteAt(tindBuf, int64(inode.tind)*BLOCKSIZE+int64(fs.superBlk.dataBlocksStartBlock)*BLOCKSIZE)
	}

	inode.size = newSize
	fs.disk.WriteAt(dataBitmapBuf, int64(fs.superBlk.dataBitmapStartBlock)*BLOCKSIZE)
	return nil
}
