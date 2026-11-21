package money_test

import (
	"fmt"

	"github.com/victorcondackdev/iso20022-pix-go/money"
)

func ExampleParse() {
	for _, s := range []string{"10.5", "0.01", "1.234", "-5"} {
		a, err := money.Parse(s)
		if err != nil {
			fmt.Println("error:", err)
			continue
		}
		fmt.Println(a.Cents(), a)
	}
	// Output:
	// 1050 10.50
	// 1 0.01
	// error: money: invalid amount: "1.234" has more than 2 fraction digits
	// error: money: invalid amount: "-5" contains '-'
}

func ExampleAmount_Add() {
	sum, _ := money.MustParse("0.1").Add(money.MustParse("0.2"))
	fmt.Println(sum)
	// Output: 0.30
}
