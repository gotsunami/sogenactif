// Copyright 2013 Mathias Monnerville. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sogenactif provides support for the online payment solution provided
// by la Société Générale.
//
// https://www.sogenactif.com/
//
// Load a config file with LoadConfig() then initialize the framework:
//   conf, err := LoadConfig("conf/demo.cfg")
//   s, err := NewSogen(conf)
//
// Given a <merchant_id>, a NewSogen() call will check that the merchant's certificate
// is available in ${merchants_rootdir}/<merchant_id> (see conf/demo.cfg) and will
// create (or overwrite) some files required by the Sogenactif plateform:
//   certif.fr.<merchant_id>.php # Your certificate file
//   parcom.<merchant_id>        # Generated, defines locations for cancel and return urls
//   parcom.sogenactif           # Generated, defines some parameters for the platform
//   pathfile                    # Generated, defines location of all files
//
// Now, using the API is a matter of serving content and calling Checkout() to initiate a
// payment, then calling HandlePayment() to get results back from the payment server.
//
// Initiate a payment with:
// 	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
// 		t := sogenactif.NewTransaction(&sogenactif.Customer{Id: "funkyab"}, 4.99)
// 		fmt.Fprintf(w, "<html><body>")
// 		sogen.Checkout(t, w) // Will add credit card logos and a link to the secure payment server
// 		fmt.Fprintf(w, "</body></html>")
// 	})
//
// Handle the return_url after a successful payment (provided the user click on the "return to site"
// button):
// 	http.HandleFunc(conf.ReturnUrl.Path, func(w http.ResponseWriter, r *http.Request) {
// 		fmt.Fprintf(w, "<html><body>")
// 		fmt.Fprintf(w, "<h2>Thank you!</h2>")
// 		p := sogen.HandlePayment(w, r)
// 		fmt.Fprintf(w, "</body></html>")
// 	})
//
// Also handle the cancel_url in case a payment is cancelled:
// 	http.HandleFunc(conf.CancelUrl.Path, func(w http.ResponseWriter, r *http.Request) {
// 		fmt.Fprintf(w, "<html><body>")
// 		fmt.Fprintf(w, "<h2>The transaction has been cancelled.</h2>")
// 		fmt.Fprintf(w, "</body></html>")
// 	})
//
// Finally, serve static content (to display credit card logos etc.) with:
//  http.Handle(conf.LogoPath, http.StripPrefix(conf.LogoPath, http.FileServer(http.Dir(conf.MediaPath))))
//  log.Fatal(http.ListenAndServe(":6060", nil))
package sogenactif

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"
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
	LibraryPath          string // Path to the provided closed-source binaries
	MerchantsRootDir     string // maps to merchant/
	MediaPath            string // Path to static files (credit cards logos etc.)
	MerchantId           string // Merchant Id
	MerchantCountry      string // Merchant country
	MerchantCurrencyCode string // Merchant currency code
	AutoResponseUrl      *url.URL
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
	// Caddie is a free field which is sent back unmodified after a successful
	// payment. It can contains up to 2048 chars.
	Caddie string
	// CancelUrl can be provided to override the cancel_url variable in the config
	// file, on a per-customer basis.
	CancelUrl *url.URL
	// ReturnUrl can be provided to override the return_url variable in the config
	// file, on a per-customer basis.
	ReturnUrl *url.URL
}

type Transaction struct {
	customer *Customer
	amount   float64
}

// Payment holds data filled (and returned) by the secure payment server.
type Payment struct {
	MerchantId                           string // Unique merchant's identifier (generally 0 followed by SIRET number)
	MerchantCountry                      string // A 2-letter country code
	Amount                               float64
	TransactionId                        string
	PaymentMeans                         string // Payment mean chosen by the customer
	TransmissionDate                     time.Time
	PaymentDate                          time.Time
	ResponseCode                         string
	PaymentCertificate                   string
	AuthorizationId                      string
	CurrencyCode                         string
	CardNumber, CVVFlag, CVVResponseCode string
	BankResponseCode                     string
	ComplementaryCode, ComplementaryInfo string
	ReturnContext                        string // Customer's buying context. Sent back unmodified.
	Caddie                               string // Free field. Sent back unmodified.
	ReceiptComplement                    string
	MerchantLanguage, Language           string
	CustomerId                           string
	CustomerEmail, CustomerIpAddress     string
	CaptureDay, CaptureMode              string
	Data                                 string
	OrderValidity                        string
	ScoreValue, ScoreColor, ScoreInfo    string
	ScoreThreshold, ScoreProfile         string
}

func (p *Payment) String() string {
	return fmt.Sprintf(`========================================
Merchant ID: %s
Merchant Country: %s
Amount: %.2f
Transaction ID: %s
Payment Means: %s
----------------------------------------
Transmission Date: %s
Payment Date: %s
Response Code: %s
Payment Certificate: %s
----------------------------------------
Authorization ID: %s
Currency Code: %s
Card Number: %s
CVV Flag: %s
CVV Response Code: %s
Bank Response Code: %s
Complementary Code: %s
Complementary Info: %s
----------------------------------------
Return Context: %s
Caddie: %s
Receipt Complement: %s
Merchant Language: %s
Language: %s
----------------------------------------
Customer ID: %s
Customer Email: %s
Customer IP Address: %s
----------------------------------------
Capture Day: %s
Capture Mode: %s
Data: %s
Order Validity: %s
----------------------------------------
Score Value: %s
Score Color: %s
Score Info: %s
Score Threshold: %s
Score Profile: %s`,
		p.MerchantId, p.MerchantCountry, p.Amount, p.TransactionId, p.PaymentMeans, p.TransmissionDate,
		p.PaymentDate, p.PaymentCertificate, p.ResponseCode, p.AuthorizationId, p.CurrencyCode, p.CardNumber,
		p.CVVFlag, p.CVVResponseCode, p.BankResponseCode, p.ComplementaryCode, p.ComplementaryInfo,
		p.ReturnContext, p.Caddie, p.ReceiptComplement, p.MerchantLanguage, p.Language, p.CustomerId,
		p.CustomerEmail, p.CustomerIpAddress, p.CaptureDay, p.CaptureMode, p.Data, p.OrderValidity,
		p.ScoreValue, p.ScoreColor, p.ScoreInfo, p.ScoreThreshold, p.ScoreProfile)
}

// requestParams defines some request parameters in the Checkout() process.
func (s *Sogen) requestParams(t *Transaction) []string {
	params := map[string]string{
		"merchant_id":      s.config.MerchantId,
		"merchant_country": s.config.MerchantCountry,
		"amount":           strconv.Itoa(int(t.amount * 100)),
		"currency_code":    s.config.MerchantCurrencyCode,
		"pathfile":         s.pathFile,
		"caddie":           t.customer.Caddie,
	}
	if t.customer.Id != "" {
		params["customer_id"] = t.customer.Id
	}
	if t.customer.CancelUrl != nil {
		params["cancel_return_url"] = t.customer.CancelUrl.String()
	}
	if t.customer.ReturnUrl != nil {
		params["normal_return_url"] = t.customer.ReturnUrl.String()
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
	if _, err := os.Stat(c.LibraryPath); err != nil {
		return nil, errors.New("bad library_path: " + err.Error())
	}

	log.Printf("Initializing the Sogenactif payment system (%s)", c.MerchantId)
	s := new(Sogen)
	s.config = c
	s.merchantBaseDir = path.Join(c.MerchantsRootDir, c.MerchantId)
	s.certificatePrefix = path.Join(s.merchantBaseDir, "certif")
	s.parametersPrefix = path.Join(s.merchantBaseDir, "parcom")
	s.parametersSogenActif = path.Join(s.merchantBaseDir, "parcom.sogenactif")
	s.pathFile = path.Join(s.merchantBaseDir, "pathfile")
	s.requestFile = path.Join(c.LibraryPath, runtime.GOOS+"_"+runtime.GOARCH, "request")
	s.responseFile = path.Join(c.LibraryPath, runtime.GOOS+"_"+runtime.GOARCH, "response")
	if _, err := os.Stat(s.requestFile); err != nil {
		return nil, errors.New("request binary: " + err.Error())
	}
	if _, err := os.Stat(s.responseFile); err != nil {
		return nil, errors.New("request binary: " + err.Error())
	}

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
	// auto_response_url config parameter is optional
	if s.config.AutoResponseUrl != nil {
		_, err = f.Write([]byte(fmt.Sprintf("AUTO_REPONSE_URL!%s!\n", s.config.AutoResponseUrl)))
	}
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
}

func formatToRFC3339(dt, offset string) (time.Time, error) {
	format := "%s-%s-%sT%s:%s:%s" + offset
	rfc, err := time.Parse(time.RFC3339, fmt.Sprintf(format, dt[:4], dt[4:6], dt[6:8], dt[8:10],
		dt[10:12], dt[12:14]))
	return rfc, err
}

// HandlePayment generates a payment from the Sogen's server
// response.
func (s *Sogen) HandlePayment(w io.Writer, r *http.Request) *Payment {
	if r == nil {
		return nil
	}
	data := r.PostFormValue("DATA")
	cmd := exec.Command(s.responseFile, "pathfile="+s.pathFile, "message="+data)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Run()
	res := strings.Split(out.String(), "!")
	code, err := res[1], res[2]

	if code == "" && err == "" {
		fmt.Fprintf(w, "error: request executable not found!")
	} else if code != "0" {
		fmt.Fprintf(w, "error using API: %s", err)
	} else {
		// err holds debug info if DEBUG is set to YES
		fmt.Fprintf(w, err)

		v := res[3:]
		amount, err := strconv.ParseFloat(v[2], 32)
		if err != nil {
			fmt.Fprintf(w, "amount conversion error: "+err.Error())
			return nil
		}
		amount /= 100

		tDate, err := formatToRFC3339(v[5], "+01:00")
		if err != nil {
			fmt.Fprintf(w, "transmission date conversion error: "+err.Error())
			return nil
		}
		pDateTime, err := formatToRFC3339(v[7]+v[6], "+01:00")
		if err != nil {
			fmt.Fprintf(w, "payment datetime conversion error: "+err.Error())
			return nil
		}

		p := Payment{
			MerchantId:         v[0],
			MerchantCountry:    v[1],
			Amount:             amount,
			TransactionId:      v[3],
			PaymentMeans:       v[4],
			TransmissionDate:   tDate,
			PaymentDate:        pDateTime,
			PaymentCertificate: v[8],
			ResponseCode:       v[9],
			AuthorizationId:    v[10],
			CurrencyCode:       v[11],
			CardNumber:         v[12],
			CVVFlag:            v[13],
			CVVResponseCode:    v[14],
			BankResponseCode:   v[15],
			ComplementaryCode:  v[16],
			ComplementaryInfo:  v[17],
			ReturnContext:      v[18],
			Caddie:             v[19],
			ReceiptComplement:  v[20],
			MerchantLanguage:   v[21],
			Language:           v[22],
			CustomerId:         v[23],
			CustomerEmail:      v[24],
			CustomerIpAddress:  v[25],
			CaptureDay:         v[26],
			CaptureMode:        v[27],
			Data:               v[28],
			OrderValidity:      v[29],
			ScoreValue:         v[30],
			ScoreColor:         v[31],
			ScoreInfo:          v[32],
			ScoreThreshold:     v[33],
			ScoreProfile:       v[34],
		}
		return &p
	}
	return nil
}
