package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var (
	requestCount uint64

	callbacks = map[uint64]callback{}
)

type callback struct {
	timer  *time.Timer
	end    time.Time
	method string
	url    string
}

type callbackStatus struct {
	RequestId     int    `json:"request_id"`
	CallbackInfo  string `json:"callback_info"`
	TimeRemaining int    `json:"time_remaining"`
}

func newCallback(w http.ResponseWriter, r *http.Request) {
	id := atomic.AddUint64(&requestCount, 1)

	method := r.Method

	url := r.Header.Get("Callback-Url")
	callbackDelay := r.Header.Get("Callback-Delay")
	if url == "" || callbackDelay == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`"Callback-Url" and "Callback-Delay" headers must be provided.`))
		return
	}

	var payload []byte

	if r.Body != nil {
		var err error
		payload, err = ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println("error while reading response body")

			w.WriteHeader(500)
			return
		}
	}

	d, err := strconv.Atoi(callbackDelay)
	if err != nil {
		// invalid duration
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`Invalid callback delay.`))

		return
	}

	delay := time.Duration(d) * time.Second

	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		log.Printf("error while creating request to %s\n", url)
	}

	req.Header = r.Header
	delete(req.Header, "Callback-Url")
	delete(req.Header, "Callback-Delay")

	log.Printf("Callback %d: %s %s in %d s.\n", id, method, url, d)

	timer := time.AfterFunc(time.Duration(d)*time.Second, func() {
		_, err = http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("error while calling back %s\n", url)

			return
		}

		log.Printf("Callback %d: %s %s executed after %d s.\n", id, method, url, d)
		delete(callbacks, id)
	})

	callback := callback{
		timer:  timer,
		end:    time.Now().Add(delay),
		method: method,
		url:    url,
	}
	callbacks[id] = callback

	info := fmt.Sprintf("%s %s", method, url)
	status := callbackStatus{
		RequestId:     int(id),
		CallbackInfo:  info,
		TimeRemaining: d,
	}

	js, err := json.Marshal(status)
	if err != nil {
		log.Println("error while marshalling status")

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(js)
}

func callbacksGet(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	c := parts[2]

	if c == "" {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	id, err := strconv.Atoi(c)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)

		return
	}

	if callback, exists := callbacks[uint64(id)]; exists {
		r := callback.end.Sub(time.Now())
		remaining := int(r / time.Second)

		info := fmt.Sprintf("%s %s", callback.method, callback.url)

		status := callbackStatus{
			id,
			info,
			remaining,
		}

		js, err := json.Marshal(status)
		if err != nil {
			log.Println("error while marshalling status")

			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

func callbacksDelete(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	c := parts[2]

	if c == "" {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	id, err := strconv.Atoi(c)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)

		return
	}

	if callback, exists := callbacks[uint64(id)]; exists {
		callback.timer.Stop()
		delete(callbacks, uint64(id))

		log.Printf("Callback %d: %s %s deleted.\n", id, callback.method, callback.url)

		w.WriteHeader(http.StatusNoContent)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

func callbacksHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		callbacksGet(w, r)
	} else if r.Method == http.MethodDelete {
		callbacksDelete(w, r)
	}
}

func main() {
	http.HandleFunc("/new", newCallback)
	http.HandleFunc("/callbacks/", callbacksHandler)

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
