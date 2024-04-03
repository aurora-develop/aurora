package funcaptcha

import (
	"encoding/binary"
	"fmt"
)

type digest struct {
	h1, h2 uint64
	length int
	seed   uint64
}

func GetMurmur128String(input string, seed uint64) string {
	d := NewWithSeed(seed)
	d.Write([]byte(input))
	h1, h2 := d.Sum()
	return fmt.Sprintf("%x%x", h1, h2)
}

func NewWithSeed(seed uint64) *digest {
	d := new(digest)
	d.seed = seed
	d.h1 = seed
	d.h2 = seed
	return d
}

func (d *digest) Write(data []byte) {
	length := len(data)
	d.length += length

	var (
		h1 = d.h1
		h2 = d.h2
		c1 = uint64(0x87c37b91114253d5)
		c2 = uint64(0x4cf5ad432745937f)
	)

	for len(data) >= 16 {
		k1 := binary.LittleEndian.Uint64(data)
		k2 := binary.LittleEndian.Uint64(data[8:])

		k1 *= c1
		k1 = (k1 << 31) | (k1 >> (64 - 31))
		k1 *= c2
		h1 ^= k1

		h1 = (h1 << 27) | (h1 >> (64 - 27))
		h1 += h2
		h1 = h1*5 + 0x52dce729

		k2 *= c2
		k2 = (k2 << 33) | (k2 >> (64 - 33))
		k2 *= c1
		h2 ^= k2

		h2 = (h2 << 31) | (h2 >> (64 - 31))
		h2 += h1
		h2 = h2*5 + 0x38495ab5

		data = data[16:]
	}

	var k1, k2 uint64

	switch len(data) {
	case 15:
		k2 ^= uint64(data[14]) << 48
		fallthrough
	case 14:
		k2 ^= uint64(data[13]) << 40
		fallthrough
	case 13:
		k2 ^= uint64(data[12]) << 32
		fallthrough
	case 12:
		k2 ^= uint64(data[11]) << 24
		fallthrough
	case 11:
		k2 ^= uint64(data[10]) << 16
		fallthrough
	case 10:
		k2 ^= uint64(data[9]) << 8
		fallthrough
	case 9:
		k2 ^= uint64(data[8])
		k2 *= c2
		k2 = (k2 << 33) | (k2 >> (64 - 33))
		k2 *= c1
		h2 ^= k2

		fallthrough
	case 8:
		k1 ^= uint64(data[7]) << 56
		fallthrough
	case 7:
		k1 ^= uint64(data[6]) << 48
		fallthrough
	case 6:
		k1 ^= uint64(data[5]) << 40
		fallthrough
	case 5:
		k1 ^= uint64(data[4]) << 32
		fallthrough
	case 4:
		k1 ^= uint64(data[3]) << 24
		fallthrough
	case 3:
		k1 ^= uint64(data[2]) << 16
		fallthrough
	case 2:
		k1 ^= uint64(data[1]) << 8
		fallthrough
	case 1:
		k1 ^= uint64(data[0])
		k1 *= c1
		k1 = (k1 << 31) | (k1 >> (64 - 31))
		k1 *= c2
		h1 ^= k1
	}

	h1 ^= uint64(length)
	h2 ^= uint64(length)

	h1 += h2
	h2 += h1

	h1 = fmix(h1)
	h2 = fmix(h2)

	h1 += h2
	h2 += h1
	d.h1 = h1
	d.h2 = h2
}

func (d *digest) Sum() (h1, h2 uint64) {
	return d.h1, d.h2
}

func fmix(k uint64) uint64 {
	k ^= k >> 33
	k *= 0xff51afd7ed558ccd
	k ^= k >> 33
	k *= 0xc4ceb9fe1a85ec53
	k ^= k >> 33
	return k
}
