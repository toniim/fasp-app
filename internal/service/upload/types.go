package upload

// InitRequest represents the request to initiate an upload
type InitRequest struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
}

// InitResponse represents the response from upload initialization
type InitResponse struct {
	FileID    string `json:"file_id"`
	UploadURL string `json:"upload_url"`
}

// CompleteRequest represents the request to complete an upload
type CompleteRequest struct {
	FileID string `json:"file_id"`
}

// CompleteResponse represents the response from upload completion
type CompleteResponse struct {
	PublicURL string `json:"public_url"`
	DirectURL string `json:"direct_url"`
}
