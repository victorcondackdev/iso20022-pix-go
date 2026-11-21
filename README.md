# iso20022-pix-go

[![CI](https://github.com/victorcondackdev/iso20022-pix-go/actions/workflows/ci.yml/badge.svg)](https://github.com/victorcondackdev/iso20022-pix-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/victorcondackdev/iso20022-pix-go.svg)](https://pkg.go.dev/github.com/victorcondackdev/iso20022-pix-go)

Typed, validated ISO 20022 messages for Brazil's instant payment system,
Pix, as exchanged through the SPI (Sistema de Pagamentos Instantâneos).
Standard library only.

The library covers the message flow a Pix participant handles every day:
the payment (`pacs.008`), its acceptance or rejection (`pacs.002`), the
return (`pacs.004`, including MED returns) and the account notification
(`camt.054`). It grew out of running these messages in production, where
the expensive bugs are rarely about XML and almost always about amounts
that drift by a centavo, identifiers that are one character short, returns
that exceed what was paid and validation errors that surface one at a
time.

```
go get github.com/victorcondackdev/iso20022-pix-go
```

## Quick start

Detect what arrived, parse it, validate it, answer it:

```go
mt, err := iso20022.Detect(payload) // reads only up to <Document>
if err != nil {
	return err
}

switch mt.ID() {
case iso20022.Pacs008:
	payment, err := pacs008.Parse(payload)
	if err != nil {
		return err
	}
	if err := payment.Validate(); err != nil {
		var vs xsd.Violations
		errors.As(err, &vs) // every violation, with its location
		report, _ := pacs002.Reject(payment, "BE17")
		return send(report)
	}
	report, err := pacs002.Accept(payment, pacs002.SettledAt(time.Now()))
	// ...
}
```

Build a payment:

```go
doc, err := pacs008.NewBuilder(time.Now()).
	Add(pacs008.Transfer{
		Amount:          money.MustParse("150.75"),
		Initiation:      pacs008.InitiationDICT,
		Debtor:          pacs008.Party{Name: "Fulano de Tal", TaxID: "52998224725"},
		DebtorAccount:   pacs008.Account{ISPB: "99999999", Branch: "0001", Number: "123456", Type: xsd.AccountCurrent},
		Creditor:        pacs008.Party{Name: "Loja Exemplo Ltda", TaxID: "11222333000181"},
		CreditorAccount: pacs008.Account{ISPB: "88888888", Branch: "0001", Number: "6543210", Type: xsd.AccountCurrent},
		PixKey:          "pagamentos@example.com",
		RemittanceInfo:  "Pedido 4521",
	}).
	Build() // generates MsgId and EndToEndId, then validates
xml, err := doc.Marshal()
```

Return part of it:

```go
ret, err := pacs004.FromPacs008(payment, pacs004.ReasonCustomerRefund, money.MustParse("50.00"),
	pacs004.WithPriorReturns(alreadyReturned)) // ErrAmountExceeded past the original amount
```

Validation reports everything at once:

```
/Document/FIToFICstmrCdtTrf/CdtTrfTxInf[0]/PmtId/EndToEndId: must be 32 characters, got 4
/Document/FIToFICstmrCdtTrf/CdtTrfTxInf[0]/IntrBkSttlmAmt/@Ccy: must be "BRL", got "USD"
/Document/FIToFICstmrCdtTrf/CdtTrfTxInf[0]/Dbtr/Id/PrvtId/Othr[0]/Id: must be a valid CPF, got "12345678900"
```

## Message coverage

| Message  | ISO name                              | Layout follows | Parse / Marshal | Validate | Helpers                                   |
|----------|---------------------------------------|----------------|-----------------|----------|-------------------------------------------|
| pacs.008 | FIToFICustomerCreditTransfer          | .001.08        | yes             | yes      | `Builder`                                 |
| pacs.002 | FIToFIPaymentStatusReport             | .001.10        | yes             | yes      | `Accept`, `Reject`, `Reasons` (EN / PT)   |
| pacs.004 | PaymentReturn                         | .001.09        | yes             | yes      | `FromPacs008`, `Reasons` (EN / PT)        |
| camt.054 | BankToCustomerDebitCreditNotification | .001.08        | yes             | yes      | `Entries`, `EndToEndIDs`, `Signed`        |
| head.001 | BusinessApplicationHeader (AppHdr)    | .001.01        | via `Inspect`   | yes      | `NewAppHdr`, MsgDefIdr consistency check  |

`Parse` accepts any version of a message (`pacs.008.001.xx`) and either a
bare `Document` or a business message that wraps an `AppHdr` and a
`Document`. Only the elements Pix uses are modelled; others are ignored on
parse and therefore not re-emitted.

Supporting packages:

| Package | Purpose |
|---------|---------|
| `money` | BRL amounts as `int64` centavos; strict ISO 20022 decimal parsing and canonical formatting |
| `ids`   | EndToEndId and RtrId (parse/generate), MsgId, ISPB, CPF, CNPJ (numeric and alphanumeric), Pix keys |
| `xsd`   | Shared ISO 20022 components (parties, accounts, agents, amounts, dates) and `Violations` |

## Command line

```
go install github.com/victorcondackdev/iso20022-pix-go/cmd/iso20022@latest

iso20022 inspect  pacs008.xml                         # detect and print key fields
iso20022 validate pacs008.xml                         # list every violation, exit 1 if any
iso20022 return -reason MD06 -amount 10.00 pacs008.xml > pacs004.xml
```

`-` reads from standard input. Sample messages live in [`testdata/`](testdata).

## Design notes

**Centavos, never floats.** `money.Amount` is an `int64` number of
centavos. `Parse` accepts exactly the lexical forms an ISO 20022 BRL amount
can take: digits, an optional point and at most two fraction digits, at
most 18 digits in total. No sign, exponent, whitespace or comma. Values
that would not survive a round trip are rejected instead of rounded.

**Strict at the edges, lenient where the catalog evolves.** Parsing rejects
malformed XML, DOCTYPE and entity declarations, a namespace of the wrong
message, a second `Document` and malformed amounts. Validation enforces the
ISO 20022 types (Max35Text, Max140Text, ISODateTime, code choices) and the
Pix rules that are stable: BRL, `CLRG`, `SLEV`, 8-digit ISPBs, the
EndToEndId and RtrId layouts, CPF and CNPJ check digits, Pix key shapes.
Reason codes outside the bundled tables are kept and accepted, because the
SPI catalog changes faster than libraries do.

**Every violation, with a location.** `Validate` walks the whole message
and returns `xsd.Violations`, each with an XPath-like path that matches the
XML element names, so a rejected message can be fixed in one pass and the
paths can go straight into logs or a `pacs.002` `AddtlInf`.

**Streaming detection.** A gateway receiving mixed traffic needs to know
the message type before choosing a parser. `Detect` drives an
`xml.Decoder` token by token and stops at the `Document` start tag, so its
cost does not grow with the message and the body is never unmarshalled
twice. It reads the version from the namespace and skips any envelope or
`AppHdr` in front of the document.

**Round trips are a tested property.** Fuzz tests check that every message
`Parse` accepts survives `Marshal` and `Parse` again unchanged, and that
every document the builder produces and `Validate` accepts does the same.
Golden files in `testdata/` are byte-identical to the marshaller output.
Validation also rejects characters XML 1.0 cannot carry, so a valid
document can always be serialised faithfully.

**Field names mirror the XML.** Struct fields are named after the ISO
20022 tags (`MsgId`, `CdtTrfTxInf`, `Dbtr`, `Othr`), so the code reads like
the message definition report and the SPI catalog. Pix-oriented builders
(`pacs008.Transfer`, `pacs004.FromPacs008`) sit on top for everyday use.

## Scope and disclaimer

This project is not affiliated with, endorsed by or derived from documents
of the Banco Central do Brasil. It models ISO 20022 elements as defined by
the ISO 20022 message definitions and the Pix conventions in public
regulation. The BCB publishes SPI catalogs that further restrict these
messages (allowed elements, cardinalities, code lists, lengths and
versions) and revises them over time. Validate production traffic against
the current catalog; `Validate` here is a strong first line, not a
certification. Sample ISPBs, CPFs and CNPJs in this repository are
fictitious or publicly known test values.

## License

[MIT](LICENSE)
