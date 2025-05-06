package dto

type Secret struct {
	Message string `json:"message"`
	OneTime bool   `json:"one_time,omitempty"`
}
