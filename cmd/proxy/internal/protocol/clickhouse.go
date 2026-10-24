package protocol

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
)

const maxCHString = 4096

// ClickHouse reads the native protocol Hello packet, which carries the
// database, user and password in the first client message.
type ClickHouse struct{}

type chHello struct {
	clientName               string
	major, minor, revision   uint64
	database, user, password string
}

func (ClickHouse) Read(r *bufio.Reader) (string, func(string) []byte, error) {
	if t, err := binary.ReadUvarint(r); err != nil || t != 0 {
		return "", nil, errors.New("first packet must be a ClickHouse Hello")
	}
	var h chHello
	var err error
	if h.clientName, err = readCHString(r); err != nil {
		return "", nil, err
	}
	for _, v := range []*uint64{&h.major, &h.minor, &h.revision} {
		if *v, err = binary.ReadUvarint(r); err != nil {
			return "", nil, err
		}
	}
	for _, s := range []*string{&h.database, &h.user, &h.password} {
		if *s, err = readCHString(r); err != nil {
			return "", nil, err
		}
	}
	forward := func(plain string) []byte {
		h.user = plain
		return h.encode()
	}
	return h.user, forward, nil
}

func readCHString(r *bufio.Reader) (string, error) {
	n, err := binary.ReadUvarint(r)
	if err != nil {
		return "", err
	}
	if n > maxCHString {
		return "", errors.New("string too long")
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return "", err
	}
	return string(b), nil
}

func (h chHello) encode() []byte {
	b := binary.AppendUvarint(nil, 0)
	str := func(s string) { b = append(binary.AppendUvarint(b, uint64(len(s))), s...) }
	str(h.clientName)
	for _, v := range []uint64{h.major, h.minor, h.revision} {
		b = binary.AppendUvarint(b, v)
	}
	str(h.database)
	str(h.user)
	str(h.password)
	return b
}
