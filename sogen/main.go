package main

import (
	"fmt"
	"github.com/matm/sogenactif"
	"log"
	"net/http"
)

// TODO: read those const and variables from a config file
const (
	port = "6060"
)

const (
	merchantId      = "014213245611111"
	merchantCountry = "fr"
	amount          = 1.00
	currencyCode    = "978"
)

func main() {
	m := &sogenactif.Merchant{
		Id:           merchantId,
		Country:      merchantCountry,
		CurrencyCode: currencyCode,
	}
	sogen, err := sogenactif.NewSogen(m)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Starting server on port %s ...\n", port)
	http.Handle("/media/", http.FileServer(http.Dir("media/")))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t := sogenactif.NewTransaction(&sogenactif.Buyer{}, amount)
		sogen.Checkout(t, w)
	})
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
