package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var httpClient = &http.Client{Timeout: 5 * time.Second}

var allowedKeys = map[string]bool{
	"Home": true, "Rev": true, "Fwd": true, "Play": true, "Select": true,
	"Left": true, "Right": true, "Down": true, "Up": true, "Back": true,
	"InstantReplay": true, "Info": true, "Backspace": true, "Search": true,
	"Enter": true, "VolumeUp": true, "VolumeDown": true, "VolumeMute": true,
	"PowerOff": true, "PowerOn": true, "FindRemote": true,
}

var litKeyRe = regexp.MustCompile(`^Lit_.+`)

type DeviceInfo struct {
	Name    string `json:"name"`
	Model   string `json:"model"`
	Serial  string `json:"serial"`
	ECPMode string `json:"ecpMode"`
	Version string `json:"version"`
}

type App struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type appsXML struct {
	XMLName xml.Name    `xml:"apps"`
	Apps    []appXMLTag `xml:"app"`
}

type appXMLTag struct {
	ID   string `xml:"id,attr"`
	Name string `xml:",chardata"`
}

type deviceInfoXML struct {
	UserDeviceName    string `xml:"user-device-name"`
	FriendlyDeviceName string `xml:"friendly-device-name"`
	ModelName         string `xml:"model-name"`
	SerialNumber      string `xml:"serial-number"`
	ECPSettingMode    string `xml:"ecp-setting-mode"`
	SoftwareVersion   string `xml:"software-version"`
}

type activeAppXML struct {
	App struct {
		Name string `xml:",chardata"`
	} `xml:"app"`
}

func normalizeLocation(loc string) string {
	loc = strings.TrimSpace(loc)
	if loc == "" {
		return ""
	}
	if !strings.HasSuffix(loc, "/") {
		loc += "/"
	}
	return loc
}

func validLocation(loc string) bool {
	u, err := url.Parse(loc)
	if err != nil || u.Scheme != "http" || u.Host == "" {
		return false
	}
	return true
}

func ecpGet(loc, path string) (*http.Response, error) {
	return httpClient.Get(normalizeLocation(loc) + strings.TrimPrefix(path, "/"))
}

func ecpPost(loc, path string) (*http.Response, error) {
	return httpClient.PostForm(normalizeLocation(loc)+strings.TrimPrefix(path, "/"), nil)
}

func queryDeviceInfo(loc string) (*DeviceInfo, error) {
	resp, err := ecpGet(loc, "query/device-info")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("device-info %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var x deviceInfoXML
	if err := xml.NewDecoder(resp.Body).Decode(&x); err != nil {
		return nil, err
	}
	name := x.UserDeviceName
	if name == "" {
		name = x.FriendlyDeviceName
	}
	if name == "" {
		name = "Roku"
	}
	return &DeviceInfo{
		Name:    name,
		Model:   x.ModelName,
		Serial:  x.SerialNumber,
		ECPMode: x.ECPSettingMode,
		Version: x.SoftwareVersion,
	}, nil
}

func queryActiveApp(loc string) string {
	resp, err := ecpGet(loc, "query/active-app")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var x activeAppXML
	if err := xml.NewDecoder(resp.Body).Decode(&x); err != nil {
		return ""
	}
	return strings.TrimSpace(x.App.Name)
}

func queryApps(loc string) ([]App, error) {
	resp, err := ecpGet(loc, "query/apps")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, ecpErr(resp.StatusCode, body)
	}
	var x appsXML
	if err := xml.Unmarshal(body, &x); err != nil {
		return nil, err
	}
	out := make([]App, 0, len(x.Apps))
	for _, a := range x.Apps {
		out = append(out, App{ID: a.ID, Name: strings.TrimSpace(a.Name)})
	}
	return out, nil
}

func keypress(loc, key string) error {
	if !validKey(key) {
		return fmt.Errorf("key not allowed: %s", key)
	}
	// Lit_ payloads may include %XX; don't let http.Client re-encode oddly
	path := "keypress/" + key
	resp, err := ecpPost(loc, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if resp.StatusCode != http.StatusOK {
		return ecpErr(resp.StatusCode, body)
	}
	return nil
}

func litKey(ch string) string {
	if ch == "" {
		return ""
	}
	esc := url.QueryEscape(ch)
	esc = strings.ReplaceAll(esc, "+", "%20")
	return "Lit_" + esc
}

func launchApp(loc, id string) error {
	id = strings.TrimSpace(id)
	if id == "" || strings.ContainsAny(id, "/?&#") {
		return fmt.Errorf("invalid app id")
	}
	resp, err := ecpPost(loc, "launch/"+id)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if resp.StatusCode != http.StatusOK {
		return ecpErr(resp.StatusCode, body)
	}
	return nil
}

func searchBrowse(loc, keyword string) error {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return fmt.Errorf("empty search")
	}
	u := normalizeLocation(loc) + "search/browse?keyword=" + url.QueryEscape(keyword)
	resp, err := httpClient.PostForm(u, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if resp.StatusCode != http.StatusOK {
		return ecpErr(resp.StatusCode, body)
	}
	return nil
}

func sendText(loc, text string) error {
	for _, r := range text {
		if r == utf8.RuneError {
			continue
		}
		if err := keypress(loc, litKey(string(r))); err != nil {
			return err
		}
	}
	return nil
}

func validKey(key string) bool {
	if allowedKeys[key] {
		return true
	}
	if litKeyRe.MatchString(key) {
		return true
	}
	// case-insensitive common keys from CLI
	for k := range allowedKeys {
		if strings.EqualFold(k, key) {
			return true
		}
	}
	return false
}

func normalizeKey(key string) string {
	if litKeyRe.MatchString(key) {
		return key
	}
	for k := range allowedKeys {
		if strings.EqualFold(k, key) {
			return k
		}
	}
	return key
}

func ecpErr(code int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	if code == http.StatusForbidden {
		if msg == "" {
			msg = "ECP command not allowed"
		}
		return fmt.Errorf("%s (HTTP 403). Enable: Settings → System → Advanced system settings → Control by mobile apps → Network access → Enabled", msg)
	}
	if msg == "" {
		msg = http.StatusText(code)
	}
	return fmt.Errorf("ECP %d: %s", code, msg)
}
