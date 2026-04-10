# kluster-compare

Compare Kubernetes resources between two clusters. Interactive TUI or plain CLI output.

Built to debug drift during Pulumi-to-ArgoCD migrations, but works for any cluster-to-cluster comparison.

## Install

### From source (Go)

```bash
go install github.com/ramsrib/kluster-compare@latest
```

### Clone and build

```bash
git clone https://github.com/ramsrib/kluster-compare.git
cd kluster-compare
make install
```

## Usage

```bash
# Interactive TUI (default)
kluster-compare ctx-a ctx-b

# Non-interactive summary
kluster-compare ctx-a ctx-b -s

# Specific resource diff (kubectl-style)
kluster-compare ctx-a ctx-b deployment/api-deployment -n app-namespace
kluster-compare ctx-a ctx-b namespace/kube-system

# Diff resources with different names on each cluster (left:right)
kluster-compare ctx-a ctx-b hpa/daily-proxy-hpa-a75e5255:daily-proxy-hpa -n app-namespace

# Filter by types or namespaces
kluster-compare ctx-a ctx-b -t deployments,services -n app-namespace

# List available contexts
kluster-compare contexts

# List supported resource types
kluster-compare types
```

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--namespace` | `-n` | Namespace to filter or target |
| `--types` | `-t` | Resource types to compare (comma-separated) |
| `--summary` | `-s` | Print summary table instead of launching TUI |
| `--discover` | | Auto-discover all resource types including CRDs |
| `--kubeconfig` | | Path to kubeconfig (default: `~/.kube/config`) |

## TUI

### Navigation

| Key | Action |
|-----|--------|
| `j/k` or arrows | Navigate / scroll |
| `enter` | Drill into resource type or view diff |
| `esc` | Go back |
| `q` | Quit |

### Resource type list

| Key | Action |
|-----|--------|
| `/` | Filter by name |
| `e` | Toggle showing empty resource types |
| `d` | Discover CRDs and custom resource types |

### Resource list

| Key | Action |
|-----|--------|
| `/` | Filter by name |

### Diff view

Side-by-side diff with color-coded changes. Left column shows the first cluster, right column shows the second.

## Resource types

By default, kluster-compare loads 19 built-in K8s resource types for fast startup:

**Core:** Namespaces, Deployments, Services, DaemonSets, StatefulSets, ConfigMaps, ServiceAccounts

**RBAC:** ClusterRoles, ClusterRoleBindings, Roles, RoleBindings

**Autoscaling:** HPA

**Policy:** PDB, PriorityClasses

**Networking:** Ingresses, NetworkPolicies, IngressClasses

**Batch:** Jobs, CronJobs

Press `d` in the TUI to auto-discover all additional resource types (CRDs) from both clusters. This finds everything installed on either cluster -- Karpenter, KEDA, cert-manager, Gateway API, Prometheus, ArgoCD, and any custom CRDs.

### Adding a static resource type

Add one entry to `internal/model/registry.go`:

```go
{Name: "MyResource", Group: "example.io", Version: "v1", Resource: "myresources", Namespaced: true},
```

No other code changes needed -- the dynamic client handles everything.

## How it works

- Connects to both clusters using kubeconfig contexts
- Fetches resources progressively in parallel via the Kubernetes dynamic client
- Matches resources by exact name first, then fuzzy-matches by stripping hex hash suffixes (e.g. `app-19e0eebb-backplane` and `app-23b5ac0a-backplane` are paired automatically)
- Normalizes resources by stripping runtime fields (status, uid, managedFields, etc.) and migration annotations (pulumi.com/\*, argocd.argoproj.io/\*)
- Computes unified diffs on the normalized YAML
- Presents results in an interactive TUI with side-by-side diff or prints to stdout
- For CLI diff mode, use `type/left:right` to compare resources with different names

## Local development

```bash
# Build
make build

# Run
./kluster-compare ctx-a ctx-b

# Run tests
make test

# Format code
make fmt

# Lint (requires golangci-lint)
make lint

# Install to $GOPATH/bin
make install

# Clean build artifacts
make clean
```

## Requirements

- Go 1.21+
- Access to two Kubernetes clusters via kubeconfig
