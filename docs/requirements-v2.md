# Distributed Cache Kubernetes Operator - V2

## Group Members
- Patrick Weiss

## 1. Project Overview

Building upon the distributed cache with Raft consensus developed in the previous semester, this project focuses on creating a Kubernetes operator to address the fundamental challenge of running stateful Raft consensus workloads on Kubernetes' stateless pod infrastructure.

The operator will automate the deployment, scaling, and lifecycle management of Raft-based distributed cache clusters in Kubernetes environments, bridging the gap between Raft's stateful requirements (persistent node identity, stable network addresses, durable storage) and Kubernetes' ephemeral, cattle-style pod management philosophy.

### 1.1 Problem Statement

**Raft requires statefulness:**
- Persistent node identities that survive restarts
- Stable network addresses for peer communication
- Durable storage for Raft logs and snapshots
- Ordered startup/shutdown for cluster membership
- Predictable DNS names for cluster formation

**Kubernetes pods are stateless by default:**
- Pods can be rescheduled to different nodes
- Pod IPs change across restarts
- Default storage is ephemeral
- Pods can be killed and recreated arbitrarily
- No guaranteed ordering in pod lifecycle

This impedance mismatch makes running Raft on Kubernetes challenging without proper orchestration.

### 1.2 Vision

To create a production-grade Kubernetes operator that makes deploying and operating Raft-based distributed caches as simple as deploying stateless applications, while preserving Raft's consistency guarantees and operational characteristics.

### 1.3 Project Goals

- **Cloud-Native Integration**: Seamlessly integrate Raft consensus with Kubernetes primitives (StatefulSets, PersistentVolumes, Services) to maintain state across pod lifecycles.
- **Operational Automation**: Automate complex operational tasks (cluster formation, scaling, failover, upgrades) through declarative Kubernetes APIs.
- **Production Readiness**: Build an operator that handles edge cases, implements best practices, and provides observability for production Kubernetes environments.

## 2. Functional Requirements

### 2.1 Custom Resource Definition (CRD)

**FR-1.1** The operator SHALL define a `DistributedCache` custom resource with fields:
- `spec.replicas`: Number of Raft nodes (must be odd number, 3-7)
- `spec.version`: Docker image version/tag
- `spec.resources`: CPU/memory resource requests and limits
- `spec.storage.size`: Persistent volume size per node
- `spec.storage.class`: Storage class for PersistentVolumeClaims
- `spec.config`: Cache configuration (eviction policies, TTL defaults, etc.)

**FR-1.2** The CRD SHALL expose cluster status in `status` field:
- Current cluster state (Initializing, Ready, Degraded, Failed)
- Leader node identity
- Ready replica count vs desired count
- Node health status per replica
- Last successful reconciliation timestamp

### 2.2 Operator Core Functionality

**FR-2.1** The operator SHALL implement a reconciliation loop that:
- Watches for `DistributedCache` resource changes
- Creates/updates Kubernetes resources (StatefulSet, Services, ConfigMaps)
- Monitors cluster health and updates status
- Handles resource deletion with proper cleanup

**FR-2.2** The operator SHALL use StatefulSets to ensure:
- Stable pod identities (node-0, node-1, node-2, ...)
- Stable network identifiers via headless service
- Ordered deployment and scaling operations
- Persistent volume claims bound to specific pods

**FR-2.3** The operator SHALL create Kubernetes Services:
- Headless service for internal Raft peer communication
- Load-balanced service for external client access
- Leader-only service routing requests to current Raft leader

### 2.3 Cluster Lifecycle Management

**FR-3.1** The operator SHALL handle cluster initialization:
- Bootstrap first node as single-node cluster
- Join subsequent nodes to existing cluster via join API
- Wait for nodes to become healthy before proceeding
- Configure proper Raft bind addresses using pod DNS names

**FR-3.2** The operator SHALL support horizontal scaling:
- Scale up: Add new nodes and join them to cluster
- Scale down: Safely remove nodes and rebalance data
- Validate scaling operations (reject even numbers, enforce max 7 nodes)
- Prevent scaling below quorum threshold

**FR-3.3** The operator SHALL handle pod failures:
- Detect failed pods through readiness/liveness probes
- Allow Kubernetes to reschedule failed pods
- Rejoin recovered nodes to existing cluster
- Restore state from persistent volumes

**FR-3.4** The operator SHALL support version upgrades:
- Rolling update strategy (one node at a time)
- Verify each node is healthy before proceeding
- Rollback on failure
- Preserve data through PersistentVolumes

### 2.4 Storage Management

**FR-4.1** The operator SHALL provision persistent storage:
- Create PersistentVolumeClaim for each pod
- Mount PVC to consistent path for Raft data
- Retain PVCs during pod restarts
- Handle PVC expansion if storage class supports it

**FR-4.2** The operator SHALL configure volume mounts:
- Raft log directory: `/data/raft`
- Snapshot directory: `/data/snapshots`
- Application data: `/data/cache`

### 2.5 Configuration Management

**FR-5.1** The operator SHALL generate ConfigMaps containing:
- Raft configuration (election timeout, heartbeat interval)
- Cache configuration (eviction policies, max size)
- Peer discovery information (cluster members)
- Logging configuration

**FR-5.2** The operator SHALL support configuration updates:
- Detect ConfigMap changes in reconciliation loop
- Trigger rolling restart if needed for configuration changes
- Support hot-reload for non-critical config parameters

### 2.6 Observability Integration

**FR-6.1** The operator SHALL expose metrics:
- Operator-level metrics (reconciliation count, errors, duration)
- Cluster-level metrics (leader changes, node health)
- Integration with Prometheus via ServiceMonitor CRD

**FR-6.2** The operator SHALL emit Kubernetes events:
- Cluster creation/deletion
- Scaling operations
- Leader election changes
- Configuration updates
- Error conditions

**FR-6.3** The operator SHALL implement health checks:
- Liveness probe: Process health check endpoint
- Readiness probe: Cluster membership and Raft state
- Startup probe: Initial cluster formation

## 3. Non-Functional Requirements

### 3.1 Reliability Requirements

**NFR-1.1** **Reconciliation Safety**: The operator SHALL be idempotent - multiple reconciliation attempts SHALL produce the same result.

**NFR-1.2** **State Persistence**: The operator SHALL preserve all committed Raft state across pod restarts and rescheduling.

**NFR-1.3** **Leader Awareness**: The operator SHALL detect Raft leader changes within 10 seconds and update routing accordingly.

**NFR-1.4** **Quorum Protection**: The operator SHALL prevent operations that would break quorum (e.g., scaling below majority).

### 3.2 Performance Requirements

**NFR-2.1** **Reconciliation Speed**: The operator SHALL complete reconciliation loops in under 5 seconds under normal conditions.

**NFR-2.2** **Scaling Time**: The operator SHALL complete scale-up operations (adding one node) in under 60 seconds.

**NFR-2.3** **Recovery Time**: Failed pods SHALL rejoin the cluster within 30 seconds of becoming ready.

### 3.3 Operational Requirements

**NFR-3.1** **Kubernetes Compatibility**: The operator SHALL support Kubernetes versions 1.27+.

**NFR-3.2** **Resource Efficiency**: The operator controller SHALL use less than 100MB memory and 0.1 CPU cores.

**NFR-3.3** **RBAC**: The operator SHALL follow principle of least privilege with minimal required permissions:
- Watch/List/Get: `DistributedCache` CRDs
- Create/Update/Delete: StatefulSets, Services, ConfigMaps, PVCs
- Update: `DistributedCache` status subresource

**NFR-3.4** **Namespace Isolation**: The operator SHALL support deployment in:
- Single namespace mode (watches one namespace)
- Cluster-wide mode (watches all namespaces)

### 3.4 Deployment Requirements

**NFR-4.1** **Installation**: The operator SHALL be installable via:
- Helm chart
- kubectl apply -f manifests/
- Operator Lifecycle Manager (OLM) bundle

**NFR-4.2** **Configuration**: The operator SHALL support configuration through:
- Environment variables
- Command-line flags
- ConfigMap for operator-level settings

### 3.5 Testing Requirements

**NFR-5.1** **Unit Testing**: The operator SHALL have 80%+ code coverage for reconciliation logic.

**NFR-5.2** **Integration Testing**: The operator SHALL include tests using:
- Kubernetes envtest for local testing
- Kind/k3s for full integration tests

**NFR-5.3** **Chaos Testing**: The operator SHALL demonstrate resilience through:
- Random pod deletion during operations
- Network partition simulation
- Persistent volume failure scenarios

### 3.6 Documentation Requirements

**NFR-6.1** **User Documentation**: The operator SHALL provide:
- Installation guide
- Quick start tutorial
- CRD API reference
- Operational runbooks (scaling, upgrading, troubleshooting)

**NFR-6.2** **Developer Documentation**: The operator SHALL document:
- Architecture and design decisions
- Development setup instructions
- Testing procedures
- Contributing guidelines

## 4. Technical Approach

### 4.1 Technology Stack

- **Language**: Go 1.22+
- **Framework**: Kubebuilder or Operator SDK
- **Client Library**: controller-runtime, client-go

### 4.2 Kubernetes Resources Created

For each `DistributedCache` CR, the operator creates:

```
DistributedCache "my-cache"
├── StatefulSet "my-cache"
│   └── Pods: my-cache-0, my-cache-1, my-cache-2
├── Service "my-cache" (ClusterIP, load-balanced)
├── Service "my-cache-headless" (headless, for peer discovery)
├── Service "my-cache-leader" (routes to current leader)
├── ConfigMap "my-cache-config"
├── PersistentVolumeClaim "data-my-cache-0"
├── PersistentVolumeClaim "data-my-cache-1"
└── PersistentVolumeClaim "data-my-cache-2"
```

### 4.3 Addressing Stateful Challenges

| Challenge | Solution |
|-----------|----------|
| **Persistent Identity** | StatefulSet provides stable pod names (node-0, node-1, ...) |
| **Stable Networking** | Headless service provides stable DNS names: `<pod>.<service>.<namespace>.svc.cluster.local` |
| **Durable Storage** | PersistentVolumeClaims bound to specific pods via volumeClaimTemplates |
| **Ordered Operations** | StatefulSet rolling update strategy with pod readiness checks |
| **Cluster Formation** | Init container or startup logic that discovers peers via DNS |
| **Leader Routing** | Service selector targeting pod with leader label |

## 5. Requirements Prioritization (MoSCoW Method)

### 5.1 MUST Have

| Requirement | Description | Justification |
|------------|-------------|---------------|
| **FR-1.1** | Basic DistributedCache CRD (replicas, version, resources, storage) | Core operator functionality - defines desired state |
| **FR-2.1** | Reconciliation loop with resource creation | Fundamental operator pattern implementation |
| **FR-2.2** | StatefulSet management | Solves core stateful problem - critical architectural choice |
| **FR-2.3** | Basic Services (headless + client) | Required for pod communication and external access |
| **FR-3.1** | Cluster initialization | Must bootstrap cluster before any operations |
| **FR-4.1** | Persistent volume provisioning | Core requirement for state preservation |
| **FR-6.3** | Health checks (readiness/liveness) | Essential for Kubernetes to manage pod lifecycle |
| **NFR-1.2** | State persistence across restarts | Validates solution to core problem |
| **NFR-3.1** | Kubernetes 1.27+ compatibility | Baseline compatibility requirement |
| **NFR-6.1** | Basic user documentation | Necessary for project evaluation |

### 5.2 SHOULD Have

| Requirement | Description | Justification |
|------------|-------------|---------------|
| **FR-1.2** | Status field with cluster state | Provides observability into operator actions |
| **FR-3.2** | Horizontal scaling (up/down) | Demonstrates dynamic cluster management |
| **FR-3.3** | Pod failure handling | Validates resilience of stateful solution |
| **FR-5.1** | ConfigMap generation | Proper configuration management pattern |
| **FR-6.1** | Metrics exposure | Production-ready observability |
| **FR-6.2** | Kubernetes events | Standard operator communication pattern |
| **NFR-1.1** | Idempotent reconciliation | Correctness requirement for operators |
| **NFR-1.4** | Quorum protection | Safety mechanism for Raft clusters |
| **NFR-3.3** | RBAC with least privilege | Security best practice |
| **NFR-5.1** | Unit test coverage 80%+ | Quality assurance |

### 5.3 COULD Have

| Requirement | Description | Justification |
|------------|-------------|---------------|
| **FR-2.3** | Leader-only service | Enhanced routing capability |
| **FR-3.4** | Version upgrades with rolling updates | Production operational requirement |
| **FR-4.2** | PVC expansion support | Advanced storage management |
| **FR-5.2** | Configuration hot-reload | Improved operational experience |
| **NFR-1.3** | Leader detection within 10s | Performance optimization |
| **NFR-2.2** | Scale-up in under 60s | Performance target |
| **NFR-3.2** | Resource efficiency targets | Optimization goal |
| **NFR-3.4** | Multi-namespace support | Deployment flexibility |
| **NFR-4.1** | Multiple installation methods (Helm, OLM) | Distribution convenience |
| **NFR-5.2** | Integration tests with Kind/k3s | Comprehensive testing |
| **NFR-5.3** | Chaos testing scenarios | Resilience validation |

### 5.4 WON'T Have

| Requirement | Description | Justification |
|------------|-------------|---------------|
| - | Multi-cluster replication | Geographic distribution beyond scope |
| - | Automatic backup/restore to S3 | Complex disaster recovery feature |
| - | Custom scheduler integration | Advanced Kubernetes feature |
| - | Network policy generation | Security beyond core functionality |
| - | Cost optimization (spot instances) | Cloud-specific feature |
| - | GitOps integration (ArgoCD/Flux) | Deployment pattern beyond scope |
| - | Multi-tenancy with resource quotas | Enterprise feature |
| - | Custom metrics for HPA | Advanced autoscaling beyond scope |
| - | Service mesh integration | Additional infrastructure complexity |
| - | Admission webhooks (validation/mutation) | Advanced operator feature |

### 5.5 Prioritization Rationale

**MUST Have Rationale:**
- Focuses on solving the core stateful-on-stateless problem
- Implements basic operator pattern correctly
- Demonstrates Kubernetes integration fundamentals
- Achievable within semester timeline
- Validates architectural approach

**SHOULD Have Rationale:**
- Adds production-readiness and operational maturity
- Demonstrates understanding of operator best practices
- Provides observability and safety mechanisms
- Realistic with focused effort throughout semester

**COULD Have Rationale:**
- Represents advanced features that enhance operator capabilities
- Stretch goals if core requirements completed early
- Demonstrates deeper Kubernetes expertise
- Not critical for validating core thesis

**WON'T Have Rationale:**
- Features requiring significant additional infrastructure
- Enterprise-level capabilities beyond academic scope
- Cloud-specific features that limit portability
- Advanced Kubernetes features that distract from core learning objectives

## 6. Success Criteria

The project will be considered successful if:

1. **Core Problem Solved**: A Raft cluster can be deployed on Kubernetes with full state preservation across pod restarts, rescheduling, and node failures.

2. **Declarative Management**: Users can create, scale, and delete distributed cache clusters using `kubectl apply` with a simple YAML manifest.

3. **Resilience Demonstrated**: The cluster maintains consensus and data integrity through simulated failures (pod deletion, node drain).

4. **Operational Simplicity**: Common operations (scaling, upgrading) are automated and require no manual intervention.

5. **Production Patterns**: The operator follows Kubernetes best practices (RBAC, health checks, metrics, events).
