package allowednamespaces

type selfSubjectAccessReview struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Spec       struct {
		ResourceAttributes ResourceAttributes `json:"resourceAttributes"`
	} `json:"spec"`
}

type selfSubjectAccessReviewStatus struct {
	Allowed bool `json:"allowed"`
}

type selfSubjectAccessReviewResponse struct {
	Status selfSubjectAccessReviewStatus `json:"status"`
}

type namespaceList struct {
	Items []namespaceItem `json:"items"`
}

type namespaceItem struct {
	Metadata namespaceMetadata `json:"metadata"`
}

type namespaceMetadata struct {
	Name string `json:"name"`
}
