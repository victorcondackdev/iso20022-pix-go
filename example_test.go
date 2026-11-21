package iso20022_test

import (
	"fmt"
	"os"

	iso20022 "github.com/victorcondackdev/iso20022-pix-go"
)

func ExampleDetect() {
	data, _ := os.ReadFile("testdata/pacs004.xml")
	mt, err := iso20022.Detect(data)
	if err != nil {
		panic(err)
	}
	fmt.Println(mt, mt.Name(), mt.Version)
	fmt.Println(mt.ID() == iso20022.Pacs004)
	// Output:
	// pacs.004.001.09 PaymentReturn 09
	// true
}

func ExampleInspect() {
	data, _ := os.ReadFile("testdata/envelope_pacs008.xml")
	env, err := iso20022.Inspect(data)
	if err != nil {
		panic(err)
	}
	fmt.Println(env.Type)
	fmt.Println(env.Header.Fr.ISPB(), "->", env.Header.To.ISPB())
	fmt.Println(env.Validate())
	// Output:
	// pacs.008.001.08
	// 99999999 -> 00038166
	// <nil>
}
