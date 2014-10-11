package ivy

import (
	"net/http"
	"os"
	"strconv"
)

type Ivy struct {
	http.ServeMux
	bus    *bus
	logger *logger
}

func NewIvy(logDir string) *Ivy {
	logger, err := newLogger(logDir)
	if err != nil {
		panic(err)
	}

	ivy := &Ivy{
		ServeMux: *http.NewServeMux(),
		bus:      newBus(logger),
		logger:   logger,
	}
	ivy.HandleFunc("/ws", ivy.serveWs)
	ivy.Handle("/log/", http.StripPrefix("/log/", http.HandlerFunc(ivy.serveLog)))
	return ivy
}

func (ivy *Ivy) serveWs(w http.ResponseWriter, r *http.Request) {
	connectWs(ivy.bus, w, r)
}

func (ivy *Ivy) serveLog(w http.ResponseWriter, r *http.Request) {
	lineQuery := r.FormValue("n")
	var lines int
	if lineQuery != "" {
		parsedLines, err := strconv.ParseInt(lineQuery, 10, 0)
		if err != nil || parsedLines < 1 {
			http.Error(w, "invalid line count", 400)
			return
		}
		lines = int(parsedLines)
	} else {
		lines = 0
	}

	at := r.FormValue("at")
	startTime := at
	endTime := at

	path := r.URL.Path

	logReader, modTime, err := ivy.logger.readGlobbedLog("/"+path, lines, startTime, endTime)
	if os.IsNotExist(err) {
		http.Error(w, "Not Found", 404)
		return
	} else if err == ErrInvalidGlob {
		http.Error(w, "invalid glob path", 400)
		return
	} else if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	http.ServeContent(w, r, "log.txt", modTime, logReader)
}

func (ivy *Ivy) Start() {
	go ivy.bus.run()
	go ivy.logger.run()
}
