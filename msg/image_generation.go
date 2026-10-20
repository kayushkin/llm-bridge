package msg

// ImageGenerationRequest asks for one or more images from a prompt, and
// optionally from reference images to edit or follow.
//
// It is the body of llm-bridge-server's POST /images and
// POST /sessions/{id}/images, and what the server writes to a harness binary's
// stdin in -generate-image mode. A harness that implements that mode reads one
// of these and writes one ImageGenerationResponse to stdout, or
// {"error":"..."} and a non-zero exit.
type ImageGenerationRequest struct {
	// Prompt says what to draw. Required.
	Prompt string `json:"prompt"`

	// ReferenceImagePaths are absolute paths, on the host the harness runs on,
	// of images to edit or to take a character, style or layout from.
	ReferenceImagePaths []string `json:"reference_image_paths,omitempty"`

	// Caller names the feature or agent asking, for logs. Free text, for
	// display only.
	Caller string `json:"caller,omitempty"`
}

// GeneratedImage is one image file a harness wrote.
type GeneratedImage struct {
	// Path is where the harness saved the image, on the host it runs on.
	Path string `json:"path"`

	// MediaType is the image's type as read from its bytes ("image/png").
	MediaType string `json:"media_type"`

	SizeBytes int64 `json:"size_bytes"`
}

// ImageGenerationResponse is what a harness in -generate-image mode returns,
// and what the server's image routes answer with.
type ImageGenerationResponse struct {
	// Images are the files the call made, in the order the harness wrote
	// them. Never empty on success: a call that made no image is an error.
	Images []GeneratedImage `json:"images"`

	// Text is whatever the model said alongside the images, passed through
	// unchanged.
	Text string `json:"text,omitempty"`

	// Usage is the token use of the model that drove the image tool, as the
	// harness reported it.
	Usage TokenUsage `json:"usage"`

	// DurationMs is the wall-clock time of the call.
	DurationMs int64 `json:"duration_ms"`

	// InstanceID is the harness instance that made the images. Set by the
	// server, which picks it.
	InstanceID string `json:"instance_id,omitempty"`

	// SessionFiles are the images as shared into a session's chat, in the
	// order of Images. Set only by POST /sessions/{id}/images.
	SessionFiles []SessionFile `json:"session_files,omitempty"`
}
