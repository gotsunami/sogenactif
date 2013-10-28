package sogenactif

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

const (
	debug   = false
	binPath = "../lib"
)

type Sogen struct {
	requestFile     string  // Path to request (proprietary) binary
	responseFile    string  // Path to response (proprietary) binary
	merchantBaseDir string  // maps to merchant/marchant_id
	config          *Config // Config file
	merchant        *Merchant
}

type Config struct {
	Debug                bool
	LogoPath             string
	CertificatePrefix    string // Merchant certificate prefix
	ParametersPrefix     string // Merchant parameters file prefix
	ParametersSogenActif string // Merchant parameters file sogenactif
	PathFile             string
	CancelUrl            string
	ReturnUrl            string
}

type Merchant struct {
	Id           string
	Country      string
	CurrencyCode string
}

type Buyer struct {
}

type Transaction struct {
	buyer  *Buyer
	amount float32
}

func (s *Sogen) requestParams(t *Transaction) []string {
	params := map[string]string{
		"merchant_id":      s.merchant.Id,
		"merchant_country": s.merchant.Country,
		"amount":           strconv.Itoa(int(t.amount * 100)),
		"currency_code":    s.merchant.CurrencyCode,
		"pathfile":         s.config.PathFile,
	}
	plist := make([]string, 0)
	for k, v := range params {
		plist = append(plist, fmt.Sprintf("%s=%s", k, v))
	}
	return plist
}

func NewTransaction(c *Buyer, amount float32) *Transaction {
	if c == nil {
		return nil
	}
	return &Transaction{buyer: c, amount: amount}
}

// NewSogen sets up all the files required by the Sogen API for
// a giver merchant.
func NewSogen(m *Merchant) (*Sogen, error) {
	if m == nil {
		return nil, errors.New("can't initialize Sogen with a nil merchant.")
	}
	s := new(Sogen)
	s.merchant = m
	s.merchantBaseDir = "merchant/" + m.Id
	s.config = &Config{
		Debug:                debug,
		LogoPath:             "/media/",
		CertificatePrefix:    s.merchantBaseDir + "/certif",
		ParametersPrefix:     s.merchantBaseDir + "/parcom",
		ParametersSogenActif: s.merchantBaseDir + "/parcom.sogenactif",
		PathFile:             s.merchantBaseDir + "/pathfile",
		CancelUrl:            "http://localhost:6060/sogen/cancel",
		ReturnUrl:            "http://localhost:6060/sogen/return",
	}

	s.requestFile = fmt.Sprintf("%s/%s_%s/request", binPath, runtime.GOOS, runtime.GOARCH)

	// TODO: check for existing merchantBaseDir with certificates
	if _, err := os.Stat(s.merchantBaseDir); err != nil {
		return nil, errors.New(fmt.Sprintf("missing certificate file in directory %s", s.merchantBaseDir))
	}
	certFile := fmt.Sprintf("%s.fr.%s.php", s.config.CertificatePrefix, m.Id)
	if _, err := os.Stat(certFile); err != nil {
		return nil, errors.New(fmt.Sprintf("missing certificate file %s", certFile))
	}
	log.Printf("Found certificate file %s", certFile)

	// Write pathfile
	f, err := os.Create(s.config.PathFile)
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
`, s.config.LogoPath, s.config.CertificatePrefix, s.config.ParametersPrefix, s.config.ParametersSogenActif)))
	if err != nil {
		return nil, err
	}
	log.Printf("Created file %s", s.config.PathFile)

	// Write parmcom.merchant_id
	parmcom := fmt.Sprintf("%s.%s", s.config.ParametersPrefix, m.Id)
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
	f, err = os.Create(s.config.ParametersSogenActif)
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
	log.Printf("Created file %s", s.config.ParametersSogenActif)

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
