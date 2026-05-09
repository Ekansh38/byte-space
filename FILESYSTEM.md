# Disk Stats

Total disk size: 64 MB (67108864 bytes)
Block size: 0x1000 bytes (4096 bytes)
Total blocks: 16384

LAYOUT

Component Start Block Blocks Bytes
Super Block 0 1 4,096
Inode Bitmap 1 1 4,096
Inode Table 2 252 1,032,192
Data Bitmap 254 1 4,096
Data Blocks 255 16,128 66,060,288
Padding 16,383 1 4,096
------ -----------
TOTAL 16,384 67,108,864

## CAPACITIES

Inodes: 8,064
Data blocks: 16,128
Ratio: 2.0 data blocks per inode (exact)

## PROOF

Target ratio: 1 inode : 2 data blocks
Inode size: 128 bytes
Data block size: 4,096 bytes
Cost per inode: 128 bytes inode + 8,192 bytes data = 8,320 bytes total

Total blocks: 16,384
Reserve for super + bitmaps: 1 + 1 + 1 = 3 blocks
Available: 16,384 - 3 = 16,381 blocks = 67,084,288 bytes

Inodes: 67,084,288 / 8,320 = 8,064.068...
Round down: 8,064 inodes

Inode table blocks: 8,064 × 128 / 4,096 = 252 blocks
Data blocks: 8,064 × 2 = 16,128 blocks

Structure total: 1 + 1 + 252 + 1 = 255 blocks
Data blocks: 16,128 blocks
Total used: 255 + 16,128 = 16,383 blocks
Padding: 16,384 - 16,383 = 1 block ✓
