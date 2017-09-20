package lib

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// SendJSON sends a JSON response.
func SendJSON(w http.ResponseWriter, arr interface{}, statusCode int) {
	content, err := json.Marshal(arr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	SendResponse(w, content, statusCode)
}

// SendResponse is a shortcut to send http responses.
func SendResponse(w http.ResponseWriter, content []byte, statusCode int) {
	if content == nil {
		content = []byte{0x00}
	}
	contentLength := strconv.Itoa(len(content))
	w.Header().Set("Content-Length", contentLength)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Server", "Postman")

	w.WriteHeader(statusCode)
	w.Write(content)
}
