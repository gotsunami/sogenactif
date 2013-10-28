package main

import (
	"fmt"
	"github.com/matm/sogenactif"
	"log"
	"net/http"
)

const (
	port   = "6060"
	amount = 1.00
)

func main() {
	// TODO: read this from a config file
	conf, err := sogenactif.LoadConfig("conf/demo.cfg")
	if err != nil {
		log.Fatal("config file error: " + err.Error())
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
