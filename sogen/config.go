package main

import (
	"errors"
	"fmt"
	"github.com/matm/sogenactif"
	"github.com/outofpluto/goconfig/config"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// replaceEnvVars replaces all ${VARNAME} with their value
// using os.Getenv().
func replaceEnvVars(src string) (string, error) {
	r, err := regexp.Compile(`\${([A-Z_]+)}`)
	if err != nil {
		return "", err
	}
	envs := r.FindAllString(src, -1)
	for _, varname := range envs {
		evar := os.Getenv(varname[2 : len(varname)-1])
		if evar == "" {
			return "", errors.New(fmt.Sprintf("error: env var %s not defined", varname))
		}
		src = strings.Replace(src, varname, evar, -1)
	}
	return src, nil
}

func handleQuery(uri *url.URL) (*url.URL, error) {
	qs, err := url.QueryUnescape(uri.String())
	if err != nil {
		return nil, err
	}
	r, err := replaceEnvVars(qs)
	if err != nil {
		return nil, err
	}
	wuri, err := url.Parse(r)
	if err != nil {
		return nil, err
	}
	return wuri, nil
}

// Parses all structure fields values, looks for any
// variable defined as ${VARNAME} and substitute it by
// calling os.Getenv().
//
// The reflect package is not used here since we cannot
// set a private field (not exported) within a struct using
// reflection.
func handleEnvVars(c *sogenactif.Config) error {
	if c == nil {
		return errors.New("handleEnvVars: nil config")
	}
	// cancel_url
	if c.CancelUrl != nil {
		curi, err := handleQuery(c.CancelUrl)
		if err != nil {
			return err
		}
		c.CancelUrl = curi
	}

	// return_url
	if c.ReturnUrl != nil {
		curi, err := handleQuery(c.ReturnUrl)
		if err != nil {
			return err
		}
		c.ReturnUrl = curi
	}
	return nil
}

// LoadConfig parses a config file and sets config settings
// variables to be used at runtime.
func LoadConfig(path string) (*sogenactif.Config, error) {
	settings := &sogenactif.Config{}

	c, err := config.ReadDefault(path)
	if err != nil {
		return nil, err
	}

	// debug
	var b bool
	if b, err = c.Bool("sogenactif", "debug"); err != nil {
		return nil, err
	}
	settings.Debug = b

	// logo_path
	var logPath string
	if logPath, err = c.String("sogenactif", "logo_path"); err != nil {
		return nil, err
	}
	settings.LogoPath = logPath

	// merchants_rootdir
	var mRootDir string
	if mRootDir, err = c.String("sogenactif", "merchants_rootdir"); err != nil {
		return nil, err
	}
	settings.MerchantsRootDir = mRootDir

	// media_path
	var mediaPath string
	if mediaPath, err = c.String("sogenactif", "media_path"); err != nil {
		return nil, err
	}
	settings.MediaPath = mediaPath

	// merchant_id
	var merchantId string
	if merchantId, err = c.String("sogenactif", "merchant_id"); err != nil {
		return nil, err
	}
	settings.MerchantId = merchantId

	// merchant_country
	var merchantCountry string
	if merchantCountry, err = c.String("sogenactif", "merchant_country"); err != nil {
		return nil, err
	}
	settings.MerchantCountry = merchantCountry

	// merchant_currency_code
	var merchantCurrencyCode string
	if merchantCurrencyCode, err = c.String("sogenactif", "merchant_currency_code"); err != nil {
		return nil, err
	}
	settings.MerchantCurrencyCode = merchantCurrencyCode

	// cancel_url
	var cUrl *url.URL
	var uri string

	if uri, err = c.String("sogenactif", "cancel_url"); err != nil {
		return nil, err
	}
	if cUrl, err = url.Parse(uri); err != nil {
		return nil, errors.New(fmt.Sprint("cancel URL: ", err.Error()))
	}
	settings.CancelUrl = cUrl

	// return_url
	if uri, err = c.String("sogenactif", "return_url"); err != nil {
		return nil, err
	}
	if cUrl, err = url.Parse(uri); err != nil {
		return nil, errors.New(fmt.Sprint("return URL: ", err.Error()))
	}
	settings.ReturnUrl = cUrl

	// Looks for env variables, perform substitutions if needed
	if err := handleEnvVars(settings); err != nil {
		return nil, err
	}
	return settings, nil
}
