package model

// APIEnvelope is the standard JSON response wrapper used by all endpoints.
type APIEnvelope struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OK 返回 200 封包。
func OK(data interface{}) APIEnvelope {
	return APIEnvelope{Code: 0, Message: "success", Data: data}
}

// Err 返回错误封包。
func Err(code int, msg string) APIEnvelope {
	return APIEnvelope{Code: code, Message: msg}
}
