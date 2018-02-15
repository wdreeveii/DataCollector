package main

import (
	"fmt"
	"log"
	"net/http"
	//"html/template"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/wdreeveii/DataCollector/DataSourceManager"
	"github.com/wdreeveii/DataCollector/SessionManager"
	"io/ioutil"
)

func main() {
	r := mux.NewRouter()
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data, err := ioutil.ReadFile("templates/main.html")
		if err != nil {
			panic("Unable to read templates/main.html")
		}
		fmt.Fprintf(w, "%s", data)
	})

	r.HandleFunc("/api/dataSourceList", dataSourceList).Methods("GET")
	r.HandleFunc("/api/dataSource", dataSourcePlot).Methods("GET")
	r.HandleFunc("/api/dataSourceStream", dataSourcePlotRT).Methods("GET")
	r.HandleFunc("/api/sessionDetails", sessionDetails).Methods("GET")
	r.HandleFunc("/api/sessionCapture", sessionCapture).Methods("POST")
	r.HandleFunc("/api/sessionList", sessionList).Methods("GET")
	r.HandleFunc("/api/recentSessions", recentSessions).Methods("GET")
	r.HandleFunc("/api/newSession", newSession).Methods("POST")
	r.HandleFunc("/api/removeSession", removeSession).Methods("POST")

	fmt.Println("Starting up..")
	log.Fatal(http.ListenAndServe(":8080", r))
}

type DataSourceResponse struct {
	DataSources []DataSourceManager.DataSource
}

func dataSourceList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	dataSources, err := DataSourceManager.DataSourceList()
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	err = enc.Encode(DataSourceResponse{DataSources: dataSources})
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func dataSourcePlot(w http.ResponseWriter, r *http.Request) {
	var s SessionManager.Session
	s.Name = r.URL.Query().Get("Name")
	s.DT = r.URL.Query().Get("DT")
	s.CapturedDataSources = []DataSourceManager.DSName{DataSourceManager.DSName(r.URL.Query().Get("metric"))}

	w.Header().Set("Content-Type", "image/svg+xml")
	err := SessionManager.Plot(w, &s)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func dataSourcePlotRT(w http.ResponseWriter, r *http.Request) {
	var s SessionManager.Session
	s.Name = r.URL.Query().Get("Name")
	s.DT = r.URL.Query().Get("DT")
	s.CapturedDataSources = []DataSourceManager.DSName{DataSourceManager.DSName(r.URL.Query().Get("metric"))}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Listen to connection close and un-register messageChan
	notify := w.(http.CloseNotifier).CloseNotify()

	err := SessionManager.PlotStream(w, notify, &s)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func sessionDetails(w http.ResponseWriter, r *http.Request) {
	var s SessionManager.Session
	s.Name = r.URL.Query().Get("Name")
	s.DT = r.URL.Query().Get("DT")

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	detail, err := SessionManager.SessionDetails(&s)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = enc.Encode(detail)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

type sessionControl struct {
	SessionManager.Session
	CaptureEnabled bool `json:"captureEnabled"`
}

func sessionCapture(w http.ResponseWriter, r *http.Request) {
	var s sessionControl

	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&s)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var t SessionManager.Session
	t.Name = s.Name
	t.DT = s.DT

	err = SessionManager.SessionControl(&t, s.CaptureEnabled)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

type SessionResponse struct {
	Sessions []SessionManager.Session
}

func sessionList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	sessions, err := SessionManager.SessionList(0, 100)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = enc.Encode(SessionResponse{Sessions: sessions})
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func recentSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	sessions, err := SessionManager.RecentSessions(10)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = enc.Encode(SessionResponse{Sessions: sessions})
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func newSession(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)

	var t SessionManager.Session
	err := decoder.Decode(&t)

	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = SessionManager.NewSession(&t)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err = enc.Encode(t)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func removeSession(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var t SessionManager.Session
	err := decoder.Decode(&t)

	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = SessionManager.RemoveSession(&t)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}
