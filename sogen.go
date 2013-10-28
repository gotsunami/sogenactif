package sogenactif

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"
)

const (
	debug   = false
	binPath = "../lib"
)

type Sogen struct {
	requestFile          string  // Path to request (proprietary) binary
	responseFile         string  // Path to response (proprietary) binary
	merchantBaseDir      string  // maps to merchant/marchant_id
	config               *Config // Config file
	certificatePrefix    string  // Merchant certificate prefix
	parametersPrefix     string  // Merchant parameters file prefix
	parametersSogenActif string  // Merchant parameters file sogenactif
	pathFile             string
}

type Config struct {
	Debug                bool
	LogoPath             string
	MerchantsRootDir     string // maps to merchant/
	MediaPath            string // Path to static files (credit cards logos etc.)
	MerchantId           string // Merchant Id
	MerchantCountry      string // Merchant country
	MerchantCurrencyCode string // Merchant currency code
	CancelUrl            *url.URL
	ReturnUrl            *url.URL
}

type Buyer struct {
}

type Transaction struct {
	buyer  *Buyer
	amount float64
}

func (s *Sogen) requestParams(t *Transaction) []string {
	params := map[string]string{
		"merchant_id":      s.config.MerchantId,
		"merchant_country": s.config.MerchantCountry,
		"amount":           strconv.Itoa(int(t.amount * 100)),
		"currency_code":    s.config.MerchantCurrencyCode,
		"pathfile":         s.pathFile,
	}
	plist := make([]string, 0)
	for k, v := range params {
		plist = append(plist, fmt.Sprintf("%s=%s", k, v))
	}
	return plist
}

func NewTransaction(c *Buyer, amount float64) *Transaction {
	if c == nil {
		return nil
	}
	return &Transaction{buyer: c, amount: amount}
}

// NewSogen sets up all the files required by the Sogen API for
// a giver merchant.
func NewSogen(c *Config) (*Sogen, error) {
	if c == nil {
		return nil, errors.New("can't initialize Sogen framework: nil config.")
	}
	c.MerchantId = strings.Trim(c.MerchantId, " ")
	if c.MerchantId == "" {
		return nil, errors.New("missing merchant ID")
	}
	c.MerchantsRootDir = strings.Trim(c.MerchantsRootDir, " ")
	if c.MerchantsRootDir == "" {
		return nil, errors.New("missing merchant root directory (for config files and certificates)")
	}

	s := new(Sogen)
	s.config = c
	s.merchantBaseDir = path.Join(c.MerchantsRootDir, c.MerchantId)
	s.certificatePrefix = path.Join(s.merchantBaseDir, "certif")
	s.parametersPrefix = path.Join(s.merchantBaseDir, "parcom")
	s.parametersSogenActif = path.Join(s.merchantBaseDir, "parcom.sogenactif")
	s.pathFile = path.Join(s.merchantBaseDir, "pathfile")
	s.requestFile = fmt.Sprintf("%s/%s_%s/request", binPath, runtime.GOOS, runtime.GOARCH)

	if _, err := os.Stat(s.merchantBaseDir); err != nil {
		return nil, errors.New(fmt.Sprintf("missing certificate file in directory %s", s.merchantBaseDir))
	}
	certFile := fmt.Sprintf("%s.fr.%s.php", s.certificatePrefix, c.MerchantId)
	if _, err := os.Stat(certFile); err != nil {
		return nil, errors.New(fmt.Sprintf("missing certificate file %s", certFile))
	}
	log.Printf("Found certificate file %s", certFile)

	// Write pathfile
	f, err := os.Create(s.pathFile)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	_, err = f.Write([]byte(fmt.Sprintf(`DEBUG!NO!
D_LOGO!%s!
F_CERTIFICATE!%s!
F_CTYPE!php!
F_PARAM!%s!
F_DEFAULT!%s!
`, s.config.LogoPath, s.certificatePrefix, s.parametersPrefix, s.parametersSogenActif)))
	if err != nil {
		return nil, err
	}
	log.Printf("Created file %s", s.pathFile)

	// Write parmcom.merchant_id
	parmcom := fmt.Sprintf("%s.%s", s.parametersPrefix, c.MerchantId)
	f, err = os.Create(parmcom)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	_, err = f.Write([]byte(fmt.Sprintf(`LOGO!/bf/chrome/common/logo.png!
CANCEL_URL!%s!
RETURN_URL!%s!
`, s.config.CancelUrl, s.config.ReturnUrl)))
	if err != nil {
		return nil, err
	}
	log.Printf("Created file %s", parmcom)

	// Write parmcom.sogenactif
	f, err = os.Create(s.parametersSogenActif)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	_, err = f.Write([]byte(fmt.Sprintf(`ADVERT!sg.gif!
BGCOLOR!ffffff!
BLOCK_ALIGN!center!
BLOCK_ORDER!1,2,3,4,5,6,7,8!
CONDITION!SSL!
CURRENCY!978!
HEADER_FLAG!yes!
LANGUAGE!fr!
LOGO2!sogenactif.gif!
MERCHANT_COUNTRY!fr!
MERCHANT_LANGUAGE!fr!
PAYMENT_MEANS!CB,2,VISA,2,MASTERCARD,2,PAYLIB,2!
TARGET!_top!
TEXTCOLOR!000000!
`)))
	if err != nil {
		return nil, err
	}
	log.Printf("Created file %s", s.parametersSogenActif)

	return s, nil
}

// Checkout generates an HTML form suitable to redirect the buyer
// to the payment server.
func (s *Sogen) Checkout(t *Transaction, w io.Writer) {
	// Execute binary
	cmd := exec.Command(s.requestFile, s.requestParams(t)...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Run()
	res := strings.Split(out.String(), "!")
	code, err, body := res[1], res[2], res[3]
	fmt.Fprintf(w, "<html><body>")
	if code == "" && err == "" {
		fmt.Fprintf(w, "error: request executable not found!")
	} else if code != "0" {
		fmt.Fprintf(w, "error using API: %s", err)
	} else {
		// No error
		fmt.Fprintf(w, body)
	}
	fmt.Fprintf(w, "</body></html>")
}
