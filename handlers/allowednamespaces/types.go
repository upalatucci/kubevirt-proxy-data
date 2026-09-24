package allowednamespaces

type AllowedNamespacesRequest struct {
	ResourceAttributes ResourceAttributes `json:"resourceAttributes"`
}

type AllowedNamespacesResponse struct {
	AllowedNamespaces []string           `json:"allowedNamespaces"`
	Failures          []NamespaceFailure `json:"failures,omitempty"`
}

type NamespaceFailure struct {
	Namespace string `json:"namespace"`
	Error     string `json:"error"`
}

type ResourceAttributes struct {
	Namespace   string `json:"namespace,omitempty"`
	Verb        string `json:"verb"`
	Group       string `json:"group,omitempty"`
	Version     string `json:"version,omitempty"`
	Resource    string `json:"resource,omitempty"`
	Subresource string `json:"subresource,omitempty"`
	Name        string `json:"name,omitempty"`
}
