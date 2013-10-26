package sogenactif

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

const (
	debug                        = false
	logoPath                     = "media"
	merchantCertificatePrefix    = "config/demo/certif"
	merchantParametersPrefix     = "config/demo/parmcom"
	merchantParametersSogenActif = "config/demo/parmcom.sogenactif"
	pathFile                     = "config/demo/pathfile"
	binPath                      = "../lib"
)

type Sogen struct {
	Debug                bool
	LogoPath             string
	CertificatePrefix    string // Merchant certificate prefix
	ParametersPrefix     string // Merchant parameters file prefix
	ParametersSogenActif string // Merchant parameters file sogenactif
	PathFile             string
	requestFile          string // Path to request (priorietary) binary
	responseFile         string // Path to response (priorietary) binary
	merchant             *Merchant
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
		"pathfile":         pathFile,
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
	s.Debug = debug
	s.merchant = m
	s.requestFile = fmt.Sprintf("%s/%s_%s/request", binPath, runtime.GOOS, runtime.GOARCH)

	// Write files
	return s, nil
}

// Checkout generates an HTML form suitable to redirect the buyer
// to the payment server.
func (s *Sogen) Checkout(t *Transaction, w http.ResponseWriter) {
	// Execute binary
	cmd := exec.Command(s.requestFile, s.requestParams(t)...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Run()
	res := strings.Split(out.String(), "!")
	code, err, body := res[1], res[2], res[3]
	if code == "" && err == "" {
		fmt.Fprintf(w, "error: request executable not found!")
	} else if code != "0" {
		fmt.Fprintf(w, "error using API: %s", err)
	} else {
		// No error
		fmt.Fprintf(w, "<html><body>")
		fmt.Fprintf(w, body)
		fmt.Fprintf(w, "</body></html>")
	}
}
