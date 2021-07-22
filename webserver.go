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

type WebServer struct {
	Cache   *cache
	Server  http.Server
	CheckFn func([]string, ...Option) ([]Result, error)
}

func (ws *WebServer) webServerCheck(w http.ResponseWriter, r *http.Request) {
	queryString := r.URL.Query()
	qsSite := queryString["site"]
	if len(qsSite) == 0 {
		http.Error(w, `{"err":"cannot find site in query string"}`, http.StatusBadRequest)
		return
	}
	var noRecursion bool
	var err error
	qsNoRecursion := queryString["no-recursion"]
	if len(qsNoRecursion) > 0 {
		noRecursion, err = strconv.ParseBool(qsNoRecursion[0])
		if err != nil {
			http.Error(w, `{"err":"cannot convert no-recursion to boolean"}`, http.StatusBadRequest)
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
	value, ok := ws.Cache.Get(qsSite[0])
	if ok {
		fmt.Fprint(w, value)
		return
	}
	fmt.Fprint(w, "return in a bit")
	ws.Cache.Store(qsSite[0], "return in a bit")
	go func() {
		results, err := ws.CheckFn(qsSite, WithStdout(output), WithDebug(debug), WithNoRecursion(noRecursion))
		if err != nil {
			ws.Cache.Store(qsSite[0], err.Error())
			return
		}
		output, err := json.Marshal(results)
		if err != nil {
			ws.Cache.Store(qsSite[0], err.Error())
			return
		}
		ws.Cache.Store(qsSite[0], string(output))
	}()

}

func (ws WebServer) WebServerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	ws.webServerCheck(w, r)
}

func NewWebServer() *WebServer {
	return &WebServer{
		Cache:   NewCache(24 * time.Hour),
		CheckFn: Check,
	}
}

func (ws *WebServer) ListenAndServe() error {
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
	handlerCheck := http.HandlerFunc(ws.WebServerHandler)
	router.Handle("/check", handlerCheck)
	ws.Server = http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.Timeout.Read) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.Timeout.Write) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.Timeout.Idle) * time.Second,
	}
	err = ws.Server.ListenAndServe()
	return err
}
