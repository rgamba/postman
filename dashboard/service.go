package dashboard

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/rgamba/postman/async"
	"github.com/rgamba/postman/stats"

	"github.com/spf13/viper"
)

var (
	appConfig  *viper.Viper
	appVersion string
	appBuild   string
)

// StartHTTPServer starts the new HTTP Dashboard service.
func StartHTTPServer(port int, config *viper.Viper, version string, build string) *http.Server {
	appConfig = config
	appVersion = version
	appBuild = build
	mux := http.NewServeMux()
	mux.HandleFunc("/settings", settingsHandler)
	mux.HandleFunc("/", defaultHandler)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Dashboard: ListenAndServe() error: %s", err)
		}
	}()

	return srv
}

//go:generate go-bindata -prefix "assets/" -pkg dashboard -o bindata.go ../assets/...

func staticHandler(rw http.ResponseWriter, req *http.Request) {
	var path string = req.URL.Path
	if path == "" {
		path = "index.html"
	}
	if bs, err := Asset(path); err != nil {
		rw.WriteHeader(http.StatusNotFound)
	} else {
		var reader = bytes.NewBuffer(bs)
		io.Copy(rw, reader)
	}
}

func settingsHandler(w http.ResponseWriter, r *http.Request) {
	context := map[string]interface{}{
		"service":   appConfig.GetStringMap("service"),
		"http":      appConfig.GetStringMap("http"),
		"dashboard": appConfig.GetStringMap("dashboard"),
	}
	renderView(w, "settings.html", context)
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	context := map[string]interface{}{
		"currentServiceName":      appConfig.GetString("service.name"),
		"currentServiceInstances": async.GetServiceInstances(appConfig.GetString("service.name")),
		"processId":               os.Getpid(),
		"requests":                stats.GetRequestsLastMinutePerService(),
		"appVersion":              appVersion,
		"appBuild":                appBuild,
	}
	renderView(w, "index.html", context)
}

func renderView(w http.ResponseWriter, tpl string, data interface{}) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Println(rec)
			http.Error(w, rec.(string), http.StatusInternalServerError)
		}
	}()

	header := string(getStaticAsset("../assets/html/header.html"))
	footer := string(getStaticAsset("../assets/html/footer.html"))

	t := template.New("main")
	tpl = header + string(getStaticAsset("../assets/html/index.html")) + footer
	t.Parse(string(tpl))

	err := t.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func getStaticAsset(name string) (bs []byte) {
	var err error
	if bs, err = Asset(name); err != nil {
		return nil
	}
	return bs
}
