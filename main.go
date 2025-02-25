package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"crypto/tls"
	"log"
	"net/http"

	"github.com/glimesh/broadcast-box/internal/relay"
	"github.com/glimesh/broadcast-box/internal/webrtc"
	"github.com/joho/godotenv"
)

const (
	envFileProd = ".env.production"
	envFileDev  = ".env.development"
)

type (
	whepLayerRequestJSON struct {
		MediaId    string `json:"mediaId"`
		EncodingId string `json:"encodingId"`
	}
)

func logHTTPError(w http.ResponseWriter, err string, code int) {
	log.Println(err)
	http.Error(w, err, code)
}

func whipHandler(res http.ResponseWriter, r *http.Request) {
	streamKey := r.Header.Get("Authorization")
	if streamKey == "" {
		logHTTPError(res, "Authorization was not set", http.StatusBadRequest)
		return
	}

	offer, err := io.ReadAll(r.Body)
	if err != nil {
		logHTTPError(res, err.Error(), http.StatusBadRequest)
		return
	}

	answer, err := webrtc.WHIP(string(offer), streamKey)
	if err != nil {
		logHTTPError(res, err.Error(), http.StatusBadRequest)
		return
	}

	res.Header().Add("Location", "/api/whip")
	res.WriteHeader(http.StatusCreated)
	fmt.Fprint(res, answer)
}

func whepHandler(res http.ResponseWriter, req *http.Request) {
	streamKey := req.Header.Get("Authorization")
	if streamKey == "" {
		logHTTPError(res, "Authorization was not set", http.StatusBadRequest)
		return
	}

	offer, err := io.ReadAll(req.Body)
	if err != nil {
		logHTTPError(res, err.Error(), http.StatusBadRequest)
		return
	}

	answer, whepSessionId, err := webrtc.WHEP(string(offer), streamKey)
	if err != nil {
		logHTTPError(res, err.Error(), http.StatusBadRequest)
		return
	}

	apiPath := req.Host + strings.TrimSuffix(req.URL.RequestURI(), "whep")
	res.Header().Add("Link", `<`+apiPath+"sse/"+whepSessionId+`>; rel="urn:ietf:params:whep:ext:core:server-sent-events"; events="layers"`)
	res.Header().Add("Link", `<`+apiPath+"layer/"+whepSessionId+`>; rel="urn:ietf:params:whep:ext:core:layer"`)
	res.Header().Add("Location", "/api/whep")
	res.WriteHeader(http.StatusCreated)
	fmt.Fprint(res, answer)
}

func whepServerSentEventsHandler(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "text/event-stream")
	res.Header().Set("Cache-Control", "no-cache")
	res.Header().Set("Connection", "keep-alive")

	vals := strings.Split(req.URL.RequestURI(), "/")
	whepSessionId := vals[len(vals)-1]

	layers, err := webrtc.WHEPLayers(whepSessionId)
	if err != nil {
		logHTTPError(res, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Fprint(res, "event: layers\n")
	fmt.Fprintf(res, "data: %s\n", string(layers))
	fmt.Fprint(res, "\n\n")
}

func whepLayerHandler(res http.ResponseWriter, req *http.Request) {
	var r whepLayerRequestJSON
	if err := json.NewDecoder(req.Body).Decode(&r); err != nil {
		logHTTPError(res, err.Error(), http.StatusBadRequest)
		return
	}

	vals := strings.Split(req.URL.RequestURI(), "/")
	whepSessionId := vals[len(vals)-1]

	if err := webrtc.WHEPChangeLayer(whepSessionId, r.EncodingId); err != nil {
		logHTTPError(res, err.Error(), http.StatusBadRequest)
		return
	}
}

type StreamStatus struct {
	StreamKey string `json:"streamKey"`
}

func statusHandler(res http.ResponseWriter, req *http.Request) {
	statuses := []StreamStatus{}
	for _, s := range webrtc.GetAllStreams() {
		statuses = append(statuses, StreamStatus{StreamKey: s})
	}

	if err := json.NewEncoder(res).Encode(statuses); err != nil {
		logHTTPError(res, err.Error(), http.StatusBadRequest)
	}
}

func indexHTMLWhenNotFound(fs http.FileSystem) http.Handler {
	fileServer := http.FileServer(fs)

	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		_, err := fs.Open(path.Clean(req.URL.Path)) // Do not allow path traversals.
		if errors.Is(err, os.ErrNotExist) {
			http.ServeFile(resp, req, "./web/build/index.html")

			return
		}
		fileServer.ServeHTTP(resp, req)
	})
}

func corsHandler(next func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Access-Control-Allow-Origin", "*")
		res.Header().Set("Access-Control-Allow-Methods", "*")
		res.Header().Set("Access-Control-Allow-Headers", "*")
		res.Header().Set("Access-Control-Expose-Headers", "*")

		if req.Method != http.MethodOptions {
			next(res, req)
		}
	}
}

func main() {
	if os.Getenv("APP_ENV") == "production" {
		log.Println("Loading `" + envFileProd + "`")

		if err := godotenv.Load(envFileProd); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Println("Loading `" + envFileDev + "`")

		if err := godotenv.Load(envFileDev); err != nil {
			log.Fatal(err)
		}
	}

	webrtc.Configure()

	relay.InitRelay(webrtc.GetWhepClient())

	if os.Getenv("ENABLE_HTTP_REDIRECT") != "" {
		go func() {
			redirectServer := &http.Server{
				Addr: ":80",
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, "https://"+r.Host+r.URL.String(), http.StatusMovedPermanently)
				}),
			}

			log.Println("Running HTTP->HTTPS redirect Server at :80")
			log.Fatal(redirectServer.ListenAndServe())
		}()

	}

	mux := http.NewServeMux()
	mux.Handle("/", indexHTMLWhenNotFound(http.Dir("./web/build")))
	mux.HandleFunc("/api/whip", corsHandler(whipHandler))
	mux.HandleFunc("/api/whep", corsHandler(whepHandler))
	mux.HandleFunc("/api/status", corsHandler(statusHandler))
	mux.HandleFunc("/api/sse/", corsHandler(whepServerSentEventsHandler))
	mux.HandleFunc("/api/layer/", corsHandler(whepLayerHandler))

	server := &http.Server{
		Handler: mux,
		Addr:    os.Getenv("HTTP_ADDRESS"),
	}

	tlsKey := os.Getenv("SSL_KEY")
	tlsCert := os.Getenv("SSL_CERT")

	if tlsKey != "" && tlsCert != "" {
		server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{},
		}

		cert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
		if err != nil {
			log.Fatal(err)
		}

		server.TLSConfig.Certificates = append(server.TLSConfig.Certificates, cert)

		log.Println("Running HTTPS Server at `" + os.Getenv("HTTP_ADDRESS") + "`")
		log.Fatal(server.ListenAndServeTLS("", ""))
	} else {
		log.Println("Running HTTP Server at `" + os.Getenv("HTTP_ADDRESS") + "`")
		log.Fatal(server.ListenAndServe())
	}

}
