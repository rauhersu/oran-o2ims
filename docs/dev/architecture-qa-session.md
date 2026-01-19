# O2IMS Operator Architecture Q&A Session

> **Date:** January 12, 2026  
> **Context:** Complete onboarding session for a senior software engineer joining the O-RAN team

---

## Table of Contents

1. [Initial Onboarding Request](#1-initial-onboarding-request)
   - [O-RAN Interface Overview](#11-o-ran-interface-overview)
   - [O-RAN O2IMS Specification v11 Focus](#12-o-ran-o2ims-specification-v11-focus)
   - [Operator Architecture Analysis](#13-operator-architecture-analysis)
   - [Inventory API Deep Dive](#14-inventory-api-deep-dive)
   - [Developer Workflow](#15-developer-workflow)
2. [Plugin Architecture](#2-plugin-architecture)
3. [Plugin Architecture ↔ Inventory API Relationship](#3-plugin-architecture--inventory-api-relationship)
   - [Sequence Diagram: Plugin ↔ Inventory Data Flow](#sequence-diagram-plugin--inventory-data-flow)
4. [Kubernetes Pod Layout](#4-kubernetes-pod-layout)
5. [Pod Container Composition](#5-pod-container-composition)
6. [Dependencies Analysis](#6-dependencies-analysis)
7. [Operators vs API Servers - Reconcilers Analysis](#7-operators-vs-api-servers---reconcilers-analysis)
8. [Inventory API Request/Response Code Path](#8-inventory-api-requestresponse-code-path)
9. [HTTP Infrastructure Deep Dive](#9-http-infrastructure-deep-dive)
10. [Clarifying Questions: Inventory CR vs Inventory API vs Provisioning](#10-clarifying-questions-inventory-cr-vs-inventory-api-vs-provisioning)
11. [Provisioning Flow Deep Dive: Who Does What?](#11-provisioning-flow-deep-dive-who-does-what)
12. [Corrections and Refinements](#12-corrections-and-refinements-based-on-official-documentation-review) *(Added after official docs review)*

---

## 1. Initial Onboarding Request

### Question

> I am a senior software engineer who has recently joined to the o-ran team, a team of software designers who manage the current workspace (github project) and are responsible to implement the o-ran specification with this olm operator. I am starting with literally zero foundation in this project so I would like your help to:
> 
> 1. Gain quick context with the o-ran interface, purpose. Stress the most important concepts.
> 2. Focus on the o-ran interface specification (version 11) as I will join as a designer to implement that.
> 3. Analyse the current operator code, describing the architecture and dependencies: own CRD, other required CRDs, kubernetes resources (secrets, configmaps, ...) the operator depends on...
> 4. Due to the point number 2, now analyse the inventory API the operator has and its current implementation. Be exhaustive on this part, I want to know the current status and how folks designed it. My current tasks are specifically:
>    - "Add support for new endpoints in O2ims_InfrastructureInventory Service API"
>    - "Update ResourcePoolInfo"
>    - "Update CloudInfo"
>    - "Update ResourceType"
>    - "Update O2ims_InfrastructureInventory Service version to 2.0.0"
>    - "Update Provisioning"
>    
>    So I want you to analyse the code now focusing on these new tasks to perform.
> 5. Suggest a workflow valid for a developer with this operator overall: when introducing new changes, I want to learn how to deploy it, undeploy it, configure other resources I might need. The root Makefile can give you useful hints for that.
> 
> Explain all those points in a step by step fashion, with concise but detailed steps.

---

### Answer

## 1.1 O-RAN Interface Overview

### What is O-RAN?

**O-RAN (Open Radio Access Network)** is an industry initiative to create open, intelligent, virtualized and interoperable RAN (Radio Access Network) systems. The key innovation is **disaggregation** - breaking the traditional monolithic RAN into modular components with standardized interfaces.

### O2 Interface Purpose

The **O2 interface** connects the **Service Management and Orchestration (SMO)** framework to the **O-Cloud** platform. Think of it as:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        SMO (Service Management & Orchestration)              │
│         ┌─────────────────────────────────────────────────────────┐         │
│         │  • Manages network functions lifecycle                   │         │
│         │  • Orchestrates deployments                              │         │
│         │  • Monitors infrastructure health                        │         │
│         └─────────────────────────────────────────────────────────┘         │
└────────────────────────────────────┬────────────────────────────────────────┘
                                     │
                                     │  O2 Interface (This Project!)
                                     │  ├─ O2ims (Infrastructure Management)
                                     │  └─ O2dms (Deployment Management)
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              O-Cloud Platform                                │
│         ┌─────────────────────────────────────────────────────────┐         │
│         │  • Physical/Virtual infrastructure                       │         │
│         │  • OpenShift/Kubernetes clusters                         │         │
│         │  • Hardware resources (nodes, NICs, accelerators)        │         │
│         └─────────────────────────────────────────────────────────┘         │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Key O2IMS Concepts (Most Important!)

| Concept | Description | Your Focus Level |
|---------|-------------|------------------|
| **O-Cloud** | The cloud platform managed by O2IMS (OpenShift + RHACM) | 🔴 High |
| **Resource Pool** | Logical grouping of resources (e.g., a cluster site) | 🔴 High |
| **Resource Type** | Classification of resources (Node, NIC, Accelerator) | 🔴 High |
| **Resource** | Individual infrastructure item (a specific node, NIC) | 🔴 High |
| **Deployment Manager** | Kubernetes cluster that deploys workloads | 🔴 High |
| **Subscription** | SMO subscribes to inventory change notifications | 🟡 Medium |
| **Alarm Dictionary** | Defines alarms that resources can raise | 🟡 Medium |

### O2IMS Service APIs

```
O2IMS Interface
├── Infrastructure Inventory Service (/o2ims-infrastructureInventory/)
│   ├── /v1 (CloudInfo)
│   ├── /v1/resourcePools
│   ├── /v1/resourcePools/{id}/resources
│   ├── /v1/resourceTypes
│   ├── /v1/deploymentManagers
│   ├── /v1/subscriptions
│   └── /v1/alarmDictionaries
│
├── Infrastructure Monitoring Service (/o2ims-infrastructureMonitoring/)
│   ├── /v1/alarms
│   ├── /v1/alarmSubscriptions
│   └── /v1/alarmProbableCauses
│
└── Infrastructure Provisioning Service (/o2ims-infrastructureProvisioning/)
    └── /v1/provisioningRequests
```

---

## 1.2 O-RAN O2IMS Specification v11 Focus

### Specification Reference

The O-RAN O2IMS specification is defined in:
- **O-RAN.WG6.O2IMS-INTERFACE-R003-v07.00** (Infrastructure Management Services)
- Current implementation targets **v7.0** with updates toward **v11**

### Key Changes You'll Be Implementing

Based on your task list, here are the spec areas to focus on:

| Task | Spec Section | Key Changes |
|------|--------------|-------------|
| **Update CloudInfo** | 3.2.6.1 OCloudInfo | New fields, extensions structure |
| **Update ResourcePoolInfo** | 3.2.6.4 ResourcePoolInfo | Location details, capacity info |
| **Update ResourceType** | 3.2.6.3 ResourceTypeInfo | resourceClass, resourceKind enums |
| **New Endpoints** | 3.2.5 | Additional query parameters, pagination |
| **Version 2.0.0** | Throughout | API versioning, backward compatibility |

### Current OpenAPI Spec Location

```
internal/service/resources/api/openapi.yaml  ← Main Inventory API spec
internal/service/provisioning/api/openapi.yaml  ← Provisioning API spec
internal/service/alarms/api/openapi.yaml  ← Alarms/Monitoring API spec
```

### Version Update Strategy

```yaml
# Current (internal/service/resources/utils/constants.go)
const CurrentInventoryVersion = "1.0.0"

# Your task: Update to
const CurrentInventoryVersion = "2.0.0"
```

---

## 1.3 Operator Architecture Analysis

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              O2IMS OPERATOR ARCHITECTURE                         │
└─────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────┐
│                                 CRD Layer                                        │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│   OWNED CRDs (Defined by this operator):                                         │
│   ┌────────────────────┐ ┌────────────────────┐ ┌────────────────────────────┐  │
│   │     Inventory      │ │  ClusterTemplate   │ │   ProvisioningRequest      │  │
│   │ ocloud.openshift.io│ │ clcm.openshift.io  │ │   clcm.openshift.io        │  │
│   └────────────────────┘ └────────────────────┘ └────────────────────────────┘  │
│                                                                                  │
│   ┌────────────────────┐ ┌────────────────────┐ ┌────────────────────────────┐  │
│   │  HardwareTemplate  │ │   HardwarePlugin   │ │    HardwareProfile         │  │
│   │ clcm.openshift.io  │ │ clcm.openshift.io  │ │   clcm.openshift.io        │  │
│   └────────────────────┘ └────────────────────┘ └────────────────────────────┘  │
│                                                                                  │
│   ┌────────────────────┐ ┌────────────────────┐                                 │
│   │   AllocatedNode    │ │ NodeAllocation     │  (plugins.clcm.openshift.io)   │
│   │                    │ │ Request            │                                 │
│   └────────────────────┘ └────────────────────┘                                 │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────┐
│                              Controller Layer                                    │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│   ┌─────────────────────────────────────────────────────────────────────────┐   │
│   │                     controller-manager (main operator)                   │   │
│   │  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────────────┐    │   │
│   │  │ Inventory       │ │ ClusterTemplate │ │ ProvisioningRequest     │    │   │
│   │  │ Reconciler      │ │ Reconciler      │ │ Reconciler              │    │   │
│   │  └─────────────────┘ └─────────────────┘ └─────────────────────────┘    │   │
│   └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────┐
│                               Service Layer                                      │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│   REST API Servers (Deployed by Inventory Reconciler):                           │
│   ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐                   │
│   │ resource-server │ │ cluster-server  │ │  alarms-server  │                   │
│   │ (Inventory API) │ │ (Dep. Managers) │ │ (Monitoring)    │                   │
│   └─────────────────┘ └─────────────────┘ └─────────────────┘                   │
│                                                                                  │
│   ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐                   │
│   │artifacts-server │ │provisioning-srv │ │  postgres-srv   │                   │
│   │ (Cluster files) │ │(Provisioning)   │ │  (Database)     │                   │
│   └─────────────────┘ └─────────────────┘ └─────────────────┘                   │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Owned CRDs (Defined in `api/` directory)

| CRD | API Group | Purpose |
|-----|-----------|---------|
| `Inventory` | `ocloud.openshift.io/v1alpha1` | Main CR - configures entire O2IMS deployment |
| `ClusterTemplate` | `clcm.openshift.io/v1alpha1` | Defines cluster deployment templates |
| `ProvisioningRequest` | `clcm.openshift.io/v1alpha1` | Requests cluster provisioning |
| `HardwareTemplate` | `clcm.openshift.io/v1alpha1` | Hardware requirements template |
| `HardwarePlugin` | `clcm.openshift.io/v1alpha1` | Hardware manager plugin registration |
| `HardwareProfile` | `clcm.openshift.io/v1alpha1` | Hardware profile definitions |
| `AllocatedNode` | `plugins.clcm.openshift.io/v1alpha1` | Allocated hardware node |
| `NodeAllocationRequest` | `plugins.clcm.openshift.io/v1alpha1` | Request for node allocation |

### External CRDs (Dependencies)

| CRD | Source | Purpose |
|-----|--------|---------|
| `ManagedCluster` | RHACM | Managed cluster representation |
| `Policy` | RHACM | Configuration policies |
| `ClusterInstance` | SiteConfig | Cluster deployment spec |
| `BareMetalHost` | Metal3 | Bare metal node management |
| `ClusterDeployment` | Hive | Cluster lifecycle |
| `ImageBasedGroupUpgrade` | IBGU | Image-based upgrades |

### Kubernetes Resources Created

```
Inventory CR creates:
├── Namespace: oran-o2ims
├── ServiceAccount: per-server
├── ClusterRole/ClusterRoleBinding: RBAC
├── ConfigMap: oran-o2ims-env-config
├── Secret: postgres-server-password, *-database-password
├── PersistentVolumeClaim: postgres-server-pvc
├── Service: per-server (ClusterIP)
├── Deployment: 8 deployments (see pod layout)
└── Ingress: oran-o2ims (routes to all servers)
```

---

## 1.4 Inventory API Deep Dive

### Current Implementation Status

| Endpoint | Status | File |
|----------|--------|------|
| `GET /v1` (CloudInfo) | ✅ Implemented | `server.go:GetCloudInfo` |
| `GET /v1/resourcePools` | ✅ Implemented | `server.go:GetResourcePools` |
| `GET /v1/resourcePools/{id}` | ✅ Implemented | `server.go:GetResourcePool` |
| `GET /v1/resourcePools/{id}/resources` | ✅ Implemented | `server.go:GetResources` |
| `GET /v1/resourcePools/{id}/resources/{id}` | ✅ Implemented | `server.go:GetResource` |
| `GET /v1/resourceTypes` | ✅ Implemented | `server.go:GetResourceTypes` |
| `GET /v1/resourceTypes/{id}` | ✅ Implemented | `server.go:GetResourceType` |
| `GET /v1/deploymentManagers` | ✅ Implemented | `server.go:GetDeploymentManagers` |
| `GET /v1/deploymentManagers/{id}` | ✅ Implemented | `server.go:GetDeploymentManager` |
| `GET /v1/subscriptions` | ✅ Implemented | `server.go:GetSubscriptions` |
| `POST /v1/subscriptions` | ✅ Implemented | `server.go:CreateSubscription` |
| `DELETE /v1/subscriptions/{id}` | ✅ Implemented | `server.go:DeleteSubscription` |
| `GET /v1/alarmDictionaries` | ✅ Implemented | `server.go:GetAlarmDictionaries` |

### Code Structure

```
internal/service/resources/
├── api/
│   ├── openapi.yaml              ← OpenAPI spec (SOURCE OF TRUTH)
│   ├── openapi-overlay.yaml      ← Spec modifications
│   ├── server.go                 ← Handler implementations (YOUR CODE)
│   ├── generated/
│   │   └── inventory.generated.go ← Auto-generated from OpenAPI
│   └── tools/
│       ├── generate.go           ← go:generate directive
│       └── oapi-codegen.yaml     ← Code generation config
├── db/
│   ├── models/
│   │   ├── resource_pool.go      ← DB model
│   │   ├── resource_type.go      ← DB model
│   │   ├── resource.go           ← DB model
│   │   └── converters.go         ← DB ↔ API model converters
│   ├── repo/
│   │   └── repository.go         ← Database queries
│   └── migrations/
│       └── *.sql                 ← SQL migrations
├── collector/
│   └── collector.go              ← Data collection from K8s/plugins
├── serve.go                      ← Server initialization
└── migration.go                  ← Migration runner
```

### Database Schema (PostgreSQL)

```sql
-- resource_pool table
CREATE TABLE resource_pool (
    resource_pool_id UUID PRIMARY KEY,
    global_location_id UUID NOT NULL,
    name VARCHAR NOT NULL,
    description VARCHAR,
    o_cloud_id UUID NOT NULL,
    location VARCHAR,
    extensions JSONB,
    data_source_id UUID NOT NULL,
    generation_id INTEGER NOT NULL,
    external_id VARCHAR NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- resource_type table
CREATE TABLE resource_type (
    resource_type_id UUID PRIMARY KEY,
    name VARCHAR NOT NULL,
    description VARCHAR,
    vendor VARCHAR,
    model VARCHAR,
    version VARCHAR,
    resource_kind VARCHAR,
    resource_class VARCHAR,
    extensions JSONB,
    data_source_id UUID NOT NULL,
    generation_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- resource table
CREATE TABLE resource (
    resource_id UUID PRIMARY KEY,
    resource_type_id UUID REFERENCES resource_type(resource_type_id),
    resource_pool_id UUID REFERENCES resource_pool(resource_pool_id),
    global_asset_id VARCHAR,
    description VARCHAR,
    extensions JSONB,
    -- ... additional fields
);
```

### Your Task Areas

#### 1. Update CloudInfo (`GetCloudInfo`)

**Current:**
```go
func (r *ResourceServer) GetCloudInfo(ctx context.Context, request api.GetCloudInfoRequestObject) (api.GetCloudInfoResponseObject, error) {
    options := commonapi.NewFieldOptions(request.Params.AllFields, request.Params.Fields, request.Params.ExcludeFields)
    result := r.Info  // Static OCloudInfo struct
    if options.IsIncluded(commonapi.ExtensionsAttribute) {
        extensions := make(map[string]interface{})
        result.Extensions = &extensions
    }
    return api.GetCloudInfo200JSONResponse(result), nil
}
```

**What to update:** Add new fields from spec v11 (e.g., `softwareVersion`, `capabilities`)

#### 2. Update ResourcePoolInfo

**Current DB Model:**
```go
type ResourcePool struct {
    ResourcePoolID   uuid.UUID              `db:"resource_pool_id"`
    GlobalLocationID uuid.UUID              `db:"global_location_id"`
    Name             string                 `db:"name"`
    Description      string                 `db:"description"`
    OCloudID         uuid.UUID              `db:"o_cloud_id"`
    Location         *string                `db:"location"`
    Extensions       map[string]interface{} `db:"extensions"`
    // ...
}
```

**What to update:** Add new fields like `capacity`, `usedCapacity`, etc.

#### 3. Update ResourceType

**Current Model:**
```go
type ResourceType struct {
    ResourceTypeID uuid.UUID `db:"resource_type_id"`
    Name           string    `db:"name"`
    Description    string    `db:"description"`
    Vendor         string    `db:"vendor"`
    Model          string    `db:"model"`
    Version        string    `db:"version"`
    ResourceKind   string    `db:"resource_kind"`   // PHYSICAL, LOGICAL, UNDEFINED
    ResourceClass  string    `db:"resource_class"`  // COMPUTE, NETWORKING, STORAGE, UNDEFINED
    // ...
}
```

**What to update:** Ensure enum values match spec, add new fields

#### 4. Version 2.0.0 Update

Files to modify:
- `internal/service/resources/utils/constants.go` - Version constant
- `internal/service/resources/api/openapi.yaml` - Info section
- API version endpoints logic

---

## 1.5 Developer Workflow

### Prerequisites

```bash
# Required tools (from Makefile)
go version              # Go 1.22+
kubectl version         # Kubernetes CLI
oc version              # OpenShift CLI
docker version          # Container runtime

# Tools installed via Makefile
make controller-gen     # Generates CRD manifests
make kustomize          # Kubernetes YAML tool
make opm               # Operator Package Manager
make operator-sdk      # Operator SDK
```

### Development Cycle

#### 1. Setup Local Environment

```bash
# Clone and setup
cd oran-o2ims.fork

# Install dependencies
make deps

# Generate code (CRDs, DeepCopy, OpenAPI)
make generate

# Build manifests
make manifests
```

#### 2. Make Code Changes

```bash
# Edit OpenAPI spec
vim internal/service/resources/api/openapi.yaml

# Regenerate Go code from OpenAPI
go generate ./internal/service/resources/api/tools/...

# Edit handler implementation
vim internal/service/resources/api/server.go

# Edit DB models if needed
vim internal/service/resources/db/models/resource_pool.go

# Add DB migrations
vim internal/service/resources/db/migrations/000X_your_migration.sql
```

#### 3. Build & Test Locally

```bash
# Run unit tests
make test

# Run linters
make lint

# Build binary
make build

# Build container image
make docker-build IMG=<your-registry>/oran-o2ims:dev
```

#### 4. Deploy to Cluster

```bash
# Option A: Deploy directly (for development)
make deploy IMG=<your-registry>/oran-o2ims:dev

# Option B: Run locally against cluster
make run

# Check deployment
kubectl get pods -n oran-o2ims
kubectl logs -n oran-o2ims deployment/controller-manager
```

#### 5. Create Test Resources

```bash
# Apply sample Inventory CR
kubectl apply -f config/samples/v1alpha1_inventory.yaml

# Watch resources being created
kubectl get all -n oran-o2ims -w
```

#### 6. Test API Endpoints

```bash
# Port-forward to resource server
kubectl port-forward -n oran-o2ims svc/resource-server 8000:8000

# Test endpoint
curl -k https://localhost:8000/o2ims-infrastructureInventory/v1/resourcePools
```

#### 7. Undeploy

```bash
# Remove operator and resources
make undeploy

# Clean up CRDs
make uninstall
```

### Key Makefile Targets

| Target | Description |
|--------|-------------|
| `make generate` | Generate DeepCopy, CRDs, OpenAPI code |
| `make manifests` | Generate RBAC, CRD manifests |
| `make build` | Build the binary |
| `make test` | Run unit tests |
| `make lint` | Run all linters |
| `make docker-build` | Build container image |
| `make deploy` | Deploy to cluster |
| `make undeploy` | Remove from cluster |
| `make bundle` | Generate OLM bundle |
| `make catalog` | Build operator catalog |

### Debugging Tips

```bash
# View operator logs
kubectl logs -n oran-o2ims -l app.kubernetes.io/name=oran-o2ims -f

# Check CRD status
kubectl describe inventory default

# Database access (port-forward)
kubectl port-forward -n oran-o2ims svc/postgres-server 5432:5432
psql -h localhost -U postgres -d resources

# API server logs
kubectl logs -n oran-o2ims deployment/resource-server -f
```

---

## 2. Plugin Architecture

### Question

> Getting back to the design (code), have you found any plugin architecture in the source code? Folks seem to use a 'plugin' jargon.

### Answer

Yes! There's a clear **Hardware Plugin Architecture** in the codebase. This is a critical part of the Cluster Lifecycle Management (CLCM) subsystem within the O2IMS operator.

### Purpose & Core Concepts

The plugin system abstracts **hardware provisioning** from different hardware managers. It allows the operator to:

- **Allocate and provision bare-metal nodes** for OpenShift clusters
- **Support multiple hardware backends** (currently **Metal3** is implemented)
- **Provide a unified API** for hardware operations regardless of the underlying infrastructure

### Plugin Architecture Components

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              PLUGIN ARCHITECTURE                                 │
└─────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────┐
│                           CRD Layer (api/)                                       │
├─────────────────────────────────────────────────────────────────────────────────┤
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────────────────┐   │
│  │  HardwarePlugin  │  │ NodeAllocation   │  │     AllocatedNode            │   │
│  │  (Registration)  │  │ Request (NAR)    │  │     (Result)                 │   │
│  └──────────────────┘  └──────────────────┘  └──────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                        Controller Layer (hwmgr-plugins/)                         │
├─────────────────────────────────────────────────────────────────────────────────┤
│  ┌──────────────────────────────────────────────────────────────────────────┐   │
│  │                    HardwarePluginReconciler                               │   │
│  │    - Watches HardwarePlugin CRs                                          │   │
│  │    - Validates plugin configuration                                       │   │
│  │    - Updates status with available resources                              │   │
│  └──────────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                     Plugin Implementations (hwmgr-plugins/metal3/)               │
├─────────────────────────────────────────────────────────────────────────────────┤
│  ┌──────────────────────────────────────────────────────────────────────────┐   │
│  │                         Metal3 Plugin                                     │   │
│  │  ┌─────────────────────┐  ┌─────────────────────┐  ┌──────────────────┐  │   │
│  │  │ NodeAllocation      │  │ AllocatedNode       │  │ HostFirmware     │  │   │
│  │  │ RequestReconciler   │  │ Reconciler          │  │ ComponentsRcnclr │  │   │
│  │  └─────────────────────┘  └─────────────────────┘  └──────────────────┘  │   │
│  │                                                                           │   │
│  │  ┌─────────────────────────────────────────────────────────────────────┐ │   │
│  │  │              REST API Server (Provisioning + Inventory)              │ │   │
│  │  └─────────────────────────────────────────────────────────────────────┘ │   │
│  └──────────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Key CRDs

| CRD | API Group | Purpose |
|-----|-----------|---------|
| `HardwarePlugin` | `clcm.openshift.io/v1alpha1` | Registers a hardware manager with the system |
| `NodeAllocationRequest` | `plugins.clcm.openshift.io/v1alpha1` | Requests node allocation from a hardware plugin |
| `AllocatedNode` | `plugins.clcm.openshift.io/v1alpha1` | Represents an allocated node from a hardware plugin |

### Plugin Registration Flow

```
1. HardwarePlugin CR created (defines plugin type, namespace, capabilities)
         │
         ▼
2. HardwarePluginReconciler validates and updates status
         │
         ▼
3. ProvisioningRequest needs hardware → creates NodeAllocationRequest
         │
         ▼
4. Plugin-specific reconciler (e.g., Metal3) processes NAR
         │
         ▼
5. AllocatedNode CR created with provisioned node details
         │
         ▼
6. ProvisioningRequest controller continues with cluster deployment
```

### Key Files

| File | Purpose |
|------|---------|
| `api/hardwaremanagement/v1alpha1/hardwareplugin_types.go` | HardwarePlugin CRD definition |
| `api/hardwaremanagement/plugins/v1alpha1/node_allocation_requests.go` | NAR CRD |
| `api/hardwaremanagement/plugins/v1alpha1/allocated_nodes.go` | AllocatedNode CRD |
| `hwmgr-plugins/controller/hardwareplugin_controller.go` | Generic plugin controller |
| `hwmgr-plugins/metal3/controller/` | Metal3-specific controllers |
| `hwmgr-plugins/metal3/server/` | Metal3 REST API server |

---

## 3. Plugin Architecture ↔ Inventory API Relationship

### Question

> Can this be related to my 'inventory' API task? Trying to find any relationship between this API task and the plugin architecture.

### Answer

**Excellent find!** Yes, there's a **direct and critical relationship** between the Hardware Plugin architecture and your Inventory API tasks.

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                                 SMO (Client)                                     │
└───────────────────────────────────────┬─────────────────────────────────────────┘
                                        │ HTTP/REST
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    O2IMS INVENTORY API (Your Task!)                              │
│                 internal/service/resources/api/openapi.yaml                      │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │  Endpoints:                                                              │    │
│  │  - /resourceTypes      ← Types of resources (node models, etc.)         │    │
│  │  - /resourcePools      ← Pools of resources (clusters, sites)           │    │
│  │  - /resources          ← Individual resources (nodes, NICs, etc.)       │    │
│  │  - /deploymentManagers ← Managed clusters                                │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
└───────────────────────────────────────┬─────────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              PostgreSQL Database                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │  Tables: resource_type, resource_pool, resource, deployment_manager     │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
└───────────────────────────────────────┬─────────────────────────────────────────┘
                                        ▲
                                        │ Populates
┌───────────────────────────────────────┴─────────────────────────────────────────┐
│                              RESOURCE COLLECTOR                                  │
│                    internal/service/resources/collector/                         │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │  DataSources:                                                            │    │
│  │  1. K8SDataSource         - Kubernetes clusters, nodes                   │    │
│  │  2. HwPluginDataSource    - Hardware inventory from plugins   ◄──────────┼────┤
│  └─────────────────────────────────────────────────────────────────────────┘    │
└───────────────────────────────────────┬─────────────────────────────────────────┘
                                        │
                                        │ Queries via REST API
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                     HARDWARE PLUGIN INVENTORY API                                │
│               hwmgr-plugins/api/openapi/specs/inventory.yaml                     │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │  Endpoints (Plugin-side):                                                │    │
│  │  - /inventory/hardwareProfiles    ← Available hardware profiles          │    │
│  │  - /inventory/resources           ← Hardware resources (BMHs, etc.)      │    │
│  │  - /inventory/resourcePools       ← Hardware pools                        │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
└───────────────────────────────────────┬─────────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                         METAL3 PLUGIN IMPLEMENTATION                             │
│                        hwmgr-plugins/metal3/server/                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │  - Queries BareMetalHost CRs from Metal3                                 │    │
│  │  - Translates to O2IMS resource format                                   │    │
│  │  - Returns hardware profiles, resources, pools                           │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### The Connection Points

| Layer | Component | Your Task Impact |
|-------|-----------|------------------|
| **O2IMS API** | `internal/service/resources/api/` | **Direct** - You'll modify these endpoints |
| **DB Models** | `internal/service/resources/db/models/` | **Direct** - May need to update schemas |
| **Collector** | `internal/service/resources/collector/` | **Indirect** - Consumes plugin data |
| **Plugin API** | `hwmgr-plugins/api/openapi/specs/inventory.yaml` | **Read** - Defines what data is available |
| **Plugin Server** | `hwmgr-plugins/metal3/server/` | **Read** - Implements data retrieval |

### Data Flow for Your Tasks

```
Your Inventory API Tasks:
├── Update ResourcePoolInfo   ──► resource_pool table ◄── Collector ◄── Plugin API
├── Update CloudInfo          ──► Direct from config (no plugin)
├── Update ResourceType       ──► resource_type table ◄── Collector ◄── Plugin API
└── New endpoints             ──► May need new collector data sources
```

### Sequence Diagram: Plugin ↔ Inventory Data Flow

The following sequence diagrams detail the step-by-step interactions between all actors in the plugin-to-inventory data flow.

#### Diagram 1: Data Collection Flow (Background Sync)

This shows how inventory data flows from hardware plugins into the database:

```
┌─────────┐ ┌──────────────┐ ┌─────────────────┐ ┌───────────────────┐ ┌─────────────────────┐ ┌──────────────┐
│Inventory│ │  resource-   │ │    Resource     │ │ HwPluginDataSource│ │ metal3-hwplugin-    │ │  Kubernetes  │
│Controller│ │   server     │ │   Collector     │ │  (per plugin)     │ │ server REST API     │ │     API      │
└────┬────┘ └──────┬───────┘ └────────┬────────┘ └─────────┬─────────┘ └──────────┬──────────┘ └──────┬───────┘
     │              │                  │                    │                      │                   │
     │ 1. Deploy    │                  │                    │                      │                   │
     │    resource- │                  │                    │                      │                   │
     │    server    │                  │                    │                      │                   │
     ├─────────────►│                  │                    │                      │                   │
     │              │                  │                    │                      │                   │
     │              │ 2. Start         │                    │                      │                   │
     │              │    collector     │                    │                      │                   │
     │              │    goroutine     │                    │                      │                   │
     │              ├─────────────────►│                    │                      │                   │
     │              │                  │                    │                      │                   │
     │              │                  │ 3. List HardwarePlugin CRs                │                   │
     │              │                  │────────────────────────────────────────────────────────────────►
     │              │                  │                    │                      │                   │
     │              │                  │◄───────────────────────────────────────────────────────────────│
     │              │                  │    [HardwarePlugin: metal3-hwplugin]      │                   │
     │              │                  │                    │                      │                   │
     │              │                  │ 4. Create          │                      │                   │
     │              │                  │    HwPluginData    │                      │                   │
     │              │                  │    Source          │                      │                   │
     │              │                  ├───────────────────►│                      │                   │
     │              │                  │                    │                      │                   │
     │              │                  │                    │ 5. GET /hardware-    │                   │
     │              │                  │                    │    manager/inventory │                   │
     │              │                  │                    │    /resourcePools    │                   │
     │              │                  │                    ├─────────────────────►│                   │
     │              │                  │                    │                      │                   │
     │              │                  │                    │                      │ 6. List           │
     │              │                  │                    │                      │    BareMetalHost  │
     │              │                  │                    │                      │    CRs            │
     │              │                  │                    │                      ├──────────────────►│
     │              │                  │                    │                      │                   │
     │              │                  │                    │                      │◄──────────────────│
     │              │                  │                    │                      │  [BMH list]       │
     │              │                  │                    │                      │                   │
     │              │                  │                    │◄─────────────────────│                   │
     │              │                  │                    │  [HW ResourcePools]  │                   │
     │              │                  │                    │                      │                   │
     │              │                  │                    │ 7. GET /hardware-    │                   │
     │              │                  │                    │    manager/inventory │                   │
     │              │                  │                    │    /resources        │                   │
     │              │                  │                    ├─────────────────────►│                   │
     │              │                  │                    │                      │                   │
     │              │                  │                    │                      │ 8. List BMH +     │
     │              │                  │                    │                      │    HostFirmware   │
     │              │                  │                    │                      │    Components     │
     │              │                  │                    │                      ├──────────────────►│
     │              │                  │                    │                      │                   │
     │              │                  │                    │                      │◄──────────────────│
     │              │                  │                    │                      │                   │
     │              │                  │                    │◄─────────────────────│                   │
     │              │                  │                    │  [HW Resources]      │                   │
     │              │                  │                    │                      │                   │
     │              │                  │◄───────────────────│                      │                   │
     │              │                  │  9. Return         │                      │                   │
     │              │                  │     ResourcePools, │                      │                   │
     │              │                  │     Resources,     │                      │                   │
     │              │                  │     ResourceTypes  │                      │                   │
     │              │                  │                    │                      │                   │
┌────┴────┐ ┌──────┴───────┐ ┌────────┴────────┐ ┌─────────┴─────────┐ ┌──────────┴──────────┐ ┌──────┴───────┐
│         │ │              │ │                 │ │                   │ │                     │ │              │
│         │ │              │ │                 │ │                   │ │                     │ │              │
│         │ │              │ │                 │ │                   │ │                     │ │              │
└─────────┘ └──────────────┘ └────────┬────────┘ └───────────────────┘ └─────────────────────┘ └──────────────┘
                                      │
                            ┌─────────┴─────────┐
                            │    PostgreSQL     │
                            │     Database      │
                            └─────────┬─────────┘
                                      │
     │              │                 │
     │              │                 │ 10. Upsert resource_pool
     │              │                 │     with generation_id
     │              │                 ├────────────────────────►
     │              │                 │
     │              │                 │ 11. Upsert resource_type
     │              │                 │     with generation_id
     │              │                 ├────────────────────────►
     │              │                 │
     │              │                 │ 12. Upsert resource
     │              │                 │     with generation_id
     │              │                 ├────────────────────────►
     │              │                 │
     │              │                 │ 13. Delete stale records
     │              │                 │     (old generation_id)
     │              │                 ├────────────────────────►
     │              │                 │
     │              │                 │ 14. Insert outbox events
     │              │                 │     for changed records
     │              │                 ├────────────────────────►
     │              │                 │
     │              │    [Repeat every N minutes]
     │              │                 │
```

#### Diagram 2: SMO API Request Flow

This shows how an SMO client queries the inventory API:

```
┌───────┐ ┌─────────────┐ ┌──────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐
│  SMO  │ │  OpenShift  │ │  resource-   │ │ Middleware │ │  Handler   │ │ Repository │ │ PostgreSQL │
│Client │ │   Ingress   │ │   server     │ │   Chain    │ │ (server.go)│ │            │ │            │
└───┬───┘ └──────┬──────┘ └──────┬───────┘ └─────┬──────┘ └─────┬──────┘ └─────┬──────┘ └─────┬──────┘
    │            │               │               │              │              │              │
    │ 1. GET /o2ims-infrastructureInventory/v1/resourcePools    │              │              │
    │    Authorization: Bearer <token>           │               │              │              │
    ├───────────►│               │               │              │              │              │
    │            │               │               │              │              │              │
    │            │ 2. Route to   │               │              │              │              │
    │            │    resource-  │               │              │              │              │
    │            │    server:8000│               │              │              │              │
    │            ├──────────────►│               │              │              │              │
    │            │               │               │              │              │              │
    │            │               │ 3. Apply      │              │              │              │
    │            │               │    middleware │              │              │              │
    │            │               ├──────────────►│              │              │              │
    │            │               │               │              │              │              │
    │            │               │               │ 4. authn:    │              │              │
    │            │               │               │    Validate  │              │              │
    │            │               │               │    JWT/Token │              │              │
    │            │               │               ├──────────────┤              │              │
    │            │               │               │              │              │              │
    │            │               │               │ 5. authz:    │              │              │
    │            │               │               │    K8s SAR   │              │              │
    │            │               │               │    check     │              │              │
    │            │               │               ├──────────────┤              │              │
    │            │               │               │              │              │              │
    │            │               │               │ 6. OpenAPI   │              │              │
    │            │               │               │    validation│              │              │
    │            │               │               ├──────────────┤              │              │
    │            │               │               │              │              │              │
    │            │               │               │ 7. Route to  │              │              │
    │            │               │               │    handler   │              │              │
    │            │               │               ├─────────────►│              │              │
    │            │               │               │              │              │              │
    │            │               │               │              │ 8. Call      │              │
    │            │               │               │              │    GetResource│             │
    │            │               │               │              │    Pools()   │              │
    │            │               │               │              ├─────────────►│              │
    │            │               │               │              │              │              │
    │            │               │               │              │              │ 9. SELECT    │
    │            │               │               │              │              │    from      │
    │            │               │               │              │              │    resource_ │
    │            │               │               │              │              │    pool      │
    │            │               │               │              │              ├─────────────►│
    │            │               │               │              │              │              │
    │            │               │               │              │              │◄─────────────│
    │            │               │               │              │              │   [rows]     │
    │            │               │               │              │              │              │
    │            │               │               │              │◄─────────────│              │
    │            │               │               │              │ []*ResourcePool             │
    │            │               │               │              │              │              │
    │            │               │               │              │ 10. Convert  │              │
    │            │               │               │              │     DB model │              │
    │            │               │               │              │     to API   │              │
    │            │               │               │              │     model    │              │
    │            │               │               │              ├──────────────┤              │
    │            │               │               │              │              │              │
    │            │               │               │◄─────────────│              │              │
    │            │               │               │ GetResourcePools200JSONResponse            │
    │            │               │               │              │              │              │
    │            │               │               │ 11. ResponseFilter:         │              │
    │            │               │               │     Apply ?fields,          │              │
    │            │               │               │     ?exclude_fields         │              │
    │            │               │               ├──────────────┤              │              │
    │            │               │               │              │              │              │
    │            │               │◄──────────────│              │              │              │
    │            │               │ 12. JSON response            │              │              │
    │            │               │     Content-Type: application/json          │              │
    │            │               │              │              │              │              │
    │            │◄──────────────│               │              │              │              │
    │            │               │               │              │              │              │
    │◄───────────│               │               │              │              │              │
    │ 13. HTTP 200 OK            │               │              │              │              │
    │     [{"resourcePoolId":..., "name":..., ...}]             │              │              │
    │            │               │               │              │              │              │
```

#### Diagram 3: Provisioning Flow (Hardware Plugin Interaction)

This shows how ProvisioningRequest triggers hardware allocation via plugins:

```
┌────────────┐ ┌─────────────────┐ ┌─────────────────┐ ┌────────────────────┐ ┌───────────────────┐ ┌────────────┐
│Provisioning│ │  controller-    │ │ metal3-hwplugin │ │  NodeAllocation    │ │   AllocatedNode   │ │ Kubernetes │
│  Request   │ │  manager        │ │ -server         │ │  Request CR        │ │       CR          │ │    API     │
│    CR      │ │  (PR Reconciler)│ │ (NAR Reconciler)│ │                    │ │                   │ │            │
└─────┬──────┘ └────────┬────────┘ └────────┬────────┘ └─────────┬──────────┘ └─────────┬─────────┘ └─────┬──────┘
      │                 │                   │                    │                      │                 │
      │ 1. User creates │                   │                    │                      │                 │
      │    Provisioning │                   │                    │                      │                 │
      │    Request CR   │                   │                    │                      │                 │
      ├────────────────►│                   │                    │                      │                 │
      │                 │                   │                    │                      │                 │
      │                 │ 2. Reconcile PR   │                    │                      │                 │
      │                 │    Validate params│                    │                      │                 │
      │                 ├──────────────────►│                    │                      │                 │
      │                 │                   │                    │                      │                 │
      │                 │ 3. Render NodeAllocationRequest        │                      │                 │
      │                 │    from HardwareTemplate               │                      │                 │
      │                 ├───────────────────────────────────────►│                      │                 │
      │                 │                   │                    │                      │                 │
      │                 │                   │ 4. Watch NAR       │                      │                 │
      │                 │                   │◄───────────────────│                      │                 │
      │                 │                   │                    │                      │                 │
      │                 │                   │ 5. Find matching BMHs via resourceSelector│                 │
      │                 │                   ├───────────────────────────────────────────────────────────►│
      │                 │                   │    List BareMetalHost where labels match                   │
      │                 │                   │                    │                      │                 │
      │                 │                   │◄───────────────────────────────────────────────────────────│
      │                 │                   │    [matched BMHs]  │                      │                 │
      │                 │                   │                    │                      │                 │
      │                 │                   │ 6. For each matched BMH:                  │                 │
      │                 │                   │    Create AllocatedNode CR               │                 │
      │                 │                   ├──────────────────────────────────────────►│                 │
      │                 │                   │                    │                      │                 │
      │                 │                   │ 7. Apply HardwareProfile settings:        │                 │
      │                 │                   │    - Update HostFirmwareSettings CR       │                 │
      │                 │                   │    - Update HostFirmwareComponents CR     │                 │
      │                 │                   ├───────────────────────────────────────────────────────────►│
      │                 │                   │                    │                      │                 │
      │                 │                   │ 8. Update NAR status: "HardwareProvisioning"               │
      │                 │                   ├───────────────────►│                      │                 │
      │                 │                   │                    │                      │                 │
      │                 │                   │ [Wait for firmware/BIOS settings to apply]│                 │
      │                 │                   │                    │                      │                 │
      │                 │                   │ 9. Update AllocatedNode status with:      │                 │
      │                 │                   │    - bmcAddress                           │                 │
      │                 │                   │    - bmcCredentialsName                   │                 │
      │                 │                   │    - bootMACAddress                       │                 │
      │                 │                   │    - interfaces[].macAddress              │                 │
      │                 │                   ├──────────────────────────────────────────►│                 │
      │                 │                   │                    │                      │                 │
      │                 │                   │ 10. Update NAR status: "Completed"        │                 │
      │                 │                   ├───────────────────►│                      │                 │
      │                 │                   │                    │                      │                 │
      │                 │ 11. Watch NAR status change            │                      │                 │
      │                 │◄──────────────────────────────────────│                      │                 │
      │                 │                   │                    │                      │                 │
      │                 │ 12. Read AllocatedNode details         │                      │                 │
      │                 │◄─────────────────────────────────────────────────────────────│                 │
      │                 │                   │                    │                      │                 │
      │                 │ 13. Update rendered ClusterInstance    │                      │                 │
      │                 │     with node hardware details         │                      │                 │
      │                 ├──────────────────►│                    │                      │                 │
      │                 │                   │                    │                      │                 │
      │                 │ 14. Create ClusterInstance CR          │                      │                 │
      │                 │     (triggers SiteConfig operator)     │                      │                 │
      │                 ├───────────────────────────────────────────────────────────────────────────────►│
      │                 │                   │                    │                      │                 │
      │ 15. Update PR   │                   │                    │                      │                 │
      │     status:     │                   │                    │                      │                 │
      │     "Progressing"                   │                    │                      │                 │
      │◄────────────────│                   │                    │                      │                 │
      │                 │                   │                    │                      │                 │
```

#### Actors Summary

| Actor | Type | Responsibility |
|-------|------|----------------|
| **SMO Client** | External | Queries O2IMS API, creates ProvisioningRequests |
| **OpenShift Ingress** | Infrastructure | Routes external traffic to internal services |
| **resource-server** | API Server | Serves Inventory API, manages data collection |
| **Resource Collector** | Background Process | Syncs data from multiple sources to PostgreSQL |
| **HwPluginDataSource** | Data Source | Fetches data from a specific hardware plugin |
| **metal3-hwplugin-server** | Hybrid (API + Controller) | Exposes hardware inventory API, manages BMH allocation |
| **controller-manager** | Operator | Reconciles Inventory, ClusterTemplate, ProvisioningRequest CRs |
| **PostgreSQL** | Database | Stores all inventory data (resource_pool, resource_type, resource) |
| **Kubernetes API** | Infrastructure | Manages CRs (HardwarePlugin, BMH, NAR, AllocatedNode) |

#### Data Flow Summary

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│                              COMPLETE DATA FLOW SUMMARY                                              │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘

                                    ┌─────────────────┐
                                    │   SMO Client    │
                                    └────────┬────────┘
                                             │
                              HTTP GET /resourcePools
                                             │
                                             ▼
┌───────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                      QUERY PATH                                                    │
│                                                                                                    │
│    ┌──────────┐     ┌─────────────┐     ┌────────────┐     ┌────────────┐     ┌────────────┐     │
│    │ Ingress  │────►│ resource-   │────►│ Middleware │────►│  Handler   │────►│ Repository │     │
│    │          │     │ server      │     │ (auth,     │     │ (business  │     │ (SQL       │     │
│    │          │     │             │     │ validation)│     │ logic)     │     │ queries)   │     │
│    └──────────┘     └─────────────┘     └────────────┘     └────────────┘     └─────┬──────┘     │
│                                                                                      │            │
│                                                                                      ▼            │
│                                                                               ┌────────────┐      │
│                                                                               │ PostgreSQL │      │
│                                                                               │ (cached    │      │
│                                                                               │ inventory) │      │
│                                                                               └─────┬──────┘      │
└─────────────────────────────────────────────────────────────────────────────────────┼─────────────┘
                                                                                      │
                                                                                      ▲
                                                                               Populated by
                                                                                      │
┌─────────────────────────────────────────────────────────────────────────────────────┼─────────────┐
│                                   COLLECTION PATH                                   │             │
│                                                                                     │             │
│    ┌─────────────┐     ┌─────────────────┐     ┌────────────────────┐              │             │
│    │ resource-   │────►│ HwPlugin        │────►│ metal3-hwplugin-   │              │             │
│    │ server      │     │ DataSource      │     │ server REST API    │              │             │
│    │ Collector   │     │                 │     │                    │──────────────┘             │
│    └─────────────┘     └─────────────────┘     └─────────┬──────────┘                            │
│                                                          │                                        │
│                                                          │ Queries                                │
│                                                          ▼                                        │
│                                              ┌────────────────────┐                               │
│                                              │   Kubernetes API   │                               │
│                                              │   - BareMetalHost  │                               │
│                                              │   - HardwarePlugin │                               │
│                                              │   - HostFirmware   │                               │
│                                              └────────────────────┘                               │
│                                                                                                   │
└───────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Kubernetes Pod Layout

### Question

> I need to understand the plugin architecture better in terms of containers, pods... Running in a cluster: could you specify the pod layout this o-cloud operator will have when deployed to k8s?

### Answer

### Complete Runtime Topology

```
┌──────────────────────────────────────────────────────────────────────────────────────────┐
│                            Namespace: oran-o2ims                                          │
└──────────────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                                    EXTERNAL ACCESS                                           │
│                        https://oran-o2ims.apps.<cluster-domain>/                            │
│  ┌──────────────────────────────────────────────────────────────────────────────────────┐   │
│  │                              OpenShift Ingress/Route                                  │   │
│  │   Routes to different backends based on path:                                         │   │
│  │   /o2ims-infrastructureInventory/*  → resource-server                                │   │
│  │   /o2ims-infrastructureMonitoring/* → alarms-server                                  │   │
│  │   /o2ims-infrastructureProvisioning/* → provisioning-server                          │   │
│  └──────────────────────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────────────────────┘
                                            │
                                            ▼
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                                     SERVICE MESH                                             │
│  ┌────────────────┐ ┌────────────────┐ ┌────────────────┐ ┌────────────────┐               │
│  │ resource-server│ │ cluster-server │ │ alarms-server  │ │ artifacts-server│               │
│  │    Service     │ │    Service     │ │    Service     │ │    Service      │               │
│  │   :8000/TCP    │ │   :8000/TCP    │ │   :8000/TCP    │ │   :8000/TCP     │               │
│  └───────┬────────┘ └───────┬────────┘ └───────┬────────┘ └───────┬─────────┘               │
│          │                  │                  │                  │                         │
│  ┌───────┴────────┐ ┌───────┴────────┐ ┌───────┴────────┐ ┌───────┴─────────┐               │
│  │ provisioning-  │ │ postgres-server│ │ hardwareplugin-│ │ metal3-hwplugin │               │
│  │ server Service │ │    Service     │ │ manager Service│ │ -server Service │               │
│  │   :8000/TCP    │ │   :5432/TCP    │ │   :8000/TCP    │ │   :8000/TCP     │               │
│  └───────┬────────┘ └───────┬────────┘ └───────┬────────┘ └───────┬─────────┘               │
└──────────┼──────────────────┼──────────────────┼──────────────────┼─────────────────────────┘
           │                  │                  │                  │
           ▼                  ▼                  ▼                  ▼
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                                        PODS                                                  │
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────────────────┐    │
│  │                           controller-manager (Deployment)                            │    │
│  │  ┌─────────────────────────────────────────────────────────────────────────────┐    │    │
│  │  │ Container: manager                                                          │    │    │
│  │  │ Image: quay.io/openshift-kni/oran-o2ims-operator:latest                    │    │    │
│  │  │ Command: oran-o2ims start controller-manager                               │    │    │
│  │  │ Reconciles: Inventory, ClusterTemplate, ProvisioningRequest CRs            │    │    │
│  │  └─────────────────────────────────────────────────────────────────────────────┘    │    │
│  └─────────────────────────────────────────────────────────────────────────────────────┘    │
│                                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────────────────┐    │
│  │                            postgres-server (Deployment)                              │    │
│  │  ┌─────────────────────────────────────────────────────────────────────────────┐    │    │
│  │  │ Container: server                                                           │    │    │
│  │  │ Image: quay.io/sclorg/postgresql-16-c9s:latest                             │    │    │
│  │  │ Port: 5432                                                                  │    │    │
│  │  │ Databases: resources, alarms, cluster                                       │    │    │
│  │  │ Volume: PVC (postgres-server-pvc)                                           │    │    │
│  │  └─────────────────────────────────────────────────────────────────────────────┘    │    │
│  └─────────────────────────────────────────────────────────────────────────────────────┘    │
│                                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────────────────┐    │
│  │                           resource-server (Deployment)                               │    │
│  │  ┌──────────────────────────────────────┐ ┌──────────────────────────────────────┐  │    │
│  │  │ InitContainer: migration             │ │ Container: server                    │  │    │
│  │  │ Command: oran-o2ims serve resources  │ │ Command: oran-o2ims serve resources  │  │    │
│  │  │          migrate                     │ │ Port: 8000 (HTTPS)                   │  │    │
│  │  │ Runs DB migrations                   │ │ Serves: /o2ims-infrastructureInventory│  │    │
│  │  └──────────────────────────────────────┘ └──────────────────────────────────────┘  │    │
│  └─────────────────────────────────────────────────────────────────────────────────────┘    │
│                                                                                              │
│  (Similar pattern for cluster-server, alarms-server, artifacts-server,                      │
│   provisioning-server, hardwareplugin-manager-server, metal3-hwplugin-server)               │
│                                                                                              │
└─────────────────────────────────────────────────────────────────────────────────────────────┘
```

### Pod Summary Table

| Pod Name | Replicas | Purpose | Port |
|----------|----------|---------|------|
| `controller-manager` | 1 | Main operator - reconciles Inventory, ClusterTemplate, ProvisioningRequest | - |
| `postgres-server` | 1 | PostgreSQL database for all services | 5432 |
| `resource-server` | 1 | O2IMS Inventory API | 8000 |
| `cluster-server` | 1 | Deployment Managers API | 8000 |
| `alarms-server` | 1 | O2IMS Monitoring/Alarms API | 8000 |
| `artifacts-server` | 1 | Cluster artifacts download | 8000 |
| `provisioning-server` | 1 | O2IMS Provisioning API | 8000 |
| `hardwareplugin-manager-server` | 1 | Hardware Plugin manager | - |
| `metal3-hwplugin-server` | 1 | Metal3 plugin (controllers + API) | 8000 |

---

## 5. Pod Container Composition

### Question

> When you have enumerated the pods in the 'pod summary table', are they really independent pods or can we find multi-container pods, with several containers running as part of the same pod?

### Answer

### Container Composition Per Pod

| Pod Name | Main Container | Init Container | Total Containers |
|----------|----------------|----------------|------------------|
| `controller-manager` | `manager` | ❌ None | **1** |
| `postgres-server` | `server` | ❌ None | **1** |
| `resource-server` | `server` | `migration` ✅ | **1 + 1 init** |
| `cluster-server` | `server` | `migration` ✅ | **1 + 1 init** |
| `alarms-server` | `server` | `migration` ✅ | **1 + 1 init** |
| `artifacts-server` | `server` | ❌ None | **1** |
| `provisioning-server` | `server` | ❌ None | **1** |
| `hardwareplugin-manager-server` | `server` | ❌ None | **1** |
| `metal3-hwplugin-server` | `server` | ❌ None | **1** |

### Init Container Pattern (Database Migration)

For pods that require database access (`resource-server`, `cluster-server`, `alarms-server`):

```yaml
spec:
  initContainers:
  - name: migration
    image: quay.io/openshift-kni/oran-o2ims-operator:latest
    command: ["oran-o2ims", "serve", "<service>", "migrate"]
    env:
    - name: <SERVICE>_PASSWORD
      valueFrom:
        secretKeyRef:
          name: <service>-database-password
          key: password
  containers:
  - name: server
    image: quay.io/openshift-kni/oran-o2ims-operator:latest
    command: ["oran-o2ims", "serve", "<service>"]
```

### Visual Container Layout

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                        PODS WITH DATABASE ACCESS                                 │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  resource-server pod                                                             │
│  ┌──────────────────────────────┐    ┌──────────────────────────────┐           │
│  │   InitContainer: migration   │───▶│   Container: server          │           │
│  │   (runs first, then exits)   │    │   (stays running)            │           │
│  │   - Runs SQL migrations      │    │   - Serves HTTP API          │           │
│  └──────────────────────────────┘    └──────────────────────────────┘           │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────┐
│                      PODS WITHOUT DATABASE (Single Container)                    │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  controller-manager pod          provisioning-server pod                         │
│  ┌──────────────────────────┐    ┌──────────────────────────────┐               │
│  │   Container: manager     │    │   Container: server          │               │
│  └──────────────────────────┘    └──────────────────────────────┘               │
│                                                                                  │
│  metal3-hwplugin-server pod (HYBRID - Controller + API in same container)        │
│  ┌──────────────────────────────────────────────────────────────────────────┐   │
│  │   Container: server                                                       │   │
│  │   - Runs Controller Manager (3 reconcilers) in goroutine                  │   │
│  │   - Runs REST API Server in goroutine                                     │   │
│  └──────────────────────────────────────────────────────────────────────────┘   │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Key Insight: Metal3 Hybrid Pod

The `metal3-hwplugin-server` is unique - it runs **both** a controller-manager AND an API server concurrently (in separate goroutines):

```go
// From hwmgr-plugins/metal3/cmd/start_metal3plugin_server.go
go func() {
    logger.Info("Starting Metal3 HardwarePlugin API server")
    err = metal3server.Serve(ctx, logger, c.CommonServerConfig, mgr.GetClient(), mgr.GetAPIReader())
}()

go func() {
    logger.Info("Starting manager")
    if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
        // ...
    }
}()
```

---

## 6. Dependencies Analysis

### Question

> More technical questions: I want you to assess the dependencies. Are we using volumes, external CRDs need to be installed as dependencies to deploy this o-cloud stuff, any other external secret?

### Answer

### 1. External CRDs (Must be installed BEFORE the operator)

| CRD Source | API Group | CRDs | Required By |
|------------|-----------|------|-------------|
| **RHACM (ACM)** | `cluster.open-cluster-management.io/v1` | `ManagedCluster` | Cluster discovery |
| **RHACM Policies** | `policy.open-cluster-management.io/v1` | `Policy` | Configuration management |
| **SiteConfig** | `siteconfig.open-cluster-management.io/v1alpha1` | `ClusterInstance` | Cluster provisioning |
| **Metal3** | `metal3.io/v1alpha1` | `BareMetalHost`, `HostFirmwareComponents` | Hardware inventory |
| **Hive** | `hive.openshift.io/v1` | `ClusterDeployment` | Cluster lifecycle |
| **Assisted Service** | `agent-install.openshift.io/v1beta1` | `InfraEnv`, `AgentClusterInstall` | Cluster installation |
| **IBGU** | `lcm.openshift.io/v1alpha1` | `ImageBasedGroupUpgrade` | Image-based upgrades |
| **MCO** | `observability.open-cluster-management.io/v1beta1` | `MultiClusterObservability` | Observability |
| **OpenShift Config** | `config.openshift.io/v1` | `ClusterVersion`, `Infrastructure` | Cluster info |
| **OpenShift Operator** | `operator.openshift.io/v1` | `IngressController` | Ingress config |

### 2. Volumes (PersistentVolumeClaims)

| Volume | Pod | Size | Purpose |
|--------|-----|------|---------|
| `postgres-server-pvc` | `postgres-server` | 10Gi (configurable) | PostgreSQL data storage |

### 3. Secrets

#### Auto-Generated Secrets (by operator)

| Secret Name | Purpose | Contents |
|-------------|---------|----------|
| `postgres-server-password` | PostgreSQL superuser | `password` key |
| `resources-database-password` | Resources DB user | `password` key |
| `cluster-database-password` | Cluster DB user | `password` key |
| `alarms-database-password` | Alarms DB user | `password` key |

#### TLS Secrets (auto-generated via OpenShift service-serving-cert)

| Secret Name | Purpose |
|-------------|---------|
| `resource-server-tls` | HTTPS for resource-server |
| `cluster-server-tls` | HTTPS for cluster-server |
| `alarms-server-tls` | HTTPS for alarms-server |
| `artifacts-server-tls` | HTTPS for artifacts-server |
| `provisioning-server-tls` | HTTPS for provisioning-server |

#### User-Provided Secrets (optional, via Inventory CR)

| Secret Reference | Purpose |
|------------------|---------|
| `spec.caBundleRef` | Custom CA bundle for TLS |
| `spec.smo.registrationEndpoint.tlsConfig.clientCertificateName` | mTLS client cert for SMO |

### 4. ConfigMaps

| ConfigMap | Purpose |
|-----------|---------|
| `oran-o2ims-env-config` | Environment configuration for servers |
| Cluster-specific ConfigMaps | Referenced by ClusterTemplate CRs |

### 5. Service Accounts & RBAC

| ServiceAccount | ClusterRole | Purpose |
|----------------|-------------|---------|
| `oran-o2ims-controller-manager` | `oran-o2ims-manager-role` | Main operator permissions |
| Per-server ServiceAccounts | Limited roles | Server-specific permissions |

### Dependency Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           EXTERNAL DEPENDENCIES                                  │
└─────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────┐
│                              REQUIRED OPERATORS                                  │
├─────────────────────────────────────────────────────────────────────────────────┤
│  ┌────────────────────┐  ┌────────────────────┐  ┌────────────────────┐        │
│  │  RHACM (ACM)       │  │  SiteConfig        │  │  Metal3            │        │
│  │  - ManagedCluster  │  │  - ClusterInstance │  │  - BareMetalHost   │        │
│  │  - Policy          │  │                    │  │  - HostFirmware    │        │
│  └────────────────────┘  └────────────────────┘  └────────────────────┘        │
└─────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────┐
│                              STORAGE REQUIREMENTS                                │
├─────────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    PersistentVolumeClaim                                 │    │
│  │  Name: postgres-server-pvc                                               │    │
│  │  Size: 10Gi (default)                                                    │    │
│  │  AccessMode: ReadWriteOnce                                               │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────┐
│                              SECRETS TOPOLOGY                                    │
├─────────────────────────────────────────────────────────────────────────────────┤
│  Auto-Generated (by operator):                                                   │
│  ┌─────────────────────┐ ┌─────────────────────┐ ┌─────────────────────┐        │
│  │ postgres-server-    │ │ resources-database- │ │ cluster-database-   │        │
│  │ password            │ │ password            │ │ password            │        │
│  └─────────────────────┘ └─────────────────────┘ └─────────────────────┘        │
│                                                                                  │
│  Auto-Generated (by OpenShift service-serving-cert-signer):                      │
│  ┌─────────────────────┐ ┌─────────────────────┐ ┌─────────────────────┐        │
│  │ resource-server-tls │ │ cluster-server-tls  │ │ alarms-server-tls   │        │
│  └─────────────────────┘ └─────────────────────┘ └─────────────────────┘        │
│                                                                                  │
│  User-Provided (optional):                                                       │
│  ┌─────────────────────┐ ┌─────────────────────┐                                │
│  │ Custom CA Bundle    │ │ SMO mTLS Client Cert│                                │
│  │ (spec.caBundleRef)  │ │ (spec.smo.tls...)   │                                │
│  └─────────────────────┘ └─────────────────────┘                                │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## 7. Operators vs API Servers - Reconcilers Analysis

### Question

> Perform another analysis, for every pod: how many of them are actually operators? And how many reconcilers do we have. You might want to respond to this in a table format, associating the CRs managed by every pod too.

### Answer

### Pod Classification Table

| Pod Name | Pod Type | Reconcilers | Primary CRs Managed | Secondary CRs (Watches/Owns) |
|----------|----------|-------------|---------------------|------------------------------|
| `controller-manager` | **Operator** ✅ | **3** | `Inventory`, `ClusterTemplate`, `ProvisioningRequest` | `Deployment`, `Namespace`, `ImageBasedGroupUpgrade`, `ClusterInstance`, `Policy`, `ManagedCluster`, `ConfigMap` |
| `hardwareplugin-manager-server` | **Operator** ✅ | **1** | `HardwarePlugin` | - |
| `metal3-hwplugin-server` | **Hybrid** ✅🌐 | **3** | `NodeAllocationRequest`, `AllocatedNode`, `HostFirmwareComponents` | + **REST API server** |
| `postgres-server` | **Database** 🗄️ | 0 | - | - |
| `resource-server` | **API Server** 🌐 | 0 | - | - |
| `cluster-server` | **API Server** 🌐 | 0 | - | - |
| `alarms-server` | **API Server** 🌐 | 0 | - | - |
| `artifacts-server` | **API Server** 🌐 | 0 | - | - |
| `provisioning-server` | **API Server** 🌐 | 0 | - | - |

### Detailed Reconciler Breakdown

#### 1. `controller-manager` Pod (3 Reconcilers)

| Reconciler | Primary CR | Watches | Owns |
|------------|------------|---------|------|
| `Reconciler` (Inventory) | `Inventory` | - | `Deployment` |
| `ClusterTemplateReconciler` | `ClusterTemplate` | `ConfigMap` | - |
| `ProvisioningRequestReconciler` | `ProvisioningRequest` | `ClusterTemplate`, `Policy`, `ManagedCluster` | `Namespace`, `ImageBasedGroupUpgrade`, `ClusterInstance` |

#### 2. `hardwareplugin-manager-server` Pod (1 Reconciler)

| Reconciler | Primary CR | Function |
|------------|------------|----------|
| `HardwarePluginReconciler` | `HardwarePlugin` | Manages HW plugin registration, updates status |

#### 3. `metal3-hwplugin-server` Pod (3 Reconcilers + API)

| Reconciler | Primary CR | Function |
|------------|------------|----------|
| `NodeAllocationRequestReconciler` | `NodeAllocationRequest` | Handles node allocation from HW manager |
| `AllocatedNodeReconciler` | `AllocatedNode` | Manages allocated node lifecycle |
| `HostFirmwareComponentsReconciler` | `HostFirmwareComponents` (Metal3) | Watches firmware updates, updates `AllocatedNode` |

### Summary Statistics

| Metric | Count |
|--------|-------|
| **Total Pods** | 9 |
| **Operator Pods** (with reconcilers) | 3 |
| **Pure API Server Pods** | 5 |
| **Database Pod** | 1 |
| **Total Reconcilers** | 7 |

### CR Ownership Map

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                              CRD MANAGEMENT TOPOLOGY                          │
└──────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                           controller-manager pod                             │
│  ┌──────────────────┐  ┌──────────────────────┐  ┌──────────────────────┐   │
│  │   Inventory      │  │  ClusterTemplate     │  │ ProvisioningRequest  │   │
│  │   Reconciler     │  │  Reconciler          │  │ Reconciler           │   │
│  └────────┬─────────┘  └──────────┬───────────┘  └──────────┬───────────┘   │
│           │                       │                          │              │
│     ┌─────▼─────┐           ┌─────▼─────┐          ┌─────────▼─────────┐    │
│     │ Inventory │           │ Cluster   │          │ Provisioning      │    │
│     │    CR     │           │ Template  │          │ Request CR        │    │
│     └───────────┘           │    CR     │          └───────────────────┘    │
│                             └───────────┘                                   │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                      hardwareplugin-manager-server pod                       │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                    HardwarePlugin Reconciler                          │   │
│  └─────────────────────────────────┬────────────────────────────────────┘   │
│                                    │                                        │
│                            ┌───────▼───────┐                                │
│                            │ HardwarePlugin│                                │
│                            │      CR       │                                │
│                            └───────────────┘                                │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                         metal3-hwplugin-server pod                           │
│  ┌─────────────────┐  ┌─────────────────┐  ┌────────────────────────────┐   │
│  │ NodeAllocation  │  │  AllocatedNode  │  │  HostFirmwareComponents    │   │
│  │ Request Rcnclr  │  │   Reconciler    │  │  Reconciler (Metal3)       │   │
│  └────────┬────────┘  └────────┬────────┘  └─────────────┬──────────────┘   │
│           │                    │                         │                  │
│    ┌──────▼──────┐      ┌──────▼──────┐          ┌───────▼───────┐          │
│    │ NodeAlloc   │      │ Allocated   │          │HostFirmware   │          │
│    │ Request CR  │      │ Node CR     │          │ Components CR │          │
│    └─────────────┘      └─────────────┘          └───────────────┘          │
│                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                      REST API Server (goroutine)                      │   │
│  │     /inventory endpoints  +  /provisioning endpoints                  │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                          PURE API SERVERS (No Reconcilers)                   │
├─────────────────────────────────────────────────────────────────────────────┤
│  resource-server      │ REST API: /o2ims-infrastructureInventory/v1/*       │
│  cluster-server       │ REST API: Deployment Managers API                   │
│  alarms-server        │ REST API: /o2ims-infrastructureMonitoring/v1/*      │
│  artifacts-server     │ REST API: Artifacts download                        │
│  provisioning-server  │ REST API: /o2ims-infrastructureProvisioning/v1/*    │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Key Insights

1. **The `controller-manager` is the main operator** - it manages the core O2IMS CRDs
2. **Hardware plugin architecture splits controllers across 2 pods**
3. **The `metal3-hwplugin-server` is unique** - it's the only pod running **both** controllers AND an API server
4. **API servers are purely reactive** - they serve HTTP requests against the PostgreSQL database

---

## 8. Inventory API Request/Response Code Path

### Question

> One more question: can you describe the code path since the inventory API receives a REST request, executing all the corresponding actions (I believe this will end up in a db query). Also, perform the same exercise for the associated response. I just want you to code walk me to be familiar with the code paths.

### Answer

### Example: `GET /o2ims-infrastructureInventory/v1/resourcePools/{resourcePoolId}`

### REQUEST PATH (Incoming)

#### Step 1: HTTP Server Receives Request

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│  HTTPS Request arrives at resource-server pod                                    │
│  GET /o2ims-infrastructureInventory/v1/resourcePools/abc-123-uuid               │
│  Headers: Authorization: Bearer <token>                                          │
└─────────────────────────────────────────────────────────────────────────────────┘
```

**File:** `internal/service/resources/serve.go`

```go
srv := &http.Server{
    Handler:      handler,
    Addr:         config.Listener.Address,
    TLSConfig:    serverTLSConfig,
    ReadTimeout:  readTimeout,
    WriteTimeout: writeTimeout,
    IdleTimeout:  idleTimeout,
}
```

#### Step 2: Middleware Chain Execution

```go
baseRouter := http.NewServeMux()
opt := generated.StdHTTPServerOptions{
    BaseRouter: baseRouter,
    Middlewares: []generated.MiddlewareFunc{
        middleware.OpenAPIValidation(swagger),
        middleware.ResponseFilter(filterAdapter),
        authz,
        authn,
        middleware.LogDuration(),
    },
    ErrorHandlerFunc: middleware.GetOranReqErrFunc(),
}
```

**Execution Order:**
1. `ErrorJsonifier()` → Wraps errors in JSON
2. `TrailingSlashStripper()` → Strips trailing `/`
3. `LogDuration()` → Starts timing
4. `authn` (Authenticator) → **Validates JWT/Token**
5. `authz` (Authorizer) → **Checks permissions (K8s SubjectAccessReview)**
6. `ResponseFilter()` → Sets up response interceptor
7. `OpenAPIValidation()` → **Validates request against OpenAPI spec**

#### Step 3: Authentication

**File:** `internal/service/common/auth/auth.go`

```go
func Authenticator(oauthHandler, kubernetesHandler authenticator.Request) middleware.Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
            response, ok, err := handler.AuthenticateRequest(req)
            if err != nil {
                middleware.ProblemDetails(w, "failed to authenticate", http.StatusUnauthorized)
                return
            }
            req = req.WithContext(request.WithUser(req.Context(), response.User))
            next.ServeHTTP(w, req)
        })
    }
}
```

#### Step 4: Handler Implementation (Your Code!)

**File:** `internal/service/resources/api/server.go`

```go
func (r *ResourceServer) GetResourcePool(ctx context.Context, request api.GetResourcePoolRequestObject) (api.GetResourcePoolResponseObject, error) {
    record, err := r.Repo.GetResourcePool(ctx, request.ResourcePoolId)
    if errors.Is(err, svcutils.ErrNotFound) {
        return api.GetResourcePool404ApplicationProblemPlusJSONResponse{
            Detail: "requested resourcePool not found",
            Status: http.StatusNotFound,
        }, nil
    }

    object := models.ResourcePoolToModel(record, commonapi.NewDefaultFieldOptions())
    return api.GetResourcePool200JSONResponse(object), nil
}
```

#### Step 5: Repository Layer (Database Query)

**File:** `internal/service/resources/db/repo/repository.go`

```go
func (r *ResourcesRepository) GetResourcePool(ctx context.Context, id uuid.UUID) (*models.ResourcePool, error) {
    return svcutils.Find[models.ResourcePool](ctx, r.Db, id)
}
```

#### Step 6: Generic Query Builder

**File:** `internal/service/common/utils/repository.go`

```go
func Find[T db.Model](ctx context.Context, db DBQuery, uuid uuid.UUID) (*T, error) {
    var record T
    tags := GetAllDBTagsFromStruct(record)

    sql, args, err := psql.Select(
        sm.Columns(tags.Columns()...),
        sm.From(record.TableName()),
        sm.Where(psql.Quote(record.PrimaryKey()).EQ(psql.Arg(uuid))),
    ).Build(ctx)

    return ExecuteCollectExactlyOneRow[T](ctx, db, sql, args)
}
```

**Generated SQL:**
```sql
SELECT resource_pool_id, global_location_id, name, description, 
       o_cloud_id, location, extensions, data_source_id, 
       generation_id, external_id, created_at 
FROM "resource_pool" 
WHERE "resource_pool_id" = $1
```

#### Step 7: Execute Query

```go
func ExecuteCollectExactlyOneRow[T db.Model](ctx context.Context, db DBQuery, sql string, args []any) (*T, error) {
    rows, err := db.Query(ctx, sql, args...)
    record, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[T])
    if errors.Is(err, pgx.ErrNoRows) {
        return nil, ErrNotFound
    }
    return &record, nil
}
```

### RESPONSE PATH (Outgoing)

#### Step 8: DB Model → API Model Conversion

**File:** `internal/service/resources/db/models/converters.go`

```go
func ResourcePoolToModel(record *ResourcePool, options *commonapi.FieldOptions) generated.ResourcePool {
    object := generated.ResourcePool{
        Description:      record.Description,
        GlobalLocationId: record.GlobalLocationID,
        Location:         record.Location,
        Name:             record.Name,
        OCloudId:         record.OCloudID,
        ResourcePoolId:   record.ResourcePoolID,
    }
    if options.IsIncluded(commonapi.ExtensionsAttribute) {
        object.Extensions = &record.Extensions
    }
    return object
}
```

#### Step 9: Typed Response Serialization (Generated)

```go
type GetResourcePool200JSONResponse ResourcePool

func (response GetResourcePool200JSONResponse) VisitGetResourcePoolResponse(w http.ResponseWriter) error {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(200)
    return json.NewEncoder(w).Encode(response)
}
```

### Complete Flow Diagram

```
   Client
     │
     │ GET /o2ims-infrastructureInventory/v1/resourcePools/{id}
     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 1. HTTP Server (serve.go) - TLS termination                                  │
└─────────────────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 2. Middleware Chain                                                          │
│    └─ authn → authz → ResponseFilter → OpenAPIValidation                    │
└─────────────────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 3. Generated Router (Go 1.22 ServeMux)                                       │
│    └─ Matches path, extracts {resourcePoolId}                               │
└─────────────────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 4. Handler (api/server.go)                                                   │
│    └─ GetResourcePool(ctx, request) → ResponseObject                        │
└─────────────────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 5. Repository (db/repo/repository.go)                                        │
│    └─ GetResourcePool(ctx, uuid) → *models.ResourcePool                     │
└─────────────────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 6. PostgreSQL Database                                                       │
│    └─ SELECT * FROM resource_pool WHERE resource_pool_id = $1               │
└─────────────────────────────────────────────────────────────────────────────┘
     │
     ▼ (Response path)
┌─────────────────────────────────────────────────────────────────────────────┐
│ 7. Model Converter → API Model                                               │
│ 8. Typed Response → JSON encode                                              │
│ 9. Middleware flush → Client                                                 │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Key Files Summary

| Layer | File | Purpose |
|-------|------|---------|
| **HTTP Server** | `internal/service/resources/serve.go` | Server setup, middleware chain |
| **Auth** | `internal/service/common/auth/auth.go` | Authn/Authz middleware |
| **Validation** | `internal/service/common/api/middleware/middleware.go` | OpenAPI validation |
| **Handlers** | `internal/service/resources/api/server.go` | Business logic |
| **Repository** | `internal/service/resources/db/repo/repository.go` | DB queries |
| **Query Utils** | `internal/service/common/utils/repository.go` | Generic CRUD |
| **DB Models** | `internal/service/resources/db/models/*.go` | DB struct definitions |
| **Converters** | `internal/service/resources/db/models/converters.go` | DB ↔ API mapping |

---

## 9. HTTP Infrastructure Deep Dive

### Question

> Ok, can you elaborate on the HTTP infra (HTTP server, golang code) used to serve the REST requests? Is it generated automatically by oapi-codegen too? What specific routing engine is generating then?

### Answer

### Overview

The project uses **Go 1.22's enhanced `net/http` standard library** as the routing engine, with code generation from **oapi-codegen v2.5.1**. No external routing frameworks (like Gin, Chi, or Echo) are used.

### 1. Code Generation with oapi-codegen

**Configuration File:** `internal/service/resources/api/tools/oapi-codegen.yaml`

```yaml
package: generated
output: ../generated/inventory.generated.go

generate:
  std-http-server: true     # ← Uses Go stdlib net/http
  client: false             # ← No client generation
  models: true              # ← Generate request/response types
  embedded-spec: true       # ← Embed OpenAPI spec in binary
  strict-server: true       # ← Generate type-safe handlers

output-options:
  skip-fmt: false
  skip-prune: true
  nullable-type: true
```

**Generation Trigger:** `internal/service/resources/api/tools/generate.go`

```go
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config oapi-codegen.yaml ../openapi.yaml
```

### 2. Routing Engine: Go 1.22 `net/http.ServeMux`

The generated code uses Go 1.22's **enhanced ServeMux** with method-based routing patterns:

```go
// From generated code
func HandlerWithOptions(si ServerInterface, options StdHTTPServerOptions) http.Handler {
    m := options.BaseRouter
    if m == nil {
        m = http.NewServeMux()
    }

    wrapper := ServerInterfaceWrapper{...}

    m.HandleFunc("GET "+options.BaseURL+"/o2ims-infrastructureInventory/v1/resourcePools", wrapper.GetResourcePools)
    m.HandleFunc("GET "+options.BaseURL+"/o2ims-infrastructureInventory/v1/resourcePools/{resourcePoolId}", wrapper.GetResourcePool)
    // ...
}
```

**Pattern format:** `"METHOD /path/{param}"`
- `{resourcePoolId}` - Path parameter extracted via `r.PathValue("resourcePoolId")`
- This is **Go 1.22+ syntax** - no external router needed!

### 3. Generated Architecture Layers

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                 Layer 1: http.ServeMux (Go 1.22 stdlib)                      │
│  Pattern: "GET /o2ims.../resourcePools/{resourcePoolId}"                    │
│  Handler: wrapper.GetResourcePool                                           │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│             Layer 2: ServerInterfaceWrapper (Generated)                      │
│  - Extracts path params via r.PathValue()                                   │
│  - Binds query params via runtime.BindQueryParameter()                      │
│  - Applies per-operation middlewares                                        │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│               Layer 3: strictHandler (Generated)                             │
│  - Builds typed RequestObject from parsed params                            │
│  - Calls your StrictServerInterface method                                  │
│  - Handles typed ResponseObject serialization                               │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│         Layer 4: Your Implementation (StrictServerInterface)                 │
│  - Pure business logic                                                      │
│  - No HTTP plumbing                                                         │
│  - Returns typed response objects                                           │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4. Type-Safe Response System

Each endpoint has typed response objects:

```go
type GetResourcePool200JSONResponse ResourcePool

func (response GetResourcePool200JSONResponse) VisitGetResourcePoolResponse(w http.ResponseWriter) error {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(200)
    return json.NewEncoder(w).Encode(response)
}

type GetResourcePool404ApplicationProblemPlusJSONResponse externalRef0.ProblemDetails

func (response GetResourcePool404ApplicationProblemPlusJSONResponse) VisitGetResourcePoolResponse(w http.ResponseWriter) error {
    w.Header().Set("Content-Type", "application/problem+json")
    w.WriteHeader(404)
    return json.NewEncoder(w).Encode(response)
}
```

Your handler just returns a typed object:

```go
func (r *ResourceServer) GetResourcePool(ctx context.Context, request api.GetResourcePoolRequestObject) (api.GetResourcePoolResponseObject, error) {
    // ...
    return api.GetResourcePool200JSONResponse(object), nil  // ← Type-safe!
}
```

### 5. Key Components Summary

| Component | Source | Purpose |
|-----------|--------|---------|
| **Router** | `net/http.ServeMux` (Go 1.22) | Pattern matching with `METHOD /path/{param}` |
| **Code Generator** | `oapi-codegen v2.5.1` | Generates types, routing, handlers from OpenAPI |
| **Path Params** | `r.PathValue("param")` | Go 1.22 native path parameter extraction |
| **Query Params** | `runtime.BindQueryParameter()` | oapi-codegen runtime binding |
| **Validation** | `kin-openapi/openapi3filter` | Request validation against OpenAPI spec |
| **JSON Encoding** | `encoding/json` (stdlib) | Request/response serialization |
| **TLS** | `crypto/tls` (stdlib) | HTTPS support |

### 6. Why This Architecture?

#### Advantages

1. **Zero External Dependencies** - Only Go stdlib + oapi-codegen runtime
2. **Type Safety** - Compile-time validation of request/response types
3. **OpenAPI First** - Spec is the source of truth
4. **Go 1.22 Features** - Modern routing without external frameworks
5. **Testability** - Mock `StrictServerInterface` easily
6. **Maintainability** - Regenerate when spec changes

#### Trade-offs

1. **Generated code is verbose** - ~3600 lines for inventory API
2. **Go 1.22+ required** - Can't use older Go versions
3. **Strict mode adds overhead** - Extra layer of indirection

---

## 10. Clarifying Questions: Inventory CR vs Inventory API vs Provisioning

### Question

> 1. I believe everything starts with provisioning the hardware. Who performs that? What's the Inventory CR and its relationship with the inventory request we will get from the SMO?

### Answer

This is a **critical distinction** that often causes confusion. Let me clarify the three different "inventory" concepts:

### The Three "Inventory" Concepts

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│                        CRITICAL DISTINCTION: THREE DIFFERENT THINGS                                  │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ 1. INVENTORY CR (Configuration)                                                                      │
│    Kind: Inventory                                                                                   │
│    API Group: ocloud.openshift.io/v1alpha1                                                          │
│    Purpose: CONFIGURES the O2IMS operator deployment                                                 │
│                                                                                                      │
│    What it does:                                                                                     │
│    - Tells operator WHICH servers to deploy (resource-server, alarms-server, etc.)                  │
│    - Configures SMO registration (URL, OAuth, mTLS)                                                 │
│    - Sets the global CloudID                                                                         │
│    - Configures TLS/Ingress settings                                                                │
│                                                                                                      │
│    Created by: Cluster Administrator (usually automatically at operator install)                    │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ 2. INVENTORY API (REST Endpoints)                                                                    │
│    Path: /o2ims-infrastructureInventory/v1/*                                                        │
│    Purpose: DISCOVERY of existing infrastructure (READ-ONLY)                                         │
│                                                                                                      │
│    What it does:                                                                                     │
│    - Returns resourcePools (clusters, hardware pools)                                               │
│    - Returns resourceTypes (node types, NIC types, etc.)                                            │
│    - Returns resources (individual nodes, NICs, accelerators)                                       │
│    - Returns deploymentManagers (managed clusters)                                                  │
│    - Manages subscriptions for change notifications                                                 │
│                                                                                                      │
│    Called by: SMO to DISCOVER what exists                                                           │
│    ⚠️ THIS DOES NOT PROVISION ANYTHING - IT'S READ-ONLY DISCOVERY!                                 │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ 3. PROVISIONING API (REST Endpoints + CR)                                                            │
│    Path: /o2ims-infrastructureProvisioning/v1/provisioningRequests                                  │
│    CR: ProvisioningRequest (clcm.openshift.io/v1alpha1)                                             │
│    Purpose: PROVISION new clusters (hardware + software)                                             │
│                                                                                                      │
│    What it does:                                                                                     │
│    - Allocates bare-metal hardware (via Hardware Plugin)                                            │
│    - Installs OpenShift clusters (via SiteConfig)                                                   │
│    - Applies configuration policies (via ACM)                                                       │
│                                                                                                      │
│    Called by: SMO to DEPLOY new infrastructure                                                      │
│    ✅ THIS IS WHERE HARDWARE PROVISIONING HAPPENS!                                                  │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

### Who Performs Hardware Provisioning?

**Short answer:** The **ProvisioningRequest** reconciler (in `controller-manager`) triggers hardware provisioning via the **Metal3 Hardware Plugin**.

**Detailed Flow:**

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│                              HARDWARE PROVISIONING FLOW                                              │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘

      SMO                                     O-Cloud Manager                           Hardware
       │                                            │                                       │
       │ 1. POST /o2ims-infrastructureProvisioning  │                                       │
       │    /v1/provisioningRequests                │                                       │
       │    {templateName, templateParameters...}   │                                       │
       ├───────────────────────────────────────────►│                                       │
       │                                            │                                       │
       │                                            │ 2. Create ProvisioningRequest CR      │
       │                                            ├──────────────────────────────────────►│
       │                                            │                                       │
       │                                            │ 3. PR Reconciler creates              │
       │                                            │    NodeAllocationRequest CR           │
       │                                            ├──────────────────────────────────────►│
       │                                            │                                       │
       │                                            │ 4. Metal3 Plugin:                     │
       │                                            │    - Finds matching BareMetalHosts    │
       │                                            │    - Applies BIOS/firmware settings   │
       │                                            │    - Creates AllocatedNode CRs        │
       │                                            │◄──────────────────────────────────────│
       │                                            │                                       │
       │                                            │ 5. PR Reconciler creates              │
       │                                            │    ClusterInstance CR                 │
       │                                            │    (SiteConfig installs cluster)      │
       │                                            ├──────────────────────────────────────►│
       │                                            │                                       │
       │ 6. Poll GET /provisioningRequests/{id}     │                                       │
       │    to monitor progress                     │                                       │
       │◄───────────────────────────────────────────│                                       │
       │                                            │                                       │
```

### The Inventory CR Explained

The `Inventory` CR is **NOT** related to inventory data - it's a **configuration resource** for the operator:

```yaml
# This is the CONFIGURATION CR (not inventory data!)
apiVersion: ocloud.openshift.io/v1alpha1
kind: Inventory
metadata:
  name: default
  namespace: oran-o2ims
spec:
  # Global cloud ID for SMO correlation
  cloudID: "550e8400-e29b-41d4-a716-446655440000"
  
  # Configure which servers to deploy
  resourceServerConfig: {}      # Deploy resource-server
  clusterServerConfig: {}       # Deploy cluster-server
  alarmServerConfig: {}         # Deploy alarms-server
  artifactsServerConfig: {}     # Deploy artifacts-server
  provisioningServerConfig: {}  # Deploy provisioning-server
  
  # SMO registration settings
  smo:
    url: "https://smo.example.com"
    registrationEndpoint: "/ocloud_observer"
    oauth:
      url: "https://keycloak.example.com/realms/oran"
      clientSecretName: "oauth-client-secrets"
      tokenEndpoint: "/protocol/openid-connect/token"
    tls:
      secretName: "oran-o2ims-tls-certificate"
  
  # Ingress/TLS configuration
  ingress:
    ingressHost: "o2ims.apps.mycluster.example.com"
    tls:
      secretName: "oran-o2ims-tls-certificate"
```

### Relationship Summary

| Concept | Type | Created By | Purpose |
|---------|------|------------|---------|
| **Inventory CR** | Kubernetes CRD | Cluster Admin | Configure O2IMS deployment |
| **Inventory API** | REST API | - | SMO discovers existing infrastructure |
| **ProvisioningRequest** | REST API + CRD | SMO | Deploy new clusters |

### Timeline of Events

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                    SYSTEM TIMELINE                                                   │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘

TIME ──────────────────────────────────────────────────────────────────────────────────────────────────►

1. SETUP PHASE (One-time, by Admin)
   │
   ├─► Install O-Cloud Manager Operator
   │
   ├─► Create Inventory CR (configures operator)
   │
   ├─► Operator deploys: postgres, resource-server, alarms-server, provisioning-server, etc.
   │
   └─► Operator registers with SMO

2. DISCOVERY PHASE (Ongoing, by Collector)
   │
   ├─► Resource Collector syncs data every N minutes
   │   ├─► From ACM: ManagedClusters → resourcePools, deploymentManagers
   │   └─► From Hardware Plugins: BMHs → resources, resourceTypes
   │
   └─► Data stored in PostgreSQL

3. SMO QUERIES (On-demand, by SMO)
   │
   ├─► GET /o2ims-infrastructureInventory/v1/resourcePools
   │   └─► Returns existing hardware pools (from PostgreSQL cache)
   │
   ├─► GET /o2ims-infrastructureInventory/v1/resources
   │   └─► Returns existing hardware (nodes, NICs, etc.)
   │
   └─► This is READ-ONLY discovery!

4. PROVISIONING PHASE (On-demand, by SMO)
   │
   ├─► POST /o2ims-infrastructureProvisioning/v1/provisioningRequests
   │   └─► Creates ProvisioningRequest CR
   │
   ├─► Hardware Plugin allocates BMHs, applies firmware
   │
   ├─► SiteConfig installs OpenShift cluster
   │
   └─► ACM applies configuration policies
```

### Key Takeaway

**The Inventory API does NOT provision hardware!**

- **Inventory API** = "What hardware/clusters do I have?" (Discovery)
- **Provisioning API** = "Deploy a new cluster on this hardware" (Action)

The Inventory CR is just configuration for the operator itself - it tells the operator how to deploy and configure the O2IMS services.

---

## 11. Provisioning Flow Deep Dive: Who Does What?

### Question

> Let me still focus on the provisioning flow:
> 1. Who provides the endpoint /provisioningRequests?
> 2. Does the Metal3 plugin create the AllocatedNode CRs and ClusterInstance CR too? When does that happen?

### Answer

### 1. Who Provides `/provisioningRequests`?

The **`provisioning-server`** pod provides the `/o2ims-infrastructureProvisioning/v1/provisioningRequests` endpoint.

**Code Location:** `internal/service/provisioning/`

```go
// internal/service/provisioning/api/server.go
type ProvisioningServer struct {
    HubClient client.Client  // ← K8s client to create CRs
}

// CreateProvisioningRequest handles POST /provisioningRequests
func (r *ProvisioningServer) CreateProvisioningRequest(ctx context.Context, request api.CreateProvisioningRequestRequestObject) (api.CreateProvisioningRequestResponseObject, error) {
    // Convert REST API request to Kubernetes CR
    provisioningRequest, err := convertProvisioningRequestApiToCR(*request.Body)
    
    // Create the ProvisioningRequest CR in Kubernetes
    err = r.HubClient.Create(ctx, provisioningRequest)  // ← Creates CR!
    // ...
}
```

**The provisioning-server is a "thin wrapper"** - it:
1. Receives REST requests from SMO
2. Converts them to `ProvisioningRequest` CRs
3. Creates/updates/deletes CRs in Kubernetes
4. Returns CR status as REST responses

It does **NOT** perform any provisioning logic - it just manages CRs!

### 2. Who Creates What CRs?

Here's the **exact breakdown** of which component creates which CR:

| CR | Created By | Pod | When |
|----|-----------|-----|------|
| `ProvisioningRequest` | **provisioning-server** | `provisioning-server` | When SMO calls POST /provisioningRequests |
| `NodeAllocationRequest` | **PR Reconciler** | `controller-manager` | After PR validation, before hardware provisioning |
| `AllocatedNode` | **Metal3 NAR Reconciler** | `metal3-hwplugin-server` | When matching BMH found for each node group |
| `ClusterInstance` | **PR Reconciler** | `controller-manager` | After hardware provisioning completes |

### Complete Provisioning Sequence Diagram

```
┌───────┐ ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────────┐ ┌─────────────────────┐
│  SMO  │ │ provisioning-   │ │ controller-     │ │ metal3-hwplugin-    │ │    Kubernetes       │
│       │ │ server          │ │ manager         │ │ server              │ │    API Server       │
│       │ │ (REST API)      │ │ (PR Reconciler) │ │ (NAR Reconciler)    │ │                     │
└───┬───┘ └────────┬────────┘ └────────┬────────┘ └──────────┬──────────┘ └──────────┬──────────┘
    │              │                   │                     │                       │
    │ 1. POST      │                   │                     │                       │
    │ /provisioning│                   │                     │                       │
    │ Requests     │                   │                     │                       │
    ├─────────────►│                   │                     │                       │
    │              │                   │                     │                       │
    │              │ 2. Create         │                     │                       │
    │              │ Provisioning      │                     │                       │
    │              │ Request CR        │                     │                       │
    │              ├──────────────────────────────────────────────────────────────────►
    │              │                   │                     │                       │
    │◄─────────────│ 3. Return 201    │                     │                       │
    │   Created    │                   │                     │                       │
    │              │                   │                     │                       │
    │              │                   │ 4. Watch: PR        │                       │
    │              │                   │    created          │                       │
    │              │                   │◄─────────────────────────────────────────────│
    │              │                   │                     │                       │
    │              │                   │ 5. Validate PR      │                       │
    │              │                   │    (template,       │                       │
    │              │                   │    schema, etc.)    │                       │
    │              │                   ├─────────────────────│                       │
    │              │                   │                     │                       │
    │              │                   │ 6. Create           │                       │
    │              │                   │    NodeAllocation   │                       │
    │              │                   │    Request CR       │                       │
    │              │                   ├──────────────────────────────────────────────►
    │              │                   │                     │                       │
    │              │                   │                     │ 7. Watch: NAR         │
    │              │                   │                     │    created            │
    │              │                   │                     │◄──────────────────────│
    │              │                   │                     │                       │
    │              │                   │                     │ 8. Find matching      │
    │              │                   │                     │    BareMetalHosts     │
    │              │                   │                     │    (by labels)        │
    │              │                   │                     ├──────────────────────►│
    │              │                   │                     │◄──────────────────────│
    │              │                   │                     │    [BMH list]         │
    │              │                   │                     │                       │
    │              │                   │                     │ 9. For EACH matched   │
    │              │                   │                     │    BMH:               │
    │              │                   │                     │    Create Allocated   │
    │              │                   │                     │    Node CR            │
    │              │                   │                     ├──────────────────────►│
    │              │                   │                     │                       │
    │              │                   │                     │ 10. Apply HW Profile  │
    │              │                   │                     │     (BIOS, firmware)  │
    │              │                   │                     ├──────────────────────►│
    │              │                   │                     │                       │
    │              │                   │                     │ 11. Update Allocated  │
    │              │                   │                     │     Node status with: │
    │              │                   │                     │     - BMC address     │
    │              │                   │                     │     - MAC addresses   │
    │              │                   │                     ├──────────────────────►│
    │              │                   │                     │                       │
    │              │                   │                     │ 12. Update NAR        │
    │              │                   │                     │     status:           │
    │              │                   │                     │     "Provisioned"     │
    │              │                   │                     ├──────────────────────►│
    │              │                   │                     │                       │
    │              │                   │ 13. Watch: NAR      │                       │
    │              │                   │     Provisioned     │                       │
    │              │                   │◄─────────────────────────────────────────────│
    │              │                   │                     │                       │
    │              │                   │ 14. Read Allocated  │                       │
    │              │                   │     Node details    │                       │
    │              │                   ├──────────────────────────────────────────────►
    │              │                   │◄─────────────────────────────────────────────│
    │              │                   │                     │                       │
    │              │                   │ 15. Create          │                       │
    │              │                   │     ClusterInstance │                       │
    │              │                   │     CR (with BMC    │                       │
    │              │                   │     details)        │                       │
    │              │                   ├──────────────────────────────────────────────►
    │              │                   │                     │                       │
    │              │                   │                     │                       │
    │              │                   │     [SiteConfig operator takes over         │
    │              │                   │      and installs the cluster]              │
    │              │                   │                     │                       │
```

### Code Paths for Each CR Creation

#### 1. ProvisioningRequest CR - Created by `provisioning-server`

```go
// internal/service/provisioning/api/server.go:136
func (r *ProvisioningServer) CreateProvisioningRequest(...) {
    provisioningRequest, _ := convertProvisioningRequestApiToCR(*request.Body)
    err = r.HubClient.Create(ctx, provisioningRequest)  // ← HERE
}
```

#### 2. NodeAllocationRequest CR - Created by `controller-manager` (PR Reconciler)

```go
// internal/controllers/provisioningrequest_hwprovision.go:88
func (t *provisioningRequestReconcilerTask) createNodeAllocationRequestResources(...) {
    // Create the node allocation request resource
    nodeAllocationRequestID, err := t.hwpluginClient.CreateNodeAllocationRequest(ctx, *nodeAllocationRequest)  // ← HERE
}
```

#### 3. AllocatedNode CR - Created by `metal3-hwplugin-server` (NAR Reconciler)

```go
// hwmgr-plugins/metal3/controller/helpers.go:232
func createNode(ctx context.Context, c client.Client, ...) error {
    node := &pluginsv1alpha1.AllocatedNode{
        ObjectMeta: metav1.ObjectMeta{
            Name:      nodename,
            Namespace: pluginNamespace,
            OwnerReferences: []metav1.OwnerReference{{
                // Owned by NodeAllocationRequest
            }},
        },
        Spec: pluginsv1alpha1.AllocatedNodeSpec{
            NodeAllocationRequest: nodeAllocationRequest.Name,
            GroupName:             groupname,
            HwProfile:             hwprofile,
        },
    }
    
    if err := c.Create(ctx, node); err != nil {  // ← HERE
        return fmt.Errorf("failed to create AllocatedNode: %w", err)
    }
}
```

#### 4. ClusterInstance CR - Created by `controller-manager` (PR Reconciler)

```go
// internal/controllers/provisioningrequest_clusterinstall.go:291
func (t *provisioningRequestReconcilerTask) applyClusterInstance(ctx context.Context, clusterInstance client.Object, isDryRun bool) error {
    // ...
    // Execute the Apply Operation (Server-Side Apply)
    if err := t.client.Patch(ctx, patchObj, client.Apply, patchOpts...); // ← HERE
}
```

### Summary: Who Does What?

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                    PROVISIONING RESPONSIBILITIES                                     │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  provisioning-server (REST API Server)                                                               │
│  ─────────────────────────────────────                                                              │
│  Creates: ProvisioningRequest CR                                                                    │
│  Role: REST-to-CR gateway (thin wrapper)                                                            │
│  Does NOT: Any provisioning logic                                                                   │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘
                                              │
                                              ▼
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  controller-manager / PR Reconciler (Operator)                                                       │
│  ─────────────────────────────────────────────                                                      │
│  Creates: NodeAllocationRequest CR, ClusterInstance CR                                              │
│  Role: Orchestrates entire provisioning workflow                                                    │
│  - Validates ProvisioningRequest                                                                    │
│  - Renders templates                                                                                │
│  - Creates NAR for hardware allocation                                                              │
│  - Waits for hardware provisioning                                                                  │
│  - Creates ClusterInstance after hardware ready                                                     │
│  - Monitors cluster installation and configuration                                                  │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘
                                              │
                                              ▼
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  metal3-hwplugin-server / NAR Reconciler (Hybrid: Operator + API)                                    │
│  ───────────────────────────────────────────────────────────────                                    │
│  Creates: AllocatedNode CR (one per matched BareMetalHost)                                          │
│  Role: Hardware allocation and configuration                                                        │
│  - Watches NodeAllocationRequest CRs                                                                │
│  - Finds BareMetalHosts matching resourceSelector                                                   │
│  - Creates AllocatedNode CR for each matched host                                                   │
│  - Applies HardwareProfile (BIOS, firmware settings)                                                │
│  - Updates AllocatedNode with BMC/MAC details                                                       │
│  - Signals completion via NAR status                                                                │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘
                                              │
                                              ▼
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  SiteConfig Operator (External - ACM)                                                                │
│  ────────────────────────────────────                                                               │
│  Watches: ClusterInstance CR                                                                        │
│  Creates: BareMetalHost provisioning, AgentClusterInstall, etc.                                     │
│  Role: Actual cluster installation                                                                  │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

### Key Insight: Metal3 Plugin Does NOT Create ClusterInstance

The **Metal3 plugin only handles hardware** (BMH selection, firmware, AllocatedNode CRs).

The **ClusterInstance is created by the PR Reconciler** in `controller-manager` AFTER:
1. Hardware provisioning is complete (NAR status = Provisioned)
2. AllocatedNode details (BMC address, MAC addresses) are read
3. These details are merged into the rendered ClusterInstance

---

## 12. Corrections and Refinements (Based on Official Documentation Review)

> **Note:** After reviewing the official documentation under `docs/`, the following corrections and refinements were identified to align my analysis with the project's canonical documentation.

### 10.1 Pod Container Composition - Correction

**Original Analysis (Incorrect):**
> Most pods show 1/1 Running

**Corrected (from `docs/user-guide/environment-setup.md`):**

The actual pod output shows **2/2 Running** for most pods, indicating a **sidecar container** (likely `kube-rbac-proxy` for metrics):

```console
$ oc get pods -n oran-o2ims
NAME                                             READY   STATUS    RESTARTS      AGE
alarms-server-5d5cfb75bf-rbp6g                   2/2     Running   0             21s
artifacts-server-c48f6bd99-xnk2n                 2/2     Running   0             21s
cluster-server-68f8946f74-l82bn                  2/2     Running   0             21s
oran-o2ims-controller-manager-555755dbd7-sprs9   2/2     Running   0             26s
postgres-server-674458bfbd-mnzt5                 1/1     Running   0             23s    ← Only postgres is 1/1
provisioning-server-86bd6bf6f-kl829              2/2     Running   0             20s
resource-server-6dbd5788df-vpq44                 2/2     Running   0             22s
```

**Corrected Container Table:**

| Pod Name | Main Container | Sidecar | Init Container | Total |
|----------|----------------|---------|----------------|-------|
| `controller-manager` | `manager` | `kube-rbac-proxy` | ❌ | **2** |
| `postgres-server` | `server` | ❌ | ❌ | **1** |
| `resource-server` | `server` | `kube-rbac-proxy` | `migration` | **2 + 1 init** |
| `cluster-server` | `server` | `kube-rbac-proxy` | `migration` | **2 + 1 init** |
| `alarms-server` | `server` | `kube-rbac-proxy` | `migration` | **2 + 1 init** |
| `artifacts-server` | `server` | `kube-rbac-proxy` | ❌ | **2** |
| `provisioning-server` | `server` | `kube-rbac-proxy` | ❌ | **2** |

### 10.2 Prerequisites - Correction

**Original Analysis:** Generic OpenShift 4.x requirements

**Corrected (from `docs/user-guide/prereqs.md`):**

| Requirement | Version/Details |
|-------------|-----------------|
| **OpenShift** | 4.20.0-rc3 or newer |
| **ACM** | v2.14 or newer |
| **SiteConfig Operator** | Enabled via ACM patch |
| **Observability Operator** | Required for alarms |
| **Red Hat OpenShift GitOps** | Required |
| **TALM** | Topology Aware Lifecycle Manager |
| **Storage** | Default StorageClass with RWO, **20Gi** free PV |

### 10.3 Storage Requirements - Correction

**Original Analysis:** `10Gi (configurable)`

**Corrected (from `docs/user-guide/prereqs.md`):**

> "Ensure a free PersistentVolume with at least **20 Gi** capacity is available for the operator's internal database"

### 10.4 Routes/Ingress - Addition

**Original Analysis:** Listed 3 main routes

**Complete Route List (from `docs/user-guide/inventory-api.md`):**

```console
$ oc get routes -n oran-o2ims
NAME                       HOST/PORT                                    PATH                                SERVICES              PORT   TERMINATION
oran-o2ims-ingress-...     o2ims.apps.<cluster>                        /o2ims-infrastructureInventory      resource-server       api    reencrypt
oran-o2ims-ingress-...     o2ims.apps.<cluster>                        /o2ims-infrastructureMonitoring     alarms-server         api    reencrypt
oran-o2ims-ingress-...     o2ims.apps.<cluster>                        /o2ims-infrastructureProvisioning   provisioning-server   api    reencrypt
oran-o2ims-ingress-...     o2ims.apps.<cluster>                        /o2ims-infrastructureArtifacts      artifacts-server      api    reencrypt
oran-o2ims-ingress-...     o2ims.apps.<cluster>                        /o2ims-infrastructureCluster        cluster-server        api    reencrypt
```

**5 Routes Total** (not 3 as originally indicated)

### 10.5 Resource Server Architecture - Key Design Pattern

**From `docs/enhancements/infrastructure-inventory-services-api/resource-server.md`:**

The resource-server implements a **PostgreSQL-backed caching architecture** that evolved from the original pass-through to ACM design:

**Key Architectural Patterns:**

1. **Generation ID Pattern** - For data synchronization from multiple sources:
   ```sql
   -- Each data source has a generation_id that increments on sync
   -- Stale data (generation_id < current) is purged after sync
   data_source_id UUID NOT NULL,
   generation_id INTEGER NOT NULL,
   ```

2. **Outbox Pattern** - For event notifications to subscribers:
   ```sql
   -- Outbox table captures before/after state for CDC
   -- INSERT: {before=NULL, after={new}}
   -- UPDATE: {before={old}, after={new}}
   -- DELETE: {before={old}, after=NULL}
   ```

3. **Service Consolidation** - Three previously separate microservices were merged:
   - Deployment Manager Server
   - Resource Server
   - Inventory Subscription Server
   
   → All consolidated into **resource-server**

### 10.6 Alarms Architecture - Key Concepts

**From `docs/enhancements/infrastructure-monitoring-service-api/alarms.md`:**

**Critical Data Relationships:**

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    ALARMS ARCHITECTURE RELATIONSHIPS                         │
└─────────────────────────────────────────────────────────────────────────────┘

ResourceType (from Inventory)
     │
     │ 1:1
     ▼
AlarmDictionary
     │ version = major.minor OCP version
     │
     │ 1:N
     ▼
AlarmDefinition (from PrometheusRules)
     │
     │ 1:1
     ▼
ProbableCause (auto-generated)
```

**Key Tables:**
- `alarm_dictionary` - Links ResourceType to AlarmDefinitions via OCP version
- `alarm_definitions` - Mapped from PrometheusRule CRs
- `probable_causes` - Auto-generated for each AlarmDefinition
- `alarm_event_record` - Active alarms from Alertmanager webhooks
- `alarm_subscription_info` - SMO subscription tracking with `event_cursor`

### 10.7 Hardware Provisioning Flow - Enhanced Detail

**From `docs/user-guide/cluster-provisioning.md`:**

**Complete Provisioning Workflow:**

```
ProvisioningRequest CR
     │
     ├─► ProvisioningRequestValidated
     │   └─ Validate timeout values, clusterInstanceParameters, policyTemplateParameters
     │
     ├─► ClusterInstanceRendered
     │   └─ Render ClusterInstance CR, validate via dry-run
     │
     ├─► ClusterResourcesCreated
     │   └─ Copy extra-manifests ConfigMap, pull-secret, create policy ConfigMap
     │
     ├─► HardwareTemplateRendered
     │   └─ Render NodeAllocationRequest from HardwareTemplate
     │
     ├─► HardwareProvisioned
     │   └─ Metal3 plugin: select BMHs, create AllocatedNode CRs, apply firmware
     │
     ├─► HardwareNodeConfigApplied
     │   └─ Update ClusterInstance with BMC details, MAC addresses from AllocatedNode
     │
     ├─► ClusterProvisioned
     │   └─ Create ClusterInstance → SiteConfig operator starts installation
     │
     └─► ConfigurationApplied
         └─ ACM enforces policies → ZTP Done
```

**Default Timeouts:**
- Hardware provisioning: **90m**
- Cluster installation: **90m**
- Cluster configuration: **30m**

### 10.8 ACM Observability Dependency - Critical for Alarms

**From `docs/dev/env_acm.md` and `docs/user-guide/alarms-user-guide.md`:**

The alarms-server is **completely dependent** on ACM Observability:

```bash
# Required for alarms to function
oc get MultiClusterObservability -n open-cluster-management-observability
oc get pods -n open-cluster-management-observability | grep alertmanager
```

**Alertmanager Webhook Configuration:**
```yaml
route:
  receiver: webhook_receiver
receivers:
  - name: webhook_receiver
    webhook_configs:
      - url: "http://alarm-server.oran-o2ims.svc.cluster.local/internal/v1/caas-alerts/alertmanager"
        send_resolved: true
```

### 10.9 Related Documentation Links

| Topic | Official Documentation |
|-------|----------------------|
| **Inventory API Usage** | `docs/user-guide/inventory-api.md` |
| **Environment Setup** | `docs/user-guide/environment-setup.md` |
| **Prerequisites** | `docs/user-guide/prereqs.md` |
| **Cluster Provisioning** | `docs/user-guide/cluster-provisioning.md` |
| **Template Overview** | `docs/user-guide/template-overview.md` |
| **Alarms User Guide** | `docs/user-guide/alarms-user-guide.md` |
| **ACM Configuration** | `docs/dev/env_acm.md` |
| **Resource Server Enhancement** | `docs/enhancements/infrastructure-inventory-services-api/resource-server.md` |
| **Alarms Enhancement** | `docs/enhancements/infrastructure-monitoring-service-api/alarms.md` |
| **Troubleshooting** | `docs/troubleshooting.md` |

---

## Document Information

- **Generated:** January 12, 2026
- **Reviewed:** January 19, 2026 (corrections based on official docs)
- **Project:** O-RAN O2IMS Operator
- **Repository:** `github.com/openshift-kni/oran-o2ims`
