package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
)

const (
	MaxUsernameLen = 32
	MaxEmailLen    = 255
	RowSize        = 4 + MaxEmailLen + MaxUsernameLen // 291 byte total
)

// single record in my database
type Row struct {
	ID       uint32
	UserName string
	Email    string
}

// func : struct -> raw bytes
func (r *Row) Serialize() ([RowSize]byte, error) {
	var buf bytes.Buffer

	// ID into 4 bytes
	// for the binary layout it is used
	err := binary.Write(&buf, binary.LittleEndian, r.ID)
	if err != nil {
		return [RowSize]byte{}, fmt.Errorf("failed to write ID: %w", err)
	}

	// packing username into 32 bytes
	userBytes := make([]byte, MaxUsernameLen)
	copy(userBytes, r.UserName)
	buf.Write(userBytes)

	// packing Email 255 bytes
	emailBytes := make([]byte, MaxEmailLen)
	copy(emailBytes, r.Email)
	buf.Write(emailBytes)

	var result [RowSize]byte
	copy(result[:], buf.Bytes())

	// 291 bytes array returned
	return result, nil
}

func Deserialize(data []byte) (*Row, error) {
	reader := bytes.NewReader(data)
	var row Row

	// unpacking ID
	err := binary.Read(reader, binary.LittleEndian, &row.ID)
	if err != nil {
		return nil, fmt.Errorf("Failed to read the Username: %w", err)
	}

	// unpacking Username
	userBytes := make([]byte, MaxUsernameLen)
	_, err = reader.Read(userBytes)
	if err != nil {
		return nil, fmt.Errorf("Failed to read Username: %w", err)
	}
	// removing the 0s placed in remaining spaces of 32 bytes
	row.UserName = strings.TrimRight(string(userBytes), "\x00")

	// unpacking Email
	emailBytes := make([]byte, MaxEmailLen)
	_, err = reader.Read(emailBytes)
	if err != nil {
		return nil, fmt.Errorf("Failed to read the Email: %w", err)
	}

	row.Email = strings.TrimRight(string(emailBytes), "\x00")

	return &row, nil
}
