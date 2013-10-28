package main

import (
	"fmt"
	"github.com/matm/sogenactif"
	"log"
	"net/http"
	"net/url"
)

const (
	port   = "6060"
	amount = 1.00
)

func main() {
	// TODO: read this from a config file
	cUrl, _ := url.Parse("http://localhost:6060/sogen/cancel")
	rUrl, _ := url.Parse("http://localhost:6060/sogen/return")
	conf := &sogenactif.Config{
		Debug:                false,
		MerchantId:           "014213245611111",
		MerchantsRootDir:     "./merchant",
		MerchantCountry:      "fr",
		MerchantCurrencyCode: "978",
		MediaPath:            "./media",
		LogoPath:             "/media/",
		CancelUrl:            cUrl,
		ReturnUrl:            rUrl,
	}
	sogen, err := sogenactif.NewSogen(conf)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Starting server on port %s ...\n", port)
	http.Handle("/media/", http.StripPrefix("/media/", http.FileServer(http.Dir(conf.MediaPath))))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t := sogenactif.NewTransaction(&sogenactif.Buyer{}, amount)
		sogen.Checkout(t, w)
	})
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
