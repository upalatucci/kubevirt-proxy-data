# Kubevirt UI Proxy

Kubevirt UI proxy data pod resolve the performance issue in large scale cluster by implementing filtering and pagination on data from kube api server in BE instead of UI.

## HTTP API

The pod exposes a small set of first-party endpoints plus a Kubernetes API proxy used by the KubeVirt UI.

### `GET /health`

Liveness and readiness check for the proxy process.

- **Response:** JSON with `status`, `ready`, `kubeAPIServer`, serving TLS file status, and in-cluster CA mount status (see [docs/openapi.yaml](docs/openapi.yaml))
- **Status codes:** `200` when `ready` is true; `503` when running but serving TLS material is missing in production mode
- **Caching:** 30 seconds (by request URI)

OpenAPI 3 description for proxy-owned endpoints: [docs/openapi.yaml](docs/openapi.yaml)

Published API reference (GitHub Pages, after merge to `main`): https://kubevirt-ui.github.io/kubevirt-apiserver-proxy/

Enable **Settings → Pages → Build and deployment → GitHub Actions** if the site is not live yet.

### `POST /apis/allowednamespaces`

Returns the namespaces where the caller is allowed to perform a Kubernetes action, using SelfSubjectAccessReview (SSAR) against the cluster API server.

- **Request body:** JSON with `resourceAttributes` (at minimum `verb`; optional `cluster` for ACM spoke routing, `group`, `version`, `resource`, `subresource`, `namespace`, `name`) and optional `maxConcurrency` (positive integer, server-capped)
- **Authorization:** Bearer token in the `Authorization` header (required)
- **Response:** `allowedNamespaces` (string array) and optional `failures` (per-namespace SSAR errors)
- **Caching:** 2 minutes (keyed by auth headers and request body; responses with `failures` are not cached)

### `GET /api/*` and `GET /apis/*`

Kubernetes has two URL prefixes:

- **`GET /api/*`** — core API (`/api/v1/namespaces/.../persistentvolumeclaims`, Pods, Secrets, …)
- **`GET /apis/*`** — named API groups (`/apis/kubevirt.io/v1/...`). Paths must include **`{group}/{version}/`**; `/apis/v1/...` without a group returns **400** (use `/api/v1/...` for core resources).

Both routes forward to the hub API server when the path has **no** `/cluster/{name}/` segment. For a managed cluster, insert **`cluster/{clusterName}/`** after `/api/` or `/apis/`:

- Hub: `GET /apis/kubevirt.io/v1/namespaces/default/virtualmachines`
- Spoke: `GET /apis/cluster/bm07/kubevirt.io/v1/namespaces/default/virtualmachines`
- Core on spoke: `GET /api/cluster/bm07/v1/namespaces/default/persistentvolumeclaims`

The hub cluster name (`localHubName` from ACM console `GET /hub`, cached) can be used in the path the same way; it still targets the hub API directly. Multicluster routing uses the ACM **cluster-proxy** addon for other cluster names. The caller's `Authorization` header (and websocket subprotocol impersonation headers when applicable) are passed through.

### Multicluster (ACM)

| Variable | Default | Purpose |
|----------|---------|---------|
| `CLUSTER_PROXY_ENABLED` | enabled | Set to `false` to disable cluster-proxy routing |
| `CLUSTER_PROXY_ADDON_HOST` | `cluster-proxy-addon-user.multicluster-engine.svc.cluster.local` | cluster-proxy Service host |
| `CLUSTER_PROXY_ADDON_PORT` | `9092` | cluster-proxy port |
| `ACM_CONSOLE_BACKEND_URL` | MCE console Service (`…:9000`) | Base URL for `GET /hub` hub discovery |
| `ACM_CONSOLE_HUB_URL` | (derived from backend URL) | Full override URL for hub configuration |
| `ACM_HUB_CONFIG_CACHE_TTL` | `24h` | Cache TTL for `localHubName` |

`POST /apis/allowednamespaces` lists namespaces on the target cluster (hub or `resourceAttributes.cluster`) and runs SSAR via cluster-proxy when a managed cluster is selected.

**List filtering (optional):** When the client sends query parameters on a **list** response (`items` array), the proxy can filter items server-side before returning JSON to the UI. This is primarily used for large VirtualMachine lists. Examples:

| Query key | Example value | Behavior |
|-----------|---------------|----------|
| `status.printableStatus` | `Error` | Keeps VMs whose printable status is one of the configured error statuses (for example `CrashLoopBackOff`, `ErrorUnschedulable`) |
| `status.conditions` | `migratable` / `notMigratable` | Filters on live-migration condition |
| `status.interfaces` | IP substring | Filters VMs with a matching interface IP |
| Label-style keys | `app=my-app` | Matches nested label maps |
| `key1\|key2` | value | OR across multiple JSON paths (see `util/util.go`) |

The response includes `totalItems` (count from the upstream list before filtering) and `items` (filtered when query parameters are present).

- **Caching:** 15 seconds (by full request URI, query string, `Authorization`, and impersonation headers)
- **Local dev:** set `APP_ENV=dev` for plain HTTP on port `8080`

## Usage

### Local development

1. `oc login` into the cluster
2. run `./start-proxy.sh`
   - builds a `kubevirt-apiserver-proxy` binary and runs at `localhost:8080`

### Tests

- `go test ./...` — unit tests including health and proxy handlers, allowed-namespaces SSAR logic, and VM list filtering with a stub kube-apiserver.

### Dev mode deployment

Save this YAML to a file locally:

```yaml
---
kind: Deployment
apiVersion: apps/v1
metadata:
  name: kubevirt-apiserver-proxy
  namespace: openshift-cnv
  labels:
    app: kubevirt-apiserver-proxy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kubevirt-apiserver-proxy
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: kubevirt-apiserver-proxy
    spec:
      containers:
        - name: kubevirt-apiserver-proxy
          image: "quay.io/kubevirt-ui/kubevirt-apiserver-proxy"
          ports:
            - containerPort: 8080
              protocol: TCP
          imagePullPolicy: Always
          volumeMounts:
            - name: cert
              mountPath: "/app/cert"
              readOnly: true
      volumes:
        - name: cert
          secret:
            secretName: kubevirt-apiserver-proxy-cert
            optional: true
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 25%
      maxSurge: 25%

---
kind: Service
apiVersion: v1
metadata:
  name: kubevirt-apiserver-proxy
  namespace: openshift-cnv
  annotations:
    service.beta.openshift.io/serving-cert-secret-name: kubevirt-apiserver-proxy-cert
spec:
  ipFamilies:
    - IPv4
  ports:
    - protocol: TCP
      port: 80
      targetPort: 8080
  selector:
    app: kubevirt-apiserver-proxy

---
kind: Route
apiVersion: route.openshift.io/v1
metadata:
  name: kubevirt-apiserver-proxy
  namespace: openshift-cnv
  annotations:
    haproxy.router.openshift.io/hsts_header: max-age=31536000;includeSubDomains;preload
spec:
  host: $HOST #example: kubevirt-apiserver-proxy-openshift-cnv.apps.uit-413-0602.rhos-psi.cnv-qe.rhood.us
  to:
    kind: Service
    name: kubevirt-apiserver-proxy
    weight: 100
  port:
    targetPort: 8080
  tls:
    termination: reencrypt
  wildcardPolicy: None
```

This yaml will create 3 resources.

1. Deployment (pod where the proxy will run)
2. Service
3. Route (in production mode this will not be created - instead kubevirt-plugin will be added to include a proxy so this route is not needed)
