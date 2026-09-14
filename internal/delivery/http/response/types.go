package response

type Response struct {
	Status  int    `json:"status"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Errors  any    `json:"errors,omitempty"`
}
