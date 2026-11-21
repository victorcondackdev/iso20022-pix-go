package pacs004

import "github.com/victorcondackdev/iso20022-pix-go/xsd"

// Return reason codes (ExternalReturnReason1Code) used for Pix returns.
const (
	ReasonBankError       = "BE08" // returned because of a participant error
	ReasonFraud           = "FR01" // returned because of fraud (e.g. MED)
	ReasonCustomerRefund  = "MD06" // refund requested by the end customer
	ReasonCreditorService = "SL02" // specific service offered by the creditor agent
)

// Reasons lists the return reason codes this package is confident about.
// The SPI catalog defines when each applies; FromPacs008 requires one of
// them unless AllowUnknownReason is given, while Parse and Validate accept
// any code.
var Reasons = xsd.NewCodeSet(
	xsd.CodeInfo{Code: ReasonBankError, Name: "BankError",
		English:    "Returned as a result of a bank error.",
		Portuguese: "Devolução por erro operacional do participante."},
	xsd.CodeInfo{Code: ReasonFraud, Name: "Fraud",
		English:    "Returned as a result of fraud.",
		Portuguese: "Devolução por fundada suspeita de fraude."},
	xsd.CodeInfo{Code: ReasonCustomerRefund, Name: "RefundRequestByEndCustomer",
		English:    "Return of funds requested by the end customer.",
		Portuguese: "Devolução solicitada pelo cliente usuário recebedor."},
	xsd.CodeInfo{Code: ReasonCreditorService, Name: "SpecificServiceOfferedByCreditorAgent",
		English:    "Returned due to a specific service offered by the creditor agent.",
		Portuguese: "Devolução por serviço específico do participante recebedor."},
)
