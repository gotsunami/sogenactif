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

// Sogen holds information for the Sogenactif platform.
type Sogen struct {
	config               *Config // Config file
	requestFile          string  // Path to request (proprietary) binary
	responseFile         string  // Path to response (proprietary) binary
	merchantBaseDir      string  // maps to merchant/marchant_id
	certificatePrefix    string  // Merchant certificate prefix
	parametersPrefix     string  // Merchant parameters file prefix
	parametersSogenActif string  // Merchant parameters file sogenactif
	pathFile             string  // pathfile name
}

// Config holds attributes required by the platform.
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

// Customer defines some attributes that can be transmitted to the
// payment server.
type Customer struct {
	// Unique customer ID. If defined, it will be passed to
	// the sogen payment server and transmitted back after a
	// successful or cancelled payment.
	Id string
}

type Transaction struct {
	customer *Customer
	amount   float64
}

func (s *Sogen) requestParams(t *Transaction) []string {
	params := map[string]string{
		"merchant_id":      s.config.MerchantId,
		"merchant_country": s.config.MerchantCountry,
		"amount":           strconv.Itoa(int(t.amount * 100)),
		"currency_code":    s.config.MerchantCurrencyCode,
		"pathfile":         s.pathFile,
	}
	if t.customer.Id != "" {
		params["customer_id"] = t.customer.Id
	}
	plist := make([]string, 0)
	for k, v := range params {
		plist = append(plist, fmt.Sprintf("%s=%s", k, v))
	}
	return plist
}

// NewTransaction creates a new transaction for a customer that can be
// used to checkout. A nil customer or a null amount returns a nil
// transaction.
func NewTransaction(c *Customer, amount float64) *Transaction {
	if c == nil || amount == 0 {
		return nil
	}
	return &Transaction{customer: c, amount: amount}
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

	log.Printf("Initializing the Sogenactif payment system (%s)", c.MerchantId)
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
	certFile := fmt.Sprintf("%s.%s.%s.php", s.certificatePrefix, c.MerchantCountry, c.MerchantId)
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
	debug := "NO"
	if s.config.Debug {
		debug = "YES"
	}
	_, err = f.Write([]byte(fmt.Sprintf(`DEBUG!%s!
D_LOGO!%s!
F_CERTIFICATE!%s!
F_CTYPE!php!
F_PARAM!%s!
F_DEFAULT!%s!
`, debug, s.config.LogoPath, s.certificatePrefix, s.parametersPrefix, s.parametersSogenActif)))
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
LANGUAGE!%s!
LOGO2!sogenactif.gif!
MERCHANT_COUNTRY!%s!
MERCHANT_LANGUAGE!%s!
PAYMENT_MEANS!CB,2,VISA,2,MASTERCARD,2,PAYLIB,2!
TARGET!_top!
TEXTCOLOR!000000!
`, s.config.MerchantCountry, s.config.MerchantCountry, s.config.MerchantCountry)))
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
		// err holds debug info if DEBUG is set to YES
		fmt.Fprintf(w, err)
		fmt.Fprintf(w, body)
	}
	fmt.Fprintf(w, "</body></html>")
}
