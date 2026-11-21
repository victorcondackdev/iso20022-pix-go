package ids_test

import (
	"bytes"
	"fmt"
	"time"

	"github.com/victorcondackdev/iso20022-pix-go/ids"
)

func ExampleParseEndToEndID() {
	id, err := ids.ParseEndToEndID("E99999999202611211030a1B2c3D4e5F")
	if err != nil {
		panic(err)
	}
	fmt.Println(id.ISPB, id.Time.Format(time.RFC3339), id.Sequence)
	// Output: 99999999 2026-11-21T10:30:00Z a1B2c3D4e5F
}

func ExampleNewReturnID() {
	at := time.Date(2026, 11, 21, 12, 0, 0, 0, time.FixedZone("BRT", -3*3600))
	// A fixed byte source makes the example deterministic; pass nil in
	// production to use crypto/rand.
	rnd := bytes.NewReader(bytes.Repeat([]byte{10, 36}, 16))
	id, _ := ids.NewReturnID("88888888", at, rnd)
	fmt.Println(id)
	// Output: D88888888202611211500AaAaAaAaAaA
}

func ExampleClassifyPixKey() {
	for _, k := range []string{"52998224725", "+5521987654321", "Fulano@Example.com"} {
		t, err := ids.ClassifyPixKey(k)
		fmt.Println(t, err)
	}
	// Output:
	// CPF <nil>
	// PHONE <nil>
	// UNKNOWN ids: invalid Pix key: e-mail key "Fulano@Example.com" must be lowercase ASCII without spaces
}
