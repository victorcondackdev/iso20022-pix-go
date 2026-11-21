package pacs004_test

import (
	"errors"
	"fmt"
	"os"

	"github.com/victorcondackdev/iso20022-pix-go/money"
	"github.com/victorcondackdev/iso20022-pix-go/pacs004"
	"github.com/victorcondackdev/iso20022-pix-go/pacs008"
)

func ExampleFromPacs008() {
	data, _ := os.ReadFile("../testdata/pacs008.xml") // a R$ 150.75 payment
	payment, _ := pacs008.Parse(data)

	ret, err := pacs004.FromPacs008(payment, pacs004.ReasonCustomerRefund, money.MustParse("50.00"))
	if err != nil {
		panic(err)
	}
	tx := ret.Transactions()[0]
	fmt.Println(tx.OrgnlEndToEndId, tx.RtrdIntrBkSttlmAmt, tx.Reason(), tx.RtrId[:9])

	// A second partial return cannot exceed what is left.
	_, err = pacs004.FromPacs008(payment, pacs004.ReasonCustomerRefund, money.MustParse("100.76"),
		pacs004.WithPriorReturns(money.MustParse("50.00")))
	fmt.Println(errors.Is(err, pacs004.ErrAmountExceeded))
	fmt.Println(err)
	// Output:
	// E99999999202611211030a1B2c3D4e5F 50.00 BRL MD06 D88888888
	// true
	// pacs004: return amount exceeds the amount available for return: 100.76 requested, 100.75 available of the original 150.75
}
