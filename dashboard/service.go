package dashboard

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
)

// StartHTTPServer starts the new HTTP Dashboard service.
func StartHTTPServer(port int) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/stats/messages", messageStatsHandler)
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

func messageStatsHandler(w http.ResponseWriter, r *http.Request) {

}

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	renderView(w, "index.html", nil)
}

func renderView(w http.ResponseWriter, tpl string, data interface{}) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Println(rec)
			http.Error(w, rec.(string), http.StatusInternalServerError)
		}
	}()

	root := "assets/html/"
	t := template.Must(template.ParseFiles(root+tpl, root+"header.html", root+"footer.html"))

	err := t.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
