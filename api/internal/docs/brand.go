package docs

import (
	_ "embed"
	"regexp"
	"strings"
)

// logoPNG is the approved colour logo, downscaled for print. Never redraw or recolour it.
//
//go:embed assets/logo.png
var logoPNG []byte

// Brand colours (HEKATE brand guidelines) as RGB.
var (
	navy     = [3]int{6, 34, 69}
	gold     = [3]int{184, 145, 63}
	bandFill = [3]int{244, 247, 251}
	ruleGrey = [3]int{220, 228, 238}
)

// Classification colours: PUBLIC, INTERNAL, CONFIDENTIAL, RESTRICTED.
var classColour = map[string][3]int{
	"PUBLIC":       {46, 125, 50},
	"INTERNAL":     {21, 101, 192},
	"CONFIDENTIAL": {169, 101, 0}, // the pack gives #B26A00, which is only 4.2:1 against white text; this is the darkest-needed 4.6:1 variant
	"RESTRICTED":   {198, 40, 40},
}

// classByCode follows the document register of the brand pack (forms_registry.json). Codes the pack does
// not list are classified here and must be confirmed by the company: payslips are Restricted because
// they carry pay data; certificates and guarantee policies are Public and verifiable.
var classByCode = map[string]string{
	"QT": "CONFIDENTIAL", "INV": "CONFIDENTIAL", "RCT": "CONFIDENTIAL", "CN": "CONFIDENTIAL", "OC": "CONFIDENTIAL",
	"SOW": "CONFIDENTIAL", "PR": "CONFIDENTIAL", "PO": "CONFIDENTIAL", "RFQ": "CONFIDENTIAL", "LT": "CONFIDENTIAL", "CTR": "CONFIDENTIAL",
	"RPT": "RESTRICTED", "PAY": "RESTRICTED", "GP": "PUBLIC", "CRT-A": "PUBLIC", "CRT": "PUBLIC", "ASM": "PUBLIC",
	"FRM-06": "CONFIDENTIAL", "FRM-07": "CONFIDENTIAL", "FRM-08": "CONFIDENTIAL", "FRM-09": "RESTRICTED", "FRM-10": "CONFIDENTIAL",
	"FRM-11": "CONFIDENTIAL", "FRM-12": "RESTRICTED", "FRM-13": "RESTRICTED", "FRM-14": "RESTRICTED", "FRM-15": "INTERNAL",
	"FRM-23": "CONFIDENTIAL", "FRM-24": "CONFIDENTIAL", "FRM-26": "CONFIDENTIAL", "FRM-27": "CONFIDENTIAL", "FRM-30": "INTERNAL",
	"FRM-31": "CONFIDENTIAL", "FRM-32": "CONFIDENTIAL", "FRM-33": "CONFIDENTIAL", "FRM-35": "INTERNAL", "FRM-36": "INTERNAL",
	"FRM-37": "INTERNAL", "FRM-38": "INTERNAL", "FRM-39": "CONFIDENTIAL", "FRM-41": "INTERNAL",
}

var refCode = regexp.MustCompile(`^HS-([A-Z]+(?:-[A-Z0-9]+)*)-\d{4}-\d{6}(?:-[0-9A-Z])?$`)

// Classification returns the banner text for a document reference. Unknown codes default to CONFIDENTIAL.
func Classification(ref string) string {
	if m := refCode.FindStringSubmatch(ref); m != nil {
		if c, ok := classByCode[m[1]]; ok {
			return c
		}
	}
	return "CONFIDENTIAL"
}

// microtext is repeated at 1.5 pt along the footer rule: unreadable to the eye and lost in a photocopy.
func microtext(width float64, strWidth func(string) float64) string {
	const unit = "HEKATESYSTEMS·SECURED·"
	var b strings.Builder
	for strWidth(b.String()+unit) < width {
		b.WriteString(unit)
	}
	return b.String()
}
