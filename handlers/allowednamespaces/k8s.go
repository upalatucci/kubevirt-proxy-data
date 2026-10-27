package allowednamespaces

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/kubevirt-ui/kubevirt-apiserver-proxy/handlers"
	"github.com/kubevirt-ui/kubevirt-apiserver-proxy/handlers/multicluster"
	"github.com/kubevirt-ui/kubevirt-apiserver-proxy/util"
)

const (
	ssarAPIPath              = "/apis/authorization.k8s.io/v1/selfsubjectaccessreviews"
	namespacesAPIPath        = "/api/v1/namespaces"
	openshiftProjectsAPIPath = "/apis/project.openshift.io/v1/projects"
)

func listAccessibleNamespaces(client *http.Client, incomingReq *http.Request, cluster string) ([]string, error) {
	namespaces, err := fetchResourceNames(client, incomingReq, cluster, namespacesAPIPath)
	if err == nil {
		return namespaces, nil
	}

	apiErr, ok := err.(*handlers.APIError)
	if !ok || apiErr.StatusCode != http.StatusForbidden {
		return nil, err
	}

	return fetchResourceNames(client, incomingReq, cluster, openshiftProjectsAPIPath)
}

func fetchResourceNames(client *http.Client, incomingReq *http.Request, cluster string, apiPath string) ([]string, error) {
	req, err := http.NewRequestWithContext(
		incomingReq.Context(),
		http.MethodGet,
		multicluster.KubernetesURL(cluster, apiPath, incomingReq),
		nil,
	)
	if err != nil {
		return nil, err
	}

	util.ForwardAuthHeaders(incomingReq, req)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &handlers.APIError{
			StatusCode: resp.StatusCode,
			Message:    handlers.StatusMessage(bodyBytes),
		}
	}

	var list namespaceList
	if err := json.Unmarshal(bodyBytes, &list); err != nil {
		return nil, err
	}

	namespaces := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		if item.Metadata.Name != "" {
			namespaces = append(namespaces, item.Metadata.Name)
		}
	}

	return namespaces, nil
}

func createSelfSubjectAccessReview(
	client *http.Client,
	incomingReq *http.Request,
	cluster string,
	resourceAttributes ResourceAttributes,
	namespace string,
) (selfSubjectAccessReviewStatus, error) {
	attrs := resourceAttributes
	attrs.Namespace = namespace
	attrs.Cluster = ""

	requestBody, err := json.Marshal(selfSubjectAccessReview{
		APIVersion: "authorization.k8s.io/v1",
		Kind:       "SelfSubjectAccessReview",
		Spec: struct {
			ResourceAttributes ResourceAttributes `json:"resourceAttributes"`
		}{
			ResourceAttributes: attrs,
		},
	})
	if err != nil {
		return selfSubjectAccessReviewStatus{}, err
	}

	req, err := http.NewRequestWithContext(
		incomingReq.Context(),
		http.MethodPost,
		multicluster.KubernetesURL(cluster, ssarAPIPath, incomingReq),
		bytes.NewReader(requestBody),
	)
	if err != nil {
		return selfSubjectAccessReviewStatus{}, err
	}

	util.ForwardAuthHeaders(incomingReq, req)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return selfSubjectAccessReviewStatus{}, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return selfSubjectAccessReviewStatus{}, err
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return selfSubjectAccessReviewStatus{}, &handlers.APIError{
			StatusCode: resp.StatusCode,
			Message:    handlers.StatusMessage(bodyBytes),
		}
	}

	var ssarResponse selfSubjectAccessReviewResponse
	if err := json.Unmarshal(bodyBytes, &ssarResponse); err != nil {
		return selfSubjectAccessReviewStatus{}, err
	}

	return ssarResponse.Status, nil
}
