package config

// Expect holds the assertions evaluated against a response. All fields are
// optional; every set field must hold for the request to pass.
type Expect struct {
	// Status is the exact expected HTTP status code (100-599).
	Status int `yaml:"status,omitempty"`
	// Contains is a substring the response body must include.
	Contains string `yaml:"contains,omitempty"`
	// MaxMS is the maximum allowed request duration in milliseconds.
	MaxMS int64 `yaml:"max_ms,omitempty"`
}

// Request defines a single API call inside a collection.
type Request struct {
	// Method defaults to GET when omitted.
	Method string `yaml:"method,omitempty"`
	URL    string `yaml:"url"`
	// Headers are sent verbatim; both keys and values may reference variables.
	Headers map[string]string `yaml:"headers,omitempty"`
	// Body is an arbitrary YAML value (typically a mapping) rendered as JSON.
	Body any `yaml:"body,omitempty"`
	// Vars are request-scoped variables; they may reference outer variables.
	Vars map[string]string `yaml:"vars,omitempty"`
	// Extract maps variable names to response references such as
	// {{response.body.token}}; extracted values are visible to later requests.
	Extract map[string]string `yaml:"extract,omitempty"`
	// Expect holds the assertions for this request. A request without Expect
	// passes as long as the HTTP exchange itself succeeds.
	Expect *Expect `yaml:"expect,omitempty"`
}

// NamedRequest pairs a request with its name inside the collection file.
type NamedRequest struct {
	Name    string
	Request Request
}

// Collection is an ordered, named set of requests plus file-level variables.
// Request order matches the order in the YAML mapping.
type Collection struct {
	Name     string
	Vars     map[string]string
	Requests []NamedRequest
}
