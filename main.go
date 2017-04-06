package main

import (
	"net/http"
	"io/ioutil"
	"time"
	"strconv"
	"bytes"
	"log"
	"os"
	"fmt"
)

func handler(w http.ResponseWriter, r *http.Request) {
	method := r.Method

	callbackUrl := r.Header.Get("Callback-Url")
	delay := r.Header.Get("Callback-Delay")
	if callbackUrl == "" || delay == "" {
		return
	}

	var payload []byte

	if r.Body != nil {
		defer r.Body.Close()

		var err error
		payload, err = ioutil.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}
	}

	duration, err :=  strconv.Atoi(delay)
	if err != nil {
		// invalid duration
		panic(err)
	}

	log.Printf("Will respond to %s in %d ms.\n", callbackUrl, duration)

	time.AfterFunc(time.Duration(duration) * time.Millisecond, func() {
		req, err := http.NewRequest(method, callbackUrl, bytes.NewReader(payload))
		if err != nil {
			panic(err)
		}

		req.Header = r.Header
		delete(r.Header, "Callback-Url")
		delete(r.Header, "Callback-Delay")

		_, err = http.DefaultClient.Do(req)
		if err != nil {
			panic(err)
		}

		log.Printf("Responded to %s in %d ms.\n", callbackUrl, duration)
	})
}

func main() {
	http.HandleFunc("/", handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	host := os.Getenv("HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	addr := fmt.Sprintf("%s:%s", host, port)

	log.Printf("Listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
