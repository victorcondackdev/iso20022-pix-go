package camt054_test

import (
	"fmt"
	"os"

	"github.com/victorcondackdev/iso20022-pix-go/camt054"
)

func ExampleParse() {
	data, _ := os.ReadFile("../testdata/camt054.xml")
	doc, err := camt054.Parse(data)
	if err != nil {
		panic(err)
	}
	for _, e := range doc.Entries() {
		fmt.Println(e.CdtDbtInd, e.Amt.Value, e.Signed(), e.EndToEndIDs())
	}
	// Output:
	// CRDT 150.75 150.75 [E99999999202611211030a1B2c3D4e5F]
	// DBIT 50.00 -50.00 [E99999999202611211030a1B2c3D4e5F]
}
