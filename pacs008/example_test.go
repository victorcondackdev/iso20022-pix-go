package pacs008_test

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/victorcondackdev/iso20022-pix-go/money"
	"github.com/victorcondackdev/iso20022-pix-go/pacs008"
	"github.com/victorcondackdev/iso20022-pix-go/xsd"
)

func ExampleBuilder() {
	at := time.Date(2026, 11, 21, 10, 30, 0, 0, time.UTC)
	doc, err := pacs008.NewBuilder(at).
		MsgID("M9999999920261121103000000000001").
		Add(pacs008.Transfer{
			EndToEndID:      "E99999999202611211030a1B2c3D4e5F",
			Amount:          money.MustParse("150.75"),
			Initiation:      pacs008.InitiationDICT,
			Debtor:          pacs008.Party{Name: "Fulano de Tal", TaxID: "52998224725"},
			DebtorAccount:   pacs008.Account{ISPB: "99999999", Branch: "0001", Number: "123456", Type: xsd.AccountCurrent},
			Creditor:        pacs008.Party{Name: "Loja Exemplo Ltda", TaxID: "11222333000181"},
			CreditorAccount: pacs008.Account{ISPB: "88888888", Branch: "0001", Number: "6543210", Type: xsd.AccountCurrent},
			PixKey:          "pagamentos@example.com",
			RemittanceInfo:  "Pedido 4521",
		}).
		Build()
	if err != nil {
		panic(err)
	}
	tx := doc.Transactions()[0]
	fmt.Println(doc.MessageName(), tx.PmtId.EndToEndId, tx.IntrBkSttlmAmt)
	fmt.Println(tx.Dbtr.TaxID(), "->", tx.CdtrAcct.PixKey())
	// Output:
	// pacs.008.001.08 E99999999202611211030a1B2c3D4e5F 150.75 BRL
	// 52998224725 -> pagamentos@example.com
}

func ExampleDocument_Validate() {
	data, _ := os.ReadFile("../testdata/pacs008.xml")
	doc, _ := pacs008.Parse(data)
	tx := &doc.FIToFICstmrCdtTrf.CdtTrfTxInf[0]
	tx.PmtId.EndToEndId = "E123"
	tx.IntrBkSttlmAmt.Ccy = "USD"
	tx.Dbtr.Id.PrvtId.Othr[0].Id = "12345678900"

	var vs xsd.Violations
	if errors.As(doc.Validate(), &vs) {
		for _, v := range vs {
			fmt.Println(v)
		}
	}
	// Output:
	// /Document/FIToFICstmrCdtTrf/CdtTrfTxInf[0]/PmtId/EndToEndId: must be 32 characters, got 4
	// /Document/FIToFICstmrCdtTrf/CdtTrfTxInf[0]/IntrBkSttlmAmt/@Ccy: must be "BRL", got "USD"
	// /Document/FIToFICstmrCdtTrf/CdtTrfTxInf[0]/Dbtr/Id/PrvtId/Othr[0]/Id: must be a valid CPF, got "12345678900"
}
