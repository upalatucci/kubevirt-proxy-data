package allowednamespaces

import (
	"log"
	"net/http"
	"sync"
)

const ssarMaxAttempts = 2

func filterAllowedNamespaces(
	client *http.Client,
	incomingReq *http.Request,
	resourceAttributes ResourceAttributes,
	namespaces []string,
	maxConcurrency int,
) ([]string, []NamespaceFailure) {
	if len(namespaces) == 0 {
		return []string{}, nil
	}

	if maxConcurrency > len(namespaces) {
		maxConcurrency = len(namespaces)
	}

	allowed := make([]string, 0, len(namespaces))
	failures := make([]NamespaceFailure, 0)
	var mu sync.Mutex
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	for _, namespace := range namespaces {
		wg.Add(1)
		go func(ns string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			status, err := createSelfSubjectAccessReviewWithRetry(client, incomingReq, resourceAttributes, ns)
			if err != nil {
				log.Printf("Failed selfsubjectaccessreview for namespace %q after %d attempts: %v", ns, ssarMaxAttempts, err)
				mu.Lock()
				failures = append(failures, NamespaceFailure{
					Namespace: ns,
					Error:     err.Error(),
				})
				mu.Unlock()
				return
			}

			if !status.Allowed {
				return
			}

			mu.Lock()
			allowed = append(allowed, ns)
			mu.Unlock()
		}(namespace)
	}

	wg.Wait()
	return allowed, failures
}

func createSelfSubjectAccessReviewWithRetry(
	client *http.Client,
	incomingReq *http.Request,
	resourceAttributes ResourceAttributes,
	namespace string,
) (selfSubjectAccessReviewStatus, error) {
	var lastErr error

	for attempt := 1; attempt <= ssarMaxAttempts; attempt++ {
		status, err := createSelfSubjectAccessReview(client, incomingReq, resourceAttributes, namespace)
		if err == nil {
			return status, nil
		}

		lastErr = err
		log.Printf("SelfSubjectAccessReview attempt %d/%d failed for namespace %q: %v", attempt, ssarMaxAttempts, namespace, err)
	}

	return selfSubjectAccessReviewStatus{}, lastErr
}
