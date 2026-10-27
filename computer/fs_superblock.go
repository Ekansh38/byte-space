package computer

import "encoding/binary"

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
