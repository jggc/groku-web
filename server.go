package main

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

type apiServer struct {
	store *deviceStore
	web   fs.FS
}

func (s *apiServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/devices", s.handleDevices)
	mux.HandleFunc("/api/device", s.handleDevice)
	mux.HandleFunc("/api/apps", s.handleApps)
	mux.HandleFunc("/api/key", s.handleKey)
	mux.HandleFunc("/api/launch", s.handleLaunch)
	mux.HandleFunc("/api/text", s.handleText)

	static, err := fs.Sub(s.web, "web")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(static))
	mux.Handle("/", fileServer)
	return mux
}

func (s *apiServer) handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	locs, err := discoverRokus(3 * time.Second)
	if err != nil {
		jsonErr(w, http.StatusBadGateway, err.Error())
		return
	}
	type item struct {
		Location string `json:"location"`
		DeviceInfo
	}
	out := make([]item, 0, len(locs))
	for _, loc := range locs {
		it := item{Location: normalizeLocation(loc)}
		if info, err := queryDeviceInfo(loc); err == nil {
			it.DeviceInfo = *info
		} else {
			it.Name = loc
		}
		out = append(out, it)
	}
	jsonOK(w, map[string]any{"devices": out})
}

func (s *apiServer) handleDevice(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		loc, err := s.store.ensure()
		if err != nil {
			jsonErr(w, http.StatusNotFound, err.Error())
			return
		}
		dev, err := enrichDevice(loc)
		if err != nil {
			jsonOK(w, map[string]any{
				"location": loc,
				"name":     loc,
				"error":    err.Error(),
			})
			return
		}
		jsonOK(w, dev)
	case http.MethodPut:
		var body struct {
			Location string `json:"location"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		loc := normalizeLocation(body.Location)
		if !validLocation(loc) {
			jsonErr(w, http.StatusBadRequest, "invalid location")
			return
		}
		dev, err := enrichDevice(loc)
		if err != nil {
			jsonErr(w, http.StatusBadGateway, err.Error())
			return
		}
		s.store.set(loc)
		jsonOK(w, dev)
	default:
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *apiServer) handleApps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	loc, err := s.store.ensure()
	if err != nil {
		jsonErr(w, http.StatusNotFound, err.Error())
		return
	}
	apps, err := queryApps(loc)
	if err != nil {
		jsonErr(w, http.StatusBadGateway, err.Error())
		return
	}
	jsonOK(w, map[string]any{"apps": apps})
}

func (s *apiServer) handleKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Key == "" {
		jsonErr(w, http.StatusBadRequest, "missing key")
		return
	}
	key := normalizeKey(body.Key)
	if !validKey(key) {
		jsonErr(w, http.StatusBadRequest, "key not allowed")
		return
	}
	loc, err := s.store.ensure()
	if err != nil {
		jsonErr(w, http.StatusNotFound, err.Error())
		return
	}
	if err := keypress(loc, key); err != nil {
		status := http.StatusBadGateway
		if strings.Contains(err.Error(), "403") {
			status = http.StatusForbidden
		}
		jsonErr(w, status, err.Error())
		return
	}
	jsonOK(w, map[string]string{"ok": "1", "key": key})
}

func (s *apiServer) handleLaunch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		jsonErr(w, http.StatusBadRequest, "missing app id")
		return
	}
	loc, err := s.store.ensure()
	if err != nil {
		jsonErr(w, http.StatusNotFound, err.Error())
		return
	}
	if err := launchApp(loc, body.ID); err != nil {
		jsonErr(w, http.StatusBadGateway, err.Error())
		return
	}
	jsonOK(w, map[string]string{"ok": "1", "id": body.ID})
}

func (s *apiServer) handleText(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	loc, err := s.store.ensure()
	if err != nil {
		jsonErr(w, http.StatusNotFound, err.Error())
		return
	}
	if err := sendText(loc, body.Text); err != nil {
		jsonErr(w, http.StatusBadGateway, err.Error())
		return
	}
	jsonOK(w, map[string]string{"ok": "1"})
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
