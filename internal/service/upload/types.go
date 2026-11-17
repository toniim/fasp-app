package upload

// Visibility choices for one upload. The server knows only public and private
// files: there is no password, expiry or view count.
const (
	// VisibilityDefault uses the "Upload as public" setting.
	VisibilityDefault = ""
	// VisibilityPublic makes the file open to anyone with the link.
	VisibilityPublic = "public"
	// VisibilityPrivate makes the file open only to its owner.
	VisibilityPrivate = "private"
)

// InitRequest represents the request to initiate an upload
type InitRequest struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
	// IsPublic marks the file public (shareable URLs) or private (owner only).
	// It is always sent, including false.
	IsPublic bool `json:"is_public"`
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
