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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t := sogenactif.NewTransaction(&sogenactif.Customer{Id: "johndoe",
			Caddie: "internal-transaction-666"}, *amount)
		fmt.Fprintf(w, `<html><body>
    <a href="https://github.com/matm/sogenactif"><img style="position: absolute; top: 0; right: 0; border: 0;" src="https://s3.amazonaws.com/github/ribbons/forkme_right_red_aa0000.png" alt="Fork me on GitHub"></a>
    <div style="text-align: center;"><h2>Sogenactif secure payment demo</h2></div>
        `)
		sogen.Checkout(t, w)
		fmt.Fprintf(w, "</body></html>")
	})
	http.HandleFunc(conf.ReturnUrl.Path, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "<html><body>")
		fmt.Fprintf(w, "<h2>Thank you!</h2>")
		p := sogen.HandlePayment(w, r)
		fmt.Fprintf(w, "<p>Try a <a href=\"/\">new transaction</a>.</p>")
		fmt.Fprintf(w, "</body></html>")
		fmt.Printf("%v\n", p)
	})
	http.HandleFunc(conf.CancelUrl.Path, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "<html><body>")
		fmt.Fprintf(w, "<h2>The transaction has been cancelled.</h2>")
		fmt.Fprintf(w, "<p>You can <a href=\"/\">try a new one</a>.</p>")
		fmt.Fprintf(w, "</body></html>")
	})
	if conf.AutoResponseUrl != nil {
		http.HandleFunc(conf.AutoResponseUrl.Path, func(w http.ResponseWriter, r *http.Request) {
			log.Println("Got autoresponse!")
			p := sogen.HandlePayment(w, r)
			// Do post-processing stuff here...
			fmt.Printf("%v\n", p)
		})
	}
	// Serve static content
	http.Handle(conf.LogoPath, http.StripPrefix(conf.LogoPath, http.FileServer(http.Dir(conf.MediaPath))))

	fmt.Printf("Starting server on port %s ...\n", *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
