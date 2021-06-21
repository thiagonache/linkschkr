package links

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Addr    string `yaml:"addr"`
		Timeout struct {
			Read  int `yaml:"read"`
			Write int `yaml:"write"`
			Idle  int `yaml:"idle"`
		} `yaml:"timeout"`
	} `yaml:"server"`
}

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
	err = json.NewEncoder(w).Encode(results)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error encoding results: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func WebServerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	webServerCheck(w, r)
}

func ListenAndServe() error {
	f, err := os.Open("config/config.yaml")
	if err != nil {
		return err
	}
	defer f.Close()
	var cfg Config
	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		return fmt.Errorf("cannot decode config file: %v", err)
	}

	router := http.NewServeMux()
	handlerCheck := http.HandlerFunc(WebServerHandler)
	router.Handle("/check", handlerCheck)
	srv := http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.Timeout.Read) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.Timeout.Write) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.Timeout.Idle) * time.Second,
	}
	err = srv.ListenAndServe()
	return err
}
