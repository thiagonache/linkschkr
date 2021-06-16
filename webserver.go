package links

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
)

func webServerCheck(w http.ResponseWriter, r *http.Request) {
	queryString := r.URL.Query()
	qsSite := queryString["site"]
	if len(qsSite) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, `{"err":"cannot find site in query string"`)
		return
	}
	var noRecursion bool
	var err error
	qsNoRecursion := queryString["no-recursion"]
	if len(qsNoRecursion) > 0 {
		noRecursion, err = strconv.ParseBool(qsNoRecursion[0])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, `{"err":"cannot convert no-recursion to boolean"`)
			return
		}
	}
	output := io.Discard
	qsOutput := queryString["output"]
	if len(qsOutput) > 0 {
		output = os.Stdout
	}
	debug := io.Discard
	qsDebug := queryString["debug"]
	if len(qsDebug) > 0 {
		debug = os.Stderr
	}
	results, err := Check(qsSite, WithStdout(output), WithDebug(debug), WithNoRecursion(noRecursion))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

func WebServerHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		webServerCheck(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func ServeHTTP() error {
	router := http.NewServeMux()
	handlerCheck := http.HandlerFunc(WebServerHandler)
	router.Handle("/check", handlerCheck)
	return http.ListenAndServe(":8080", router)
}
