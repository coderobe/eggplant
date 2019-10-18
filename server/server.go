package server

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/NYTimes/gziphandler"
	"github.com/boreq/eggplant/library"
	"github.com/boreq/eggplant/logging"
	"github.com/boreq/eggplant/server/api"
	_ "github.com/boreq/eggplant/statik"
	"github.com/boreq/eggplant/store"
	"github.com/julienschmidt/httprouter"
	"github.com/rakyll/statik/fs"
	"github.com/rs/cors"
)

var log = logging.New("server")

type handler struct {
	library        *library.Library
	trackStore     *store.TrackStore
	thumbnailStore *store.ThumbnailStore
}

func (h *handler) Browse(r *http.Request, ps httprouter.Params) (interface{}, api.Error) {
	path := ps.ByName("path")
	path = strings.Trim(path, "/")

	log.Debug("path", "path", path)

	var dirs []string
	if path != "" {
		dirs = strings.Split(path, "/")
	}

	var ids []library.Id
	for _, name := range dirs {
		ids = append(ids, library.Id(name))
	}

	d, err := h.library.Browse(ids)
	if err != nil {
		log.Error("browse error", "err", err)
		return nil, api.InternalServerError
	}

	return d, nil
}

var isIdValid = regexp.MustCompile(`^[a-zA-Z0-9]+$`).MatchString

func (h *handler) Track(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := trimExtension(ps.ByName("id"))
	if !isIdValid(id) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Add("Accept-Ranges", "bytes")
	h.trackStore.ServeFile(w, r, id)
}

func trimExtension(s string) string {
	if index := strings.LastIndex(s, "."); index >= 0 {
		s = s[:index]
	}
	return s
}

func Serve(l *library.Library, trackStore *store.TrackStore, thumbnailStore *store.ThumbnailStore, address string) error {
	handler, err := newHandler(l, trackStore, thumbnailStore)
	if err != nil {
		return err
	}

	// Add CORS middleware
	handler = cors.AllowAll().Handler(handler)

	// Add GZIP middleware
	handler = gziphandler.GzipHandler(handler)

	log.Info("starting listening", "address", address)
	return http.ListenAndServe(address, handler)
}

func newHandler(l *library.Library, trackStore *store.TrackStore, thumbnailStore *store.ThumbnailStore) (http.Handler, error) {
	h := &handler{
		library:        l,
		trackStore:     trackStore,
		thumbnailStore: thumbnailStore,
	}

	statikFS, err := fs.New()
	if err != nil {
		return nil, err
	}

	router := httprouter.New()

	// API
	router.GET("/api/browse/*path", api.Wrap(h.Browse))
	router.GET("/api/track/:id", h.Track)

	// Frontend
	router.NotFound = http.FileServer(statikFS)

	return router, nil
}
