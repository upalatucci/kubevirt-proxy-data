package allowednamespaces

import (
	"net/http"
)

type namespaceCandidate struct {
	name    string
	cluster string
}

func listNamespaceCandidates(
	client *http.Client,
	incomingReq *http.Request,
	cluster string,
) ([]namespaceCandidate, error) {
	namespaces, err := listAccessibleNamespaces(client, incomingReq, cluster)
	if err != nil {
		return nil, err
	}
	candidates := make([]namespaceCandidate, 0, len(namespaces))
	for _, name := range namespaces {
		candidates = append(candidates, namespaceCandidate{name: name, cluster: cluster})
	}
	return candidates, nil
}
