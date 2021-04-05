package random

import (
	crand "crypto/rand"

	"encoding/binary"
	"log"
)

type Source struct{}

func (s Source) Seed(seed int64) {}

func (s Source) Int63() int64 {
	return int64(s.Uint64() & ^uint64(1<<63))
}

func (s Source) Uint64() (v uint64) {
	err := binary.Read(crand.Reader, binary.BigEndian, &v)
	if err != nil {
		log.Fatal(err)
	}
	return v
}
