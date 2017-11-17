package server

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/allencloud/automan/server/config"
	"github.com/allencloud/automan/server/fetcher"
	"github.com/allencloud/automan/server/gh"
	"github.com/allencloud/automan/server/processor"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// DefaultAddress is the default address daemon will listen to.
var DefaultAddress = ":6789"

// Server refers to a
type Server struct {
	listenAddress   string
	processor       *processor.Processor
	fetcher         *fetcher.Fetcher
	maintainersTeam string
}

// NewServer constructs a brand new automan server
func NewServer(config config.Config) *Server {
	ghClient := gh.NewClient(config.Owner, config.Repo, config.AccessToken)
	return &Server{
		processor:     processor.NewProcessor(ghClient),
		listenAddress: config.HTTPListen,
		fetcher:       fetcher.NewFetcher(ghClient),
	}
}

// Run runs the server.
func (s *Server) Run() error {
	// start fetcher in a goroutine
	go s.fetcher.Work()

	// start webserver
	listenAddress := s.listenAddress
	if listenAddress == "" {
		listenAddress = DefaultAddress
	}

	r := mux.NewRouter()

	// register ping api
	r.HandleFunc("/_ping", pingHandler).Methods("GET")

	// github webhook API
	r.HandleFunc("/events", s.eventHandler).Methods("POST")

	// travisCI webhook API
	r.HandleFunc("/ci_notifications", s.ciNotificationHandler).Methods("POST")
	return http.ListenAndServe(listenAddress, r)
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Debug("/_ping request received")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte{'O', 'K'})
	return
}

func (s *Server) eventHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Debug("/events request received")
	eventType := r.Header.Get("X-Github-Event")

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	r.Body.Close()

	if err := s.processor.HandleEvent(eventType, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	return
}

//
func (s *Server) ciNotificationHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Info("/ci_notifications events reveived")
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logrus.Infof("r.Form: %v", r.Form)

	str := r.PostForm.Get("payload")
	if str != "" {
		logrus.Infof("r.PostForm[payload]: %v", r.PostForm.Get("payload"))
	}

	data := []byte(str)

	type config struct {
		pull_request_number int
		pull_request_title  string
	}
	type TravisCI struct {
		id     int
		number string
		cfg    config
	}

	var st TravisCI

	if err := json.Unmarshal(data, &st); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logrus.Infof("id: %s", st.id)

	logrus.Infof("pull request title: %s", st.cfg.pull_request_title)

	logrus.Infof("pull request number: %d", st.cfg.pull_request_number)

	w.WriteHeader(http.StatusOK)
	return
}
