# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [0.1.0] - 2026-11-21

### Added

- `pacs008`: FIToFICustomerCreditTransfer model (pacs.008.001.08 layout),
  `Parse`, `Marshal`, `Validate` and a Pix-oriented `Builder`.
- `pacs002`: FIToFIPaymentStatusReport model (pacs.002.001.10 layout),
  `Accept` and `Reject` helpers and a bilingual table of common status
  reason codes.
- `pacs004`: PaymentReturn model (pacs.004.001.09 layout) and
  `FromPacs008`, which builds full or partial returns and refuses to return
  more than the original amount, including across prior partial returns.
- `camt054`: BankToCustomerDebitCreditNotification model
  (camt.054.001.08 layout) with entry and reference helpers.
- `ids`: EndToEndId and RtrId parsing and generation, MsgId helpers, ISPB,
  CPF, numeric and alphanumeric CNPJ validation and Pix key classification.
- `money`: exact BRL amounts in centavos with strict ISO 20022 decimal
  parsing.
- Root package: streaming message detection (`Detect`, `DetectReader`) and
  head.001 application header support (`Inspect`, `AppHdr`).
- `cmd/iso20022`: `inspect`, `validate` and `return` commands.
- Validation that reports every violation with an XPath-like location.
- Golden sample messages, example tests and fuzz tests for parsing,
  round trips, identifiers and amounts.

[0.1.0]: https://github.com/victorcondackdev/iso20022-pix-go/releases/tag/v0.1.0
