package model

// APIEnvelope is the standard JSON response wrapper used by all endpoints.
type APIEnvelope struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OK returns a 200 envelope with the given data.
func OK(data interface{}) APIEnvelope {
	return APIEnvelope{Code: 0, Message: "success", Data: data}
}

// Err returns an error envelope.
func Err(code int, msg string) APIEnvelope {
	return APIEnvelope{Code: code, Message: msg}
}
