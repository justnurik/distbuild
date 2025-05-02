package main

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

func startServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "lol")
	})

	http.ListenAndServe(":8083", mux)
}

func main() {
	go startServer()

	time.After(time.Second)

	count := 50_000

	client := http.Client{
		Transport: &http.Transport{
			MaxIdleConns:    1000,
			IdleConnTimeout: 90 * time.Second,
		},
	}
	var wg sync.WaitGroup
	wg.Add(count)
	for range count {
		time.After(time.Millisecond)
		go func() {
			defer wg.Done()
			req, err := http.NewRequest("GET", "http://localhost:8083/api", nil)
			if err != nil {
				panic(fmt.Sprintf("wtf: %s", err.Error()))
			}

			resp, err := client.Do(req)
			if err != nil {
				panic(fmt.Sprintf("wtf: %s", err.Error()))
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				panic(fmt.Sprintf("wtf: %s", err.Error()))
			}

			fmt.Println(string(body))
		}()
	}

	wg.Wait()
}
