package main

import (
	"fmt"
	"log"
)

func main() {
	originalRow := Row{
		ID:       1,
		UserName: "Govind",
		Email:    "aviga14@gmail.com",
	}

	// test serialize
	serializedData, err := originalRow.Serialize()
	if err != nil {
		log.Fatalf("Serialization failed: %v", err)
	}
	fmt.Printf("Serialized Row Length: %d bytes\n", len(serializedData))

	// test deserialize
	deserializedRow, err := Deserialize(serializedData[:])
	if err != nil {
		log.Fatalf("Desrerialization failed: %v", err)
	}
	fmt.Printf("Desreialized Row -> ID: %d, User: %s, Email: %s\n", deserializedRow.ID, deserializedRow.UserName, deserializedRow.Email)
}
