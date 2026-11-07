// Package dashboard hosts bundled assets in AppKit/WKWebView. The bridge has no network listener.
package dashboard

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa -framework WebKit -framework AVKit -framework AVFoundation -framework CoreMedia
#include <stdlib.h>
#include "dashboard.h"
*/
import "C"
import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"unsafe"

	"kian/internal/app"
	"kian/internal/models"
	"kian/internal/session"
)

//go:embed assets/*
var assets embed.FS
var current *Host
var currentMu sync.Mutex

type Host struct {
	Controller *app.Controller
	OnEvent    func(app.Event)
	OnQuit     func()
	sendMu     sync.Mutex
}

func New(c *app.Controller) *Host { return &Host{Controller: c} }
func str(s string) *C.char        { return C.CString(s) }
func (h *Host) Init() {
	currentMu.Lock()
	current = h
	currentMu.Unlock()
	html, _ := assets.ReadFile("assets/index.html")
	css, _ := assets.ReadFile("assets/style.css")
	js, _ := assets.ReadFile("assets/app.js")
	page := strings.ReplaceAll(string(html), "/*STYLE*/", string(css))
	page = strings.ReplaceAll(page, "/*SCRIPT*/", "try {\n"+string(js)+"\n} catch(e) {document.getElementById('error').hidden=false;document.querySelector('#error span').textContent=String(e.stack||e);}")
	p := str(page)
	C.dashboardInit(p)
	C.free(unsafe.Pointer(p))
	go func() {
		for e := range h.Controller.Events {
			if h.OnEvent != nil {
				h.OnEvent(e)
			}
			h.Send(e)
			h.Controller.AfterEvent()
		}
	}()
}
func (h *Host) Show() { C.dashboardShow() }
func (h *Host) Send(v any) {
	h.sendMu.Lock()
	defer h.sendMu.Unlock()
	b, e := json.Marshal(v)
	if e != nil {
		return
	}
	p := str(string(b))
	C.dashboardSend(p)
	C.free(unsafe.Pointer(p))
}

//export goDashboardMessage
func goDashboardMessage(raw *C.char) {
	b := C.GoString(raw)
	currentMu.Lock()
	h := current
	currentMu.Unlock()
	if h != nil {
		go h.handle(b)
	}
}

type request struct {
	ID     int             `json:"id"`
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}

func decode(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, v)
}
func (h *Host) handle(raw string) {
	var r request
	if json.Unmarshal([]byte(raw), &r) != nil {
		return
	}
	data, err := h.dispatch(r)
	reply := map[string]any{"id": r.ID, "result": data}
	if err != nil {
		reply["error"] = err.Error()
	}
	h.Send(app.Event{Type: "reply", Data: reply})
}
func (h *Host) dispatch(r request) (any, error) {
	c := h.Controller
	switch r.Action {
	case "diagnostic":
		var message string
		_ = decode(r.Data, &message)
		log.Printf("dashboard: %s", message)
		return nil, nil
	case "quit":
		if h.OnQuit != nil {
			go h.OnQuit()
		}
		return nil, nil
	case "snapshot":
		return c.Snapshot(), nil
	case "search":
		var q string
		_ = decode(r.Data, &q)
		q = strings.ToLower(q)
		items, e := c.Store.List()
		if e != nil {
			return nil, e
		}
		ids := []string{}
		for _, item := range items {
			v, e := c.Store.Get(item.ID)
			if e != nil {
				continue
			}
			match := strings.Contains(strings.ToLower(v.Title), q)
			for _, seg := range v.Segments {
				if strings.Contains(strings.ToLower(session.Text(v, seg)), q) {
					match = true
					break
				}
			}
			if match {
				ids = append(ids, v.ID)
			}
		}
		return ids, nil
	case "start":
		return nil, c.Start()
	case "stop":
		c.Stop()
		return nil, nil
	case "mode":
		var mode int
		if e := decode(r.Data, &mode); e != nil {
			return nil, e
		}
		return nil, c.SetMode(audioMode(mode))
	case "get":
		var id string
		_ = decode(r.Data, &id)
		return c.Store.Get(id)
	case "rename":
		var d struct{ ID, Title string }
		_ = decode(r.Data, &d)
		if strings.TrimSpace(d.Title) == "" {
			return nil, errors.New("enter a session title")
		}
		e := c.Store.Update(d.ID, func(v *session.Session) error { v.Title = strings.TrimSpace(d.Title); return nil })
		c.Emit("sessions", nil)
		return nil, e
	case "delete":
		var id string
		_ = decode(r.Data, &id)
		e := c.Delete(id)
		c.Emit("sessions", nil)
		return nil, e
	case "correct":
		var d struct {
			ID, Segment string
			Text        *string
		}
		_ = decode(r.Data, &d)
		e := c.Store.Correct(d.ID, d.Segment, d.Text)
		if e == nil {
			c.Emit("correction", d)
		}
		return nil, e
	case "enqueue":
		var d struct {
			Sources []string
			Kind    string
			Retain  bool
		}
		if e := decode(r.Data, &d); e != nil {
			return nil, e
		}
		for _, p := range d.Sources {
			if e := c.Add(p, d.Kind, d.Retain); e != nil {
				return nil, e
			}
		}
		return nil, nil
	case "cancel", "retry":
		var id string
		_ = decode(r.Data, &id)
		if r.Action == "cancel" {
			return nil, c.CancelJob(id)
		}
		return nil, c.Retry(id)
	case "probe", "metadata":
		var source string
		_ = decode(r.Data, &source)
		ctx, cancel := contextTimeout()
		defer cancel()
		if r.Action == "probe" {
			return probe(ctx, source)
		}
		return fetch(ctx, source)
	case "pickFiles":
		C.dashboardPickFiles(0)
		return nil, nil
	case "pickModel":
		C.dashboardPickFiles(1)
		return nil, nil
	case "settings":
		var s models.Settings
		if e := decode(r.Data, &s); e != nil {
			return nil, e
		}
		return nil, c.Configure(s)
	case "downloadModel":
		return nil, c.DownloadModel()
	case "verifyModel":
		go c.Discover()
		return nil, nil
	case "export", "copy":
		var d struct{ ID, Format, Mode string }
		_ = decode(r.Data, &d)
		v, e := c.Store.Get(d.ID)
		if e != nil {
			return nil, e
		}
		text, e := session.Export(v, d.Format, d.Mode)
		if e != nil {
			return nil, e
		}
		p := str(text)
		defer C.free(unsafe.Pointer(p))
		if r.Action == "copy" {
			C.dashboardCopy(p)
		} else {
			name := strings.Map(func(r rune) rune {
				if strings.ContainsRune("/:\\\x00", r) {
					return '-'
				}
				return r
			}, v.Title) + "." + d.Format
			n := str(name)
			defer C.free(unsafe.Pointer(n))
			C.dashboardSave(n, p)
		}
		return nil, nil
	case "top":
		var enabled bool
		_ = decode(r.Data, &enabled)
		if enabled {
			C.dashboardTop(1)
		} else {
			C.dashboardTop(0)
		}
		return nil, nil
	case "play":
		var d struct {
			ID      string
			Seconds float64
		}
		_ = decode(r.Data, &d)
		v, e := c.Store.Get(d.ID)
		if e != nil {
			return nil, e
		}
		if v.MediaPath == "" {
			return nil, errors.New("local media was not retained; the original source is listed above")
		}
		if _, e = os.Stat(v.MediaPath); e != nil {
			return nil, e
		}
		p := str(v.MediaPath)
		defer C.free(unsafe.Pointer(p))
		C.dashboardPlay(p, C.double(max(0, d.Seconds)))
		return nil, nil
	case "reveal":
		var id string
		_ = decode(r.Data, &id)
		path := c.Store.Dir
		if id != "" {
			v, e := c.Store.Get(id)
			if e != nil {
				return nil, e
			}
			if v.MediaPath != "" {
				path = v.MediaPath
			} else if strings.HasPrefix(v.Source, "https://") || strings.HasPrefix(v.Source, "http://") {
				path = v.Source
			}
		}
		return nil, exec.Command("open", path).Start()
	}
	return nil, fmt.Errorf("unknown action: %s", r.Action)
}

func NotifyExisting() { C.dashboardActivate() }
