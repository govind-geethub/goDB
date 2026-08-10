package main

import (
	"fmt"
	"log"
)

func main() {

	// initialize Pager(creates the db file)
	pager, err := NewPager("test.db")
	if err != nil {
		log.Fatalf("Failed to initialize pager: %v", err)
	}
	defer pager.Close()

	// create sample rows
	row1 := Row{ID: 1, UserName: "Govind", Email: "govind16kr@gmail.com"}
	row2 := Row{ID: 2, UserName: "Avin", Email: "avin14a@gmail.com"}

	// serialize rows to binary
	bytes1, _ := row1.Serialize()
	bytes2, _ := row2.Serialize()

	// load or create Page 0 (4096 bytes)
	page0, err := pager.ReadPage(0)
	if err != nil {
		log.Fatalf("Failed to read page 0: %v", err)
	}

	// pack rows into slot 0 and slot 1 of Page 0
	copy(page0[0:291], bytes1[:])
	copy(page0[291:582], bytes2[:])

	err = pager.WritePage(0, page0)
	if err != nil {
		log.Fatalf("Failed to write Page 0: %v", err)
	}

	fmt.Println("Successfully wrote 2 rows into Page 0 on test.db!")

	// read page 0 back from the disk and deserialize slot 1(Avin)
	readPage, _ := pager.ReadPage(0)
	extractRow, err := Deserialize(readPage[291:582])
	if err != nil {
		log.Fatalf("Failed to deserialize row: %v", err)
	}

	fmt.Printf("Read from Disk -> ID: %d, User: %s, Email: %s\n", extractRow.ID, extractRow.UserName, extractRow.Email)
}
