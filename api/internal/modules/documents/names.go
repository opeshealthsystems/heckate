package documents

// typeNames are the official names of the document codes, from the approved brand pack's register.
var typeNames = map[string]string{
	"ASM":    "Assessment Certificate",
	"CN":     "Credit Note",
	"CRT":    "Certificate",
	"CRT-A":  "Certificate of Assessment",
	"CRT-C":  "Certificate",
	"CTR":    "Contract",
	"FRM-06": "Transmittal and Fax Cover Sheet",
	"FRM-07": "Client Information and Onboarding Form",
	"FRM-08": "Service Request and Requirements Questionnaire",
	"FRM-09": "Penetration Test Scoping Form",
	"FRM-10": "Security Assessment Pre-Engagement Questionnaire",
	"FRM-11": "hecID e-KYC Integration Request",
	"FRM-12": "Background Screening Request",
	"FRM-13": "Client KYC and Due-Diligence Form",
	"FRM-14": "Incident Report and Emergency Response Request",
	"FRM-15": "Customer Feedback and Satisfaction Form",
	"FRM-23": "Change Request Form",
	"FRM-24": "Rate Card",
	"FRM-26": "Project Kick-off Record",
	"FRM-27": "Delivery Note and Acceptance Certificate",
	"FRM-30": "Timesheet and Service Log",
	"FRM-31": "Project Closure and Sign-off",
	"FRM-32": "Job Application Form",
	"FRM-33": "Offer of Employment",
	"FRM-35": "Leave Request",
	"FRM-36": "Expense Claim",
	"FRM-37": "Visitor and Laboratory Access Log",
	"FRM-38": "Asset Issue and Return Form",
	"FRM-39": "Supplier Registration Form",
	"FRM-41": "Supplier Evaluation Form",
	"GP":     "Guarantee Policies",
	"INV":    "Invoice",
	"LT":     "Letter",
	"OC":     "Order Confirmation",
	"PAY":    "Payslip",
	"PO":     "Purchase Order",
	"PR":     "Commercial and Technical Proposal",
	"QT":     "Quotation",
	"RCT":    "Receipt",
	"RFQ":    "Request for Quotation",
	"RPT":    "Penetration Test Report",
	"SOW":    "Statement of Work",
}

// TypeName is the official name of a document code (HS-FRM-23 gives "Change Request Form"), or the code itself.
func TypeName(code string) string {
	if n, ok := typeNames[code]; ok {
		return n
	}
	return code
}
