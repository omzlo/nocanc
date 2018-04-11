package intelhex

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"github.com/omzlo/nocand/clog"
	"io"
)

const (
	DataRecord                   = 0
	EndOfFileRecord              = 1
	ExtendedSegmentAddressRecord = 2
	StartSegmentAddressRecord    = 3
	ExtendedLinearAddressRecord  = 4
	StartLinearAddressRecord     = 5
)

type IntelHexMemBlock struct {
	Type    uint8
	Address uint32
	Data    []byte
}

func (block *IntelHexMemBlock) Copy(dest []byte, offset uint32, maxlen uint32) uint32 {
	if offset > uint32(len(block.Data)) {
		return 0
	}

	var clen uint32
	if offset+maxlen <= uint32(len(block.Data)) {
		clen = maxlen
	} else {
		clen = uint32(len(block.Data)) - offset
	}
	copy(dest, block.Data[offset:offset+clen])
	return clen
}

func (block *IntelHexMemBlock) Trim(trim_char byte) uint32 {
	var count uint32 = 0

	for blen := uint32(len(block.Data) - 1); blen > 0; blen-- {
		if block.Data[blen-1] == trim_char {
			block.Data = block.Data[:blen-1]
			count++
		} else {
			break
		}
	}
	return count
}

type IntelHex struct {
	Size   uint
	Blocks []*IntelHexMemBlock
}

func New() *IntelHex {
	return &IntelHex{Size: 0, Blocks: make([]*IntelHexMemBlock, 0, 8)}
}

func (ihex *IntelHex) Add(btype uint8, address uint32, data []byte) {

	//clog.Debug("ADD %d @%x len=%d",btype,address,len(data))
	for i, block := range ihex.Blocks {
		if btype == block.Type && address == block.Address+uint32(len(block.Data)) {
			ihex.Blocks[i].Data = append(block.Data, data...)
			ihex.Size += uint(len(data))
			return
		}
	}
	block := &IntelHexMemBlock{btype, address, make([]byte, len(data))}
	copy(block.Data, data)
	ihex.Blocks = append(ihex.Blocks, block)
	ihex.Size += uint(len(data))
}

func (ihex *IntelHex) Load(r io.Reader) error {
	var (
		byte_count       uint8
		address          uint32
		btype            uint8
		extended_address uint32 = 0
	)
	line_count := 0

	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line_count++
		line := scanner.Text()

		if line[0] != ':' {
			return fmt.Errorf("Missing ':' at the beginning of line %d", line_count)
		}
		data, err := hex.DecodeString(line[1:])
		if err != nil {
			return fmt.Errorf("Failed to decode hex file data on line %d: %s", line_count, err.Error())
		}
		if len(data) < 5 || len(data) != (5+int(data[0])) {
			return fmt.Errorf("Missing data in hexfile on line %d", line_count)
		}
		byte_count = data[0]
		address = (uint32(data[1]) << 8) | uint32(data[2])
		btype = data[3]
		checksum := data[0]
		for i := 1; i < len(data)-1; i++ {
			checksum += data[i]
		}
		checksum = (^checksum) + 1
		if checksum != data[len(data)-1] {
			return fmt.Errorf("Checksum error on line %d, expected %02x but got %02x", line_count, data[len(data)-1], checksum)
		}
		switch btype {
		case 0:
			ihex.Add(btype, extended_address+address, data[4:4+byte_count])
		case 1:
			if byte_count != 0 {
				return fmt.Errorf("End of file marker has non zero length on line %d", line_count)
			}
			return nil
		case 2:
			if byte_count != 2 {
				return fmt.Errorf("Extended segment address record should be of length 2 on line %d", line_count)
			}
			extended_address = (uint32(data[4]) << 12) | (uint32(data[5]) << 4)
		case 3:
			if byte_count != 4 {
				return fmt.Errorf("Start segment address directive is of incorrect length on line %d", line_count)
			} else {
				clog.Debug("Ignoring Start Segment Address directive with value %04x on line %d in firmware",
					(uint32(data[4])<<24)|(uint32(data[5])<<16)|(uint32(data[6])<<8)|uint32(data[7]), line_count)
			}
		case 4:
			if byte_count != 2 {
				return fmt.Errorf("Extended linear address record should be of length 2 on line %d", line_count)
			}
			extended_address = (uint32(data[4]) << 24) | (uint32(data[5]) << 16)
		case 5:
			if byte_count != 4 {
				return fmt.Errorf("Start linear address directive is of incorrect length on line %d", line_count)
			} else {
				clog.Debug("Ignoring Start Linear Address directive with value %04x on line %d in firmware",
					(uint32(data[4])<<24)|(uint32(data[5])<<16)|(uint32(data[6])<<8)|uint32(data[7]), line_count)
			}
		default:
			clog.Warning("Firmware contains a block of unknown type %02x on line %d, whihc will be ignored.", btype, line_count)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("Failed to read next data after line %d: %s", line_count, err.Error())
	}
	return fmt.Errorf("Unexpected end of file on line %d", line_count)
}

func (hex *IntelHex) Save(w io.Writer) error {
	var extended_address uint32 = 0
	var checksum uint8

	for _, block := range hex.Blocks {

		if extended_address != (block.Address >> 16) {
			extended_address = (block.Address >> 16)
			checksum = ^(0x02 + 0x00 + 0x00 + 0x04 + uint8(extended_address>>8) + uint8(extended_address&0xFF)) + 1
			fmt.Fprintf(w, ":02000004%02X%02X%02X\n", (extended_address >> 8), (extended_address & 0xFF), checksum)
		}

		var pos uint32 = 0
		var length uint32 = uint32(len(block.Data))
		for pos < length {
			var blen uint8
			var i uint32

			address := block.Address + pos

			if length-pos < 16 {
				blen = uint8(length - pos)
			} else {
				blen = 16
			}

			checksum = blen + uint8((address>>8)&0xFF) + uint8((address)&0xFF) + 0x00
			fmt.Fprintf(w, ":%02X%02X%02X00", blen, ((address >> 8) & 0xFF), ((address) & 0xFF))
			for i = 0; i < uint32(blen); i++ {
				fmt.Fprintf(w, "%02X", block.Data[pos+i])
				checksum += block.Data[pos+i]
			}
			fmt.Fprintf(w, "%02X\n", ((^checksum)+1)&0xFF)
			pos += uint32(blen)
		}
	}
	fmt.Fprintf(w, ":00000001FF")
	return nil
}

func (hex *IntelHex) IterateBlocks(fn func(uint8, uint32, []byte, interface{}) error, extra interface{}) {

}
