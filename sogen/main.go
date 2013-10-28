package main

import (
	"flag"
	"fmt"
	"github.com/matm/sogenactif"
	"log"
	"net/http"
	"os"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, fmt.Sprintf("Usage: %s [options] settings.conf \n", os.Args[0]))
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	port := flag.String("p", "6060", "http server listening port")
	amount := flag.Float64("t", 1.00, "transaction amount")
	flag.Parse()
	if len(flag.Args()) != 1 {
		flag.Usage()
	}

	conf, err := sogenactif.LoadConfig(flag.Arg(0))
	if err != nil {
		log.Fatal("config file error: " + err.Error())
	}
	sogen, err := sogenactif.NewSogen(conf)
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/media/", http.StripPrefix("/media/", http.FileServer(http.Dir(conf.MediaPath))))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t := sogenactif.NewTransaction(&sogenactif.Customer{Id: "Mat"}, *amount)
		sogen.Checkout(t, w)
	})
	fmt.Printf("Starting server on port %s ...\n", *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
