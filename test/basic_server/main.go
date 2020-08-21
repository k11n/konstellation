package main

import (
	"net/http"
	"os"
	"strings"
)

func main() {
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("hello from /"))
	})
	http.HandleFunc("/config/", func(writer http.ResponseWriter, request *http.Request) {
		prefixLen := len("/config/")
		if len(request.URL.Path) == prefixLen {
			writer.Write([]byte(os.Getenv("APP_CONFIG")))
			return
		}
		key := request.URL.Path[prefixLen:]
		writer.Write([]byte(os.Getenv(strings.ToUpper(key))))
	})
	http.HandleFunc("/config/all", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte(os.Getenv("APP_CONFIG")))
	})

	http.ListenAndServe(":8080", nil)
}
