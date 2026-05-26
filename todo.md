# KaaS Bifrost — Project TODO
 
> **KaaS Bifrost** is a self-hosted Kubernetes multi-tenancy platform.
> Each tenant gets an isolated realm on shared infrastructure — connected by the bridge, separated by design.
 
---
 
## Project Structure
 
```
bifrost/
├── cmd/bifrost/            ← CLI entry point
├── internal/
│   ├── provisioner/        ← kubeadm, SSH, CNI bootstrap
│   ├── tenant/             ← namespace, RBAC, quota, network policy
│   ├── bao/                ← OpenBao deploy, unseal, policies
│   ├── harbor/             ← registry, projects, robot accounts
│   ├── telemetry/          ← Prometheus, Loki, Tempo, Grafana
│   ├── security/           ← Falco, Kyverno
│   ├── storage/            ← Longhorn, MinIO
│   ├── ingress/            ← Ingress-NGINX, Cert-Manager, External-DNS
│   └── k8sclient/          ← client-go / controller-runtime wrapper
├── configs/templates/      ← YAML/HCL templates
├── dashboard/              ← customer web portal (phase 7)
├── docs/
├── Makefile
├── go.mod
└── README.md
```
 
---
 
## Phase 1 — Core Infrastructure 🔴
 
### Go Project Scaffold
- [ ] Initialise Go module (`github.com/<handle>/bifrost`)
- [ ] Set up Cobra CLI with root command
- [ ] Add `bifrost init`, `bifrost tenant`, `bifrost status` command stubs
- [ ] Set up internal package structure
- [ ] Write Makefile (build, lint, test targets)
### Provisioner (kubeadm + SSH)
- [ ] SSH client wrapper in Go (`golang.org/x/crypto/ssh`)
- [ ] `bifrost init` — SSH into control plane node, run `kubeadm init`
- [ ] `bifrost init` — SSH into worker nodes, run `kubeadm join`
- [ ] Pull kubeconfig back to local machine after init
- [ ] Store kubeconfig securely for subsequent `client-go` calls
- [ ] Handle node drain and reset (`bifrost node remove`)
### Cilium + Hubble
- [ ] Deploy Cilium via Helm after kubeadm init
- [ ] Enable Hubble relay + UI
- [ ] Verify CNI is healthy before proceeding
- [ ] Write default-deny `NetworkPolicy` template
- [ ] Write per-tenant allow rules template (intra-namespace, DNS, OpenBao egress)
- [ ] Write `CiliumNetworkPolicy` template for L7 rules
- [ ] Apply network policies via `client-go` on tenant creation
### OpenBao
- [ ] Deploy OpenBao into `kaas-system` namespace via Helm
- [ ] Set up dedicated root OpenBao instance for transit auto-unseal
- [ ] Configure transit secret engine on root Bao
- [ ] Configure cluster Bao to auto-unseal via root Bao transit key
- [ ] Enable Kubernetes auth method on cluster Bao
- [ ] Write Go package wrapping OpenBao SDK (`internal/bao`)
- [ ] `bifrost init` provisions Bao on first run
- [ ] Per-tenant secret path (`secret/customers/<name>/`)
- [ ] Per-tenant Bao policy (scoped to their path only)
- [ ] Per-tenant Kubernetes auth role (scoped to tenant namespace SA)
### Longhorn
- [ ] Deploy Longhorn into `kaas-storage` namespace via Helm
- [ ] Set as default StorageClass
- [ ] Verify volumes are healthy before proceeding
- [ ] Configure replica count (2 across worker nodes)
---
 
## Phase 2 — Platform Layer 🔴
 
### Harbor
- [ ] Deploy Harbor into `kaas-harbor` namespace via Helm
- [ ] Configure Longhorn as persistent storage backend
- [ ] Enable Trivy vulnerability scanning (block on critical CVEs)
- [ ] Write Go package wrapping Harbor API (`internal/harbor`)
- [ ] On `bifrost tenant add`: create Harbor project for tenant
- [ ] On `bifrost tenant add`: create robot account scoped to tenant project
- [ ] Store robot account credentials in OpenBao (`secret/customers/<name>/harbor`)
- [ ] Create `imagePullSecret` in tenant namespace sourced from OpenBao
- [ ] On `bifrost tenant remove`: delete Harbor project + robot account
- [ ] Configure proxy cache for Docker Hub / ghcr.io
- [ ] Configure image retention policies
### Cert-Manager
- [ ] Deploy Cert-Manager into `kaas-ingress` namespace via Helm
- [ ] Configure Let's Encrypt `ClusterIssuer` (staging + production)
- [ ] Verify webhook is healthy
- [ ] Auto-provision TLS cert on tenant `Ingress` creation
- [ ] Add cert expiry to `bifrost status` output
### Ingress-NGINX
- [ ] Deploy Ingress-NGINX into `kaas-ingress` namespace via Helm
- [ ] Configure as default IngressClass
- [ ] Verify LoadBalancer / NodePort is reachable externally
- [ ] On `bifrost tenant add`: create tenant `Ingress` resource with TLS annotation
- [ ] Ensure Cilium NetworkPolicy allows ingress from NGINX controller only
### External-DNS
- [ ] Deploy External-DNS into `kaas-ingress` namespace via Helm
- [ ] Configure DNS provider credentials (Cloudflare / Route53) in OpenBao
- [ ] Verify DNS records are created on `Ingress` creation
- [ ] On `bifrost tenant remove`: clean up DNS records
---
 
## Phase 3 — Security Layer 🟠
 
### Falco
- [ ] Deploy Falco into `kaas-security` namespace via Helm
- [ ] Enable eBPF driver (works with Cilium)
- [ ] Configure alert output to stdout + webhook
- [ ] Write custom rules for tenant namespace events
  - [ ] Shell spawned inside container
  - [ ] Write to sensitive paths (e.g. `/etc`, `/usr`)
  - [ ] Unexpected outbound connection
  - [ ] Privilege escalation attempt
- [ ] Route Falco alerts into Grafana via Loki (phase 4)
- [ ] Add Falco alert summary to `bifrost status`
### Kyverno
- [ ] Deploy Kyverno into `kaas-security` namespace via Helm
- [ ] Write and apply cluster-wide policies:
  - [ ] Block privileged containers
  - [ ] Block `hostNetwork`, `hostPID`, `hostIPC`
  - [ ] Require all images to come from kaas-harbor registry
  - [ ] Require resource limits on all containers
  - [ ] Require pod labels (tenant, app)
  - [ ] Block `latest` image tag
- [ ] Write per-tenant policies applied on tenant creation
- [ ] Add Kyverno policy violation reporting to `bifrost status`
---
 
## Phase 4 — Observability 🟠
 
### Prometheus
- [ ] Deploy Prometheus into `kaas-monitoring` namespace via Helm (kube-prometheus-stack)
- [ ] Configure Hubble as scrape target
- [ ] Configure Longhorn as scrape target
- [ ] Configure Harbor as scrape target
- [ ] Configure per-tenant scrape via namespace label selector
- [ ] Set retention policy
### Loki
- [ ] Deploy Loki into `kaas-monitoring` namespace via Helm
- [ ] Configure Promtail as log collector (scrapes all pod logs)
- [ ] Route Falco alerts into Loki
- [ ] Configure per-tenant log isolation via namespace label
- [ ] Add Loki as Grafana datasource
### Tempo
- [ ] Deploy Tempo into `kaas-monitoring` namespace via Helm
- [ ] Configure OpenTelemetry Collector as trace receiver
- [ ] Configure per-tenant trace isolation
- [ ] Add Tempo as Grafana datasource
### Grafana
- [ ] Deploy Grafana into `kaas-monitoring` namespace via Helm
- [ ] Configure Prometheus, Loki, Tempo as datasources
- [ ] Add Hubble dashboards
- [ ] Add Longhorn dashboards
- [ ] Add per-tenant dashboard (scoped by namespace label)
- [ ] Add Harbor vulnerability scan dashboard
- [ ] Add Falco alert dashboard
- [ ] Embed Grafana views in customer dashboard (phase 7)
---
 
## Phase 5 — Storage Extension 🟡
 
### MinIO
- [ ] Deploy MinIO into `kaas-storage` namespace via Helm
- [ ] Configure Longhorn as persistent storage backend
- [ ] Write Go package wrapping MinIO SDK (`internal/storage`)
- [ ] On `bifrost tenant add`: create MinIO bucket for tenant
- [ ] Store MinIO credentials in OpenBao (`secret/customers/<name>/minio`)
- [ ] Expose S3-compatible endpoint to tenant namespace
- [ ] Configure Harbor to use MinIO as image blob storage backend
- [ ] On `bifrost tenant remove`: clean up bucket + credentials
---
 
## Phase 6 — Developer Experience 🟢
 
### ArgoCD
- [ ] Deploy ArgoCD into `kaas-gitops` namespace via Helm
- [ ] Configure SSO / admin credentials in OpenBao
- [ ] On `bifrost tenant add`: create ArgoCD `AppProject` scoped to tenant namespace
- [ ] On `bifrost tenant add`: create tenant ArgoCD RBAC role
- [ ] On `bifrost tenant remove`: delete AppProject + RBAC
- [ ] Expose ArgoCD UI link in customer dashboard
### KEDA
- [ ] Deploy KEDA into `kaas-system` namespace via Helm
- [ ] Configure default `ScaledObject` template for tenant workloads
- [ ] Support scaling triggers: HTTP, cron, queue depth
- [ ] Document how tenants define their own `ScaledObject` resources
---
 
## Phase 7 — Customer Dashboard 🟢
 
### Backend (Go)
- [ ] Scaffold Go HTTP server (`chi` or `gin`)
- [ ] Auth middleware (JWT, session management)
- [ ] Tenant-scoped API endpoints:
  - [ ] `GET /api/tenant/:id/status` — namespace health, quota usage
  - [ ] `GET /api/tenant/:id/pods` — running workloads
  - [ ] `GET /api/tenant/:id/flows` — Hubble network flows
  - [ ] `GET /api/tenant/:id/images` — Harbor project images + scan results
  - [ ] `GET /api/tenant/:id/secrets` — OpenBao secret keys (not values)
  - [ ] `GET /api/tenant/:id/alerts` — Falco alerts
  - [ ] `GET /api/tenant/:id/metrics` — Prometheus query proxy
### Frontend
- [ ] Scaffold React app
- [ ] Login page
- [ ] Overview dashboard (quota, pod count, health)
- [ ] Network flows view (Hubble)
- [ ] Image registry view (Harbor)
- [ ] Secrets manager view (OpenBao)
- [ ] Alerts view (Falco)
- [ ] Grafana embed (metrics, logs, traces)
- [ ] ArgoCD link (phase 6)
---
 
## Ongoing / Cross-Cutting
 
- [ ] Write `bifrost tenant add <name>` full happy path (phases 1-4)
- [ ] Write `bifrost tenant remove <name>` full teardown
- [ ] Write `bifrost status` with all component health checks
- [ ] Write `bifrost upgrade` for component version bumping
- [ ] Unit tests for all internal packages
- [ ] Integration tests against a local `kind` cluster
- [ ] Write `docs/architecture.md`
- [ ] Write `docs/getting-started.md`
- [ ] Write `docs/tenant-guide.md`
- [ ] CI pipeline (GitHub Actions — lint, test, build)
- [ ] Versioned releases with `goreleaser`
