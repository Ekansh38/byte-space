

## Overview

Disk size         :   64 MB        :   0x400_0000    (bytes)
Block size        :   4096 bytes   :   0x1000        (bytes)
Total blocks      :   16_384       :   0x4000        (count)

Inode count       :   8_064        :   0x1F80        (count)
Data block count  :   16_128       :   0x3F00        (count)
Ratio             :   2:1          :   N/A           (ratio)


## Layout

```text
          64MB                   
+----------------------+
| Super Block          |
+----------------------+
| Inode Bitmap         |
+----------------------+
| Inode Table          |
|                      |
|                      |
+----------------------+
| Data Bitmap          |
+----------------------+
| Data Blocks          |
|                      |
|                      |
|                      |
|                      |
|                      |
+----------------------+
| Padding              |
+----------------------+
```

```text
| Component    | Start Block | Blocks | Bytes      |
|--------------|-------------|--------|------------|
| Super Block  | 0           | 1      | 4096       |
| Inode Bitmap | 1           | 1      | 4096       |
| Inode Table  | 2           | 252    | 1032192    |
| Data Bitmap  | 254         | 1      | 4096       |
| Data Blocks  | 255         | 16128  | 66060288   |
| Padding      | 16383       | 1      | 4096       |
| TOTAL        | N/A         | 16384  | 67,108,864 |
```


## Reasoning/Proof

Target ratio: 1 inode : 2 data blocks

Inode size: 128 bytes

Data block size: 4096 bytes

Cost per inode: 128 + 2(4096)  = 8,320 bytes

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
