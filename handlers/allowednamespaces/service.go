package allowednamespaces

import (
	"log"
	"net/http"
	"sort"
	"sync"
)

const ssarMaxAttempts = 2

func filterAllowedNamespaces(
	client *http.Client,
	incomingReq *http.Request,
	resourceAttributes ResourceAttributes,
	candidates []namespaceCandidate,
	maxConcurrency int,
) ([]string, []NamespaceFailure) {
	if len(candidates) == 0 {
		return []string{}, nil
	}

	if maxConcurrency > len(candidates) {
		maxConcurrency = len(candidates)
	}

	allowed := make([]string, 0, len(candidates))
	failures := make([]NamespaceFailure, 0)
	var mu sync.Mutex
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	for _, candidate := range candidates {
		wg.Add(1)
		go func(candidate namespaceCandidate) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			status, err := createSelfSubjectAccessReviewWithRetry(
				client,
				incomingReq,
				candidate.cluster,
				resourceAttributes,
				candidate.name,
			)
			if err != nil {
				log.Printf(
					"Failed selfsubjectaccessreview for namespace %q (cluster %q) after %d attempts: %v",
					candidate.name,
					candidate.cluster,
					ssarMaxAttempts,
					err,
				)
				mu.Lock()
				failures = append(failures, NamespaceFailure{
					Namespace: candidate.name,
					Error:     err.Error(),
				})
				mu.Unlock()
				return
			}

			if !status.Allowed {
				return
			}

			mu.Lock()
			allowed = append(allowed, candidate.name)
			mu.Unlock()
		}(candidate)
	}

	wg.Wait()
	sort.Strings(allowed)
	sort.Slice(failures, func(i, j int) bool {
		return failures[i].Namespace < failures[j].Namespace
	})
	return allowed, failures
}

func createSelfSubjectAccessReviewWithRetry(
	client *http.Client,
	incomingReq *http.Request,
	cluster string,
	resourceAttributes ResourceAttributes,
	namespace string,
) (selfSubjectAccessReviewStatus, error) {
	var lastErr error

	for attempt := 1; attempt <= ssarMaxAttempts; attempt++ {
		status, err := createSelfSubjectAccessReview(client, incomingReq, cluster, resourceAttributes, namespace)
		if err == nil {
			return status, nil
		}

		lastErr = err
		log.Printf("SelfSubjectAccessReview attempt %d/%d failed for namespace %q: %v", attempt, ssarMaxAttempts, namespace, err)
	}

	return selfSubjectAccessReviewStatus{}, lastErr
}
