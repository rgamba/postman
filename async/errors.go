package async

import (
	"encoding/json"
	"fmt"
)

// Error is the async specific error
// representation.
type Error struct {
	Code        string
	Message     string
	Description string
	Meta        interface{}
}

// Error gets the general error description
func (err Error) Error() string {
	return fmt.Sprintf("%s: %s", err.Code, err.Message)
}

// JSON gets a json string representation of the error.
func (err Error) JSON() string {
	errHash := err.ToMap()
	str, _ := json.Marshal(errHash)
	return string(str)
}

func (err Error) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"code":     err.Code,
		"error":    err.Message,
		"metadata": err.Meta,
	}
}

func createError(code string, message string, meta interface{}) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Meta:    meta,
	}
}
