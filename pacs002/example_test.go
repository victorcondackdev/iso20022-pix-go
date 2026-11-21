package pacs002_test

import (
	"fmt"
	"os"
	"time"

	"github.com/victorcondackdev/iso20022-pix-go/pacs002"
	"github.com/victorcondackdev/iso20022-pix-go/pacs008"
)

func ExampleReject() {
	data, _ := os.ReadFile("../testdata/pacs008.xml")
	payment, _ := pacs008.Parse(data)

	report, err := pacs002.Reject(payment, "AM04", pacs002.At(time.Date(2026, 11, 21, 10, 30, 2, 0, time.UTC)))
	if err != nil {
		panic(err)
	}
	tx := report.Transactions()[0]
	info, _ := pacs002.Reasons.Lookup(tx.Reason())
	fmt.Println(tx.OrgnlEndToEndId, tx.TxSts, tx.Reason())
	fmt.Println(info.English)
	fmt.Println(info.Portuguese)
	// Output:
	// E99999999202611211030a1B2c3D4e5F RJCT AM04
	// Insufficient funds to cover the amount.
	// Saldo insuficiente para cobrir o valor.
}

func ExampleAccept() {
	data, _ := os.ReadFile("../testdata/pacs008.xml")
	payment, _ := pacs008.Parse(data)
	report, _ := pacs002.Accept(payment, pacs002.SettledAt(time.Date(2026, 11, 21, 10, 30, 1, 0, time.UTC)))
	tx := report.Transactions()[0]
	fmt.Println(tx.TxSts, tx.Accepted(), tx.FctvIntrBkSttlmDt.DtTm)
	// Output: ACSC true 2026-11-21T10:30:01.000Z
}
