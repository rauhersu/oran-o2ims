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
12. [Is Metal3 the Only Plugin?](#12-is-metal3-the-only-plugin)
13. [Corrections and Refinements](#13-corrections-and-refinements-based-on-official-documentation-review) *(Added after official docs review)*
14. [Server Pattern Deep Dive](#14-server-pattern-deep-dive) *(Added January 21, 2026)*
    - [Single Dockerfile Architecture](#141-single-dockerfile-architecture)
    - [Single Binary with Cobra Subcommands](#142-single-binary-with-cobra-subcommands)
    - [Deployment Creation by Inventory Controller](#143-deployment-creation-by-inventory-controller)
    - [OpenAPI Code Generation](#144-openapi-code-generation)
    - [Runtime OpenAPI Spec Usage](#145-runtime-openapi-spec-usage)
15. [StrictServerInterface Deep Dive and Testing](#15-strictserverinterface-deep-dive-and-testing) *(Added January 26, 2026)*
    - [StrictServerInterface Motivation and Creation](#151-strictserverinterface-motivation-and-creation)
    - [Why StrictServerInterface is Excellent for Testability](#152-why-strictserverinterface-is-excellent-for-testability)
    - [Existing Test Examples in the Codebase](#153-existing-test-examples-in-the-codebase)
    - [How to Test Your V11 Inventory Updates](#154-how-to-test-your-v11-inventory-updates)
    - [Testing Pattern Summary](#155-testing-pattern-summary)
    - [Key Files for Reference](#156-key-files-for-reference)
16. [Inventory Testing Assessment and Integration Testing Strategy](#16-inventory-testing-assessment-and-integration-testing-strategy) *(Added January 26, 2026)*
    - [Current Testing Status for Resource Server](#161-current-testing-status-for-resource-server)
    - [Types of Tests and Libraries Used](#162-types-of-tests-and-libraries-used)
    - [Integration Testing Status](#163-integration-testing-status)
    - [Testcontainers Assessment and Implementation Guide](#164-testcontainers-assessment-and-implementation-guide)
    - [Detailed Implementation Guidance](#165-detailed-implementation-guidance)
    - [Test Pattern Attribution](#166-test-pattern-attribution)
17. [E2E Test Framework Deep Dive (Brent Rowsell)](#17-e2e-test-framework-deep-dive-brent-rowsell) *(Added January 26, 2026)*
    - [E2E Test Framework Overview](#171-e2e-test-framework-overview)
    - [Brent's Initial Contributions](#172-brents-initial-contributions)
    - [Mock Hardware Plugin Server](#173-mock-hardware-plugin-server)
    - [E2E Test Architecture Summary](#174-e2e-test-architecture-summary)

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
└── Ingress: 
(routes to all servers)
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

## 12. Is Metal3 the Only Plugin?

### Question

> Is Metal3 our only plugin in the plugin architecture?

### Answer

**Yes, Metal3 is currently the only implemented plugin**, but the architecture is **designed for extensibility**.

#### Current State

```
hwmgr-plugins/
├── api/                    # Shared plugin API (OpenAPI specs)
├── cmd/                    # Generic HardwarePlugin manager
├── controller/             # Generic HardwarePlugin controller
│   └── utils/
│       └── constants.go    # Only Metal3HardwarePluginID defined
│
└── metal3/                 # ← ONLY implemented plugin
    ├── cmd/
    ├── controller/
    └── server/
```

```go
// hwmgr-plugins/controller/utils/constants.go
const HardwarePluginLabel = "clcm.openshift.io/hardware-plugin"

const (
    Metal3HardwarePluginID = "metal3-hwplugin"  // ← Only one!
)
```

#### Why Only Metal3?

**Metal3 (Bare Metal Operator)** is the standard for managing bare metal servers in the Kubernetes ecosystem, particularly in OpenShift. It's the primary integration point because:

1. **OpenShift uses Metal3** for all bare metal provisioning
2. **ACM (Advanced Cluster Management)** relies on Metal3 for managed clusters
3. **SiteConfig operator** expects `BareMetalHost` CRs from Metal3

#### Extensible Architecture

However, the code is **designed for multiple plugins**:

```go
// hwmgr-plugins/api/server/provisioning/server.go
type HardwarePluginServer struct {
    HubClient          client.Client
    Namespace          string
    HardwarePluginID   string      // ← Generic - not hardcoded
    ResourcePrefix     string
}
```

The filtering uses labels that allow multiple plugins to coexist:

```go
listOptions := client.MatchingLabels{
    hwmgrutils.HardwarePluginLabel: h.HardwarePluginID,  // Filter by plugin
}
```

#### Potential Future Plugins

The architecture could support additional hardware managers like:

| Potential Plugin | Hardware Manager |
|-----------------|------------------|
| `dell-hwplugin` | Dell OpenManage/iDRAC |
| `hpe-hwplugin` | HP iLO / OneView |
| `lenovo-hwplugin` | Lenovo XClarity |
| `redfish-hwplugin` | Generic Redfish BMC |

#### How to Add a New Plugin

If you needed to add a new plugin, you would:

1. **Add new constant**: `hwmgr-plugins/controller/utils/constants.go`
   ```go
   const (
       Metal3HardwarePluginID = "metal3-hwplugin"
       DellHardwarePluginID   = "dell-hwplugin"  // New
   )
   ```

2. **Create new directory**: `hwmgr-plugins/dell/`
   - `controller/` - Reconcilers for Dell-specific logic
   - `server/` - Implement the plugin API for Dell

3. **Register plugin CR**: Create `HardwarePlugin` CR pointing to the Dell API

#### Summary

| Question | Answer |
|----------|--------|
| Only implemented plugin? | **Metal3** |
| Extensible? | **Yes** - architecture supports multiple plugins |
| Why Metal3 only? | It's the standard for OpenShift bare metal |
| Can others be added? | **Yes** - by implementing the plugin API |

---

## 13. Corrections and Refinements (Based on Official Documentation Review)

> **Note:** After reviewing the official documentation under `docs/`, the following corrections and refinements were identified to align my analysis with the project's canonical documentation.

### 13.1 Pod Container Composition - Correction

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

### 13.2 Prerequisites - Correction

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

### 13.3 Storage Requirements - Correction

**Original Analysis:** `10Gi (configurable)`

**Corrected (from `docs/user-guide/prereqs.md`):**

> "Ensure a free PersistentVolume with at least **20 Gi** capacity is available for the operator's internal database"

### 13.4 Routes/Ingress - Addition

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

### 13.5 Resource Server Architecture - Key Design Pattern

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

### 13.6 Alarms Architecture - Key Concepts

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

### 13.7 Hardware Provisioning Flow - Enhanced Detail

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

### 13.8 ACM Observability Dependency - Critical for Alarms

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

### 13.9 Related Documentation Links

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

## 14. Server Pattern Deep Dive

### Question

> I have a new question, examine the servers (resource-server, cluster-server, ...). Explain the low level details for its creation: do they have dedicated dockerfiles? How are they started?

### Answer

### 14.1 Single Dockerfile Architecture

**All servers share ONE Dockerfile** located at the project root. There are no dedicated Dockerfiles per server.

```dockerfile
# Dockerfile (project root)
# Build the manager binary
FROM registry.hub.docker.com/library/golang:1.24 AS builder
# ...build steps...

# Build single binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -gcflags "${GOBUILD_GCFLAGS}" -mod=vendor -a

# Use distroless as minimal base image
FROM gcr.io/distroless/static:nonroot AS production
WORKDIR /
COPY --from=builder /workspace/oran-o2ims /usr/bin
USER 65532:65532

ENTRYPOINT ["/usr/bin/oran-o2ims"]
```

**Key insight**: The build produces a **single binary** called `oran-o2ims` that contains ALL server functionality. There are no separate images per server.

### 14.2 Single Binary with Cobra Subcommands

The main entry point at `main.go` registers all server commands using Cobra CLI:

```go
// main.go
func main() {
    toolBuilder := internal.NewTool().
        AddArgs(os.Args...).
        AddCommand(cmd.Start).                              // start controller-manager
        AddCommand(cmd.Version).
        AddCommand(hwpluginscmd.Start).                     // hardwareplugin-manager start
        AddCommand(metal3plugincmd.Start).                  // metal3-hardwareplugin-manager start
        AddCommand(alarmscmd.GetAlarmRootCmd).              // alarms-server serve
        AddCommand(clustercmd.GetClusterRootCmd).           // cluster-server serve
        AddCommand(inventorycmd.GetResourcesRootCmd).       // resource-server serve
        AddCommand(artifactscmd.GetArtifactsRootCmd).       // artifacts-server serve
        AddCommand(provisioningcmd.GetProvisioningRootCmd)  // provisioning-server serve
}
```

**Command Structure:**

| Command | Subcommand | Description |
|---------|------------|-------------|
| `resource-server` | `serve` | Start resource/inventory API server |
| `resource-server` | `migrate` | Run DB migrations |
| `cluster-server` | `serve` | Start cluster/deployment managers API |
| `alarms-server` | `serve` | Start monitoring/alarms API |
| `artifacts-server` | `serve` | Start artifacts download server |
| `provisioning-server` | `serve` | Start provisioning API |
| `hardwareplugin-manager` | `start` | Start HW plugin manager controller |
| `metal3-hardwareplugin-manager` | `start` | Start Metal3 plugin (hybrid) |
| `start` | `controller-manager` | Start main operator |

### 14.3 Deployment Creation by Inventory Controller

The **Inventory reconciler** (`internal/controllers/inventory_controller.go`) creates all server deployments **programmatically** - not from YAML files.

The `deployServer()` function builds Kubernetes Deployments dynamically:

```go
func (t *reconcilerTask) deployServer(ctx context.Context, serverName string) error {
    // Select the container image
    image := t.image
    if t.object.Spec.Image != nil {
        image = *t.object.Spec.Image
    }

    // Build the deployment's spec
    deploymentSpec := appsv1.DeploymentSpec{
        Replicas: k8sptr.To(int32(1)),
        Template: corev1.PodTemplateSpec{
            Spec: corev1.PodSpec{
                ServiceAccountName: serverName,
                Containers: []corev1.Container{
                    {
                        Name:    constants.ServerContainerName,
                        Image:   image,
                        Command: []string{constants.ManagerExec},  // /usr/bin/oran-o2ims
                        Args:    deploymentContainerArgs,           // ["resource-server", "serve", ...]
                    },
                },
            },
        },
    }

    // For servers with databases, add init container for migrations
    if ctlrutils.HasDatabase(serverName) {
        deploymentSpec.Template.Spec.InitContainers = []corev1.Container{
            {
                Name:    constants.MigrationContainerName,  // "migration"
                Image:   image,
                Command: []string{constants.ManagerExec},
                Args:    []string{serverName, "migrate"},   // e.g., ["resource-server", "migrate"]
            },
        }
    }
}
```

**Server Arguments** are defined in `internal/controllers/utils/constants.go`:

```go
var ResourceServerArgs = []string{
    constants.ResourceServerCmd,    // "resource-server"
    constants.ServeSubcommand,      // "serve"
    fmt.Sprintf("--api-listener-address=0.0.0.0:%d", constants.DefaultContainerPort),
    fmt.Sprintf("--tls-server-cert=%s/%s", constants.TLSServerMountPath, constants.TLSCertField),
    fmt.Sprintf("--tls-server-key=%s/%s", constants.TLSServerMountPath, constants.TLSKeyField),
}
```

### Server Startup Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                                    BUILD TIME                                                │
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│   Dockerfile ──────►  Single Binary: /usr/bin/oran-o2ims                                   │
│                              │                                                              │
│                              ├── "start controller-manager"  ──► operator                   │
│                              ├── "resource-server serve"     ──► resource API               │
│                              ├── "cluster-server serve"      ──► cluster API                │
│                              ├── "alarms-server serve"       ──► alarms API                 │
│                              ├── "artifacts-server serve"    ──► artifacts API              │
│                              ├── "provisioning-server serve" ──► provisioning API           │
│                              └── "metal3-hardwareplugin-manager start" ──► metal3           │
└─────────────────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                                    RUNTIME                                                   │
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│   Inventory CR created                                                                      │
│         │                                                                                   │
│         ▼                                                                                   │
│   controller-manager (Inventory Reconciler)                                                 │
│         │                                                                                   │
│         ├──► deployServer("resource-server") ──► Creates Deployment:                        │
│         │        Container: image = quay.io/.../oran-o2ims:latest                          │
│         │        Command:   ["/usr/bin/oran-o2ims"]                                         │
│         │        Args:      ["resource-server", "serve", "--api-listener-address=...", ...]│
│         │        InitContainer: ["resource-server", "migrate"]  (if HasDatabase)           │
│         │                                                                                   │
│         ├──► deployServer("cluster-server")  ──► Similar pattern                           │
│         ├──► deployServer("alarms-server")   ──► Similar pattern                           │
│         ├──► deployServer("artifacts-server")──► Similar pattern (no InitContainer)        │
│         └──► deployServer("provisioning-server") ──► Similar (no InitContainer)            │
└─────────────────────────────────────────────────────────────────────────────────────────────┘
```

### Key Takeaways

| Aspect | Details |
|--------|---------|
| **Dockerfiles** | **1 shared Dockerfile** - all servers use the same image |
| **Binary** | **1 binary** (`oran-o2ims`) with **Cobra subcommands** |
| **Deployment creation** | **Programmatic** via `deployServer()` in Inventory controller |
| **Server differentiation** | **Command-line arguments** determine which server code runs |
| **DB migrations** | **Init container** runs `[server-name] migrate` before main container |
| **TLS** | Certificates mounted from OpenShift service-serving-cert secrets |
| **Code location** | Each server in `internal/service/<name>/` with `cmd/`, `api/`, `serve.go` |

---

### Question

> Do every server load its own openapi file for api validations? Or must we manually (make generate) the golang code for those servers to use?

### Answer

### 14.4 OpenAPI Code Generation

**YES to both** - Each server has its own OpenAPI spec AND requires code generation.

Each server:
1. **Has its own `openapi.yaml` file** - specific to that API
2. **Requires code generation** (`go generate` or `make go-generate`) to create the Go code
3. **Embeds the OpenAPI spec at build time** - compressed and base64-encoded into the generated `.go` file
4. **Loads the embedded spec at runtime** via `GetSwagger()` for request validation

#### OpenAPI Spec Organization

```
internal/service/
├── common/api/
│   ├── openapi.yaml              ← Shared types (ProblemDetails, etc.)
│   ├── tools/
│   │   ├── generate.go           ← //go:generate directive
│   │   └── oapi-codegen.yaml     ← Code generation config
│   └── generated/
│       └── common.generated.go   ← Generated types + embedded spec
│
├── resources/api/                (Inventory API - /o2ims-infrastructureInventory)
│   ├── openapi.yaml              ← ResourcePool, ResourceType, Resource, etc.
│   ├── openapi-overlay.yaml      ← Modifications/patches to spec
│   ├── tools/
│   │   ├── generate.go
│   │   └── oapi-codegen.yaml
│   └── generated/
│       └── inventory.generated.go  ← ~3650 lines of generated code
│
├── cluster/api/                   (Cluster API)
├── alarms/api/                    (Monitoring API)
├── artifacts/api/                 (Artifacts API)
└── provisioning/api/              (Provisioning API)
```

#### Code Generation Configuration

Each server has its own `oapi-codegen.yaml` config:

```yaml
# internal/service/resources/api/tools/oapi-codegen.yaml
package: generated
output: ../generated/inventory.generated.go

generate:
  std-http-server: true      # Generate Go 1.22 stdlib router
  client: false              # No HTTP client (server-only)
  models: true               # Generate struct types
  embedded-spec: true        # Embed OpenAPI spec in binary ← KEY!
  strict-server: true        # Type-safe request/response handlers

import-mapping:
  ../../common/api/openapi.yaml: "github.com/openshift-kni/oran-o2ims/internal/service/common/api/generated"
```

#### The `generate.go` Trigger File

```go
// internal/service/resources/api/tools/generate.go
package tools

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config oapi-codegen.yaml ../openapi.yaml
```

#### How to Regenerate Code

```bash
# Using go generate (processes all //go:generate directives)
go generate ./...

# Or using the Makefile target
make go-generate

# Regenerate a specific service
cd internal/service/resources/api/tools && go generate
```

#### Summary Table

| Server | OpenAPI Spec Location | Generated File | Size |
|--------|----------------------|----------------|------|
| **resource-server** | `internal/service/resources/api/openapi.yaml` | `inventory.generated.go` | ~3650 lines |
| **cluster-server** | `internal/service/cluster/api/openapi.yaml` | `cluster.generated.go` | ~3250 lines |
| **alarms-server** | `internal/service/alarms/api/openapi.yaml` | `alarms.generated.go` | ~2500 lines |
| **artifacts-server** | `internal/service/artifacts/api/openapi.yaml` | `artifacts.generated.go` | ~1050 lines |
| **provisioning-server** | `internal/service/provisioning/api/openapi.yaml` | `provisioning.generated.go` | ~1350 lines |
| **common (shared)** | `internal/service/common/api/openapi.yaml` | `common.generated.go` | ~290 lines |

---

### Question

> Then the openapi file is not directly used by any server (it is not read), only the generated go code is used. Am I correct?

### Answer

**Yes, 100% correct!**

The `openapi.yaml` files are **never read at runtime**. They are only used at **code generation time**.

```
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                                    BUILD TIME                                                │
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│   openapi.yaml ────► oapi-codegen ────► *.generated.go                                     │
│        │                                      │                                             │
│   (YAML file on disk)                    (Go source code with                               │
│                                           embedded spec as                                  │
│                                           base64+gzip string)                              │
│                                                                                             │
│   This file is NOT                       This IS compiled                                   │
│   included in the                        into the binary                                    │
│   final binary                                                                              │
└─────────────────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                                    RUNTIME                                                   │
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│   /usr/bin/oran-o2ims                                                                       │
│         │                                                                                   │
│         ├── Contains compiled *.generated.go code                                           │
│         │                                                                                   │
│         └── GetSwagger() decodes embedded spec from Go string constant                      │
│                    │                                                                        │
│                    ▼                                                                        │
│              openapi3.T (in-memory object)                                                  │
│                    │                                                                        │
│                    └── Used by middleware.OpenAPIValidation()                               │
│                                                                                             │
│   NO file I/O needed!                                                                       │
│   NO openapi.yaml file required in container!                                               │
└─────────────────────────────────────────────────────────────────────────────────────────────┘
```

**Key Implications:**

| Aspect | Implication |
|--------|-------------|
| **Container image** | Does NOT need `openapi.yaml` files - they're embedded in the binary |
| **Runtime dependencies** | Zero file I/O for OpenAPI specs |
| **Spec changes** | Require regenerating code (`go generate`) and rebuilding the binary |
| **Validation** | Happens against the spec that was embedded at build time |

---

### Question

> Does the generated.go show the go structures or just an encoded version of the openapi?

### Answer

**Both!** The generated file contains:

1. ✅ **Readable Go structs** for all OpenAPI schemas (models)
2. ✅ **HTTP handler interfaces** (type-safe server interface)
3. ✅ **Router/handler code** (request parsing, response serialization)
4. ✅ **AND** the encoded OpenAPI spec (for runtime validation)

#### What's Inside `inventory.generated.go` (~3650 lines)

```
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                          inventory.generated.go STRUCTURE                                    │
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│  Lines 1-30:      Package declaration, imports                                              │
│                                                                                             │
│  Lines 31-850:    GO STRUCT TYPES (readable!)                                              │
│                   - DeploymentManager                                                       │
│                   - OCloudInfo                                                              │
│                   - Resource                                                                │
│                   - ResourcePool                                                            │
│                   - ResourceType                                                            │
│                   - Subscription                                                            │
│                                                                                             │
│  Lines 850-1200:  Request/Response parameter types                                          │
│                   - GetResourcePoolParams                                                   │
│                   - GetResourcesParams                                                      │
│                                                                                             │
│  Lines 1200-1400: StrictServerInterface (handler interface)                                │
│                   - Your server.go implements this                                          │
│                                                                                             │
│  Lines 1400-2800: HTTP handler wrapper code                                                 │
│                   - Request parsing                                                         │
│                   - Path parameter extraction                                               │
│                   - Query parameter binding                                                 │
│                   - Response serialization                                                  │
│                                                                                             │
│  Lines 2800-3450: Typed response implementations                                            │
│                   - GetResourcePool200JSONResponse                                          │
│                   - GetResourcePool404ApplicationProblemPlusJSONResponse                    │
│                                                                                             │
│  Lines 3456-3650: ENCODED OPENAPI SPEC (base64+gzip)                                       │
│                   - var swaggerSpec = []string{...}                                         │
│                   - GetSwagger() function                                                   │
└─────────────────────────────────────────────────────────────────────────────────────────────┘
```

#### Examples of Readable Go Structs

```go
// DeploymentManager Information about a deployment manager.
type DeploymentManager struct {
    Capabilities        map[string]string      `json:"capabilities"`
    Capacity            map[string]string      `json:"capacity"`
    DeploymentManagerId openapi_types.UUID     `json:"deploymentManagerId"`
    Description         string                 `json:"description"`
    Extensions          *map[string]interface{} `json:"extensions,omitempty"`
    Name                string                 `json:"name"`
    OCloudId            openapi_types.UUID     `json:"oCloudId"`
    ServiceUri          string                 `json:"serviceUri"`
    SupportedLocations  []string               `json:"supportedLocations"`
}

// OCloudInfo defines model for OCloudInfo.
type OCloudInfo struct {
    Description   string                  `json:"description"`
    Extensions    *map[string]interface{} `json:"extensions,omitempty"`
    GlobalcloudId openapi_types.UUID      `json:"globalcloudId"`
    Name          string                  `json:"name"`
    OCloudId      openapi_types.UUID      `json:"oCloudId"`
    ServiceUri    string                  `json:"serviceUri"`
}
```

#### Handler Interface (also readable Go)

```go
// StrictServerInterface represents all server handlers.
type StrictServerInterface interface {
    // Get information about the O-Cloud
    GetCloudInfo(ctx context.Context, request GetCloudInfoRequestObject) (GetCloudInfoResponseObject, error)

    // Get list of resource pools
    GetResourcePools(ctx context.Context, request GetResourcePoolsRequestObject) (GetResourcePoolsResponseObject, error)

    // Get a specific resource pool
    GetResourcePool(ctx context.Context, request GetResourcePoolRequestObject) (GetResourcePoolResponseObject, error)
    // ... more handlers
}
```

#### Summary

| Content | Readable? | Purpose |
|---------|-----------|---------|
| **Go struct types** | ✅ Yes | Data models (ResourcePool, Resource, etc.) |
| **Handler interface** | ✅ Yes | Contract your `server.go` implements |
| **Request/response types** | ✅ Yes | Type-safe API boundaries |
| **Router registration** | ✅ Yes | HTTP routing code |
| **Encoded OpenAPI spec** | ❌ No (base64) | Runtime request/response validation |

The encoded spec is only ~5% of the file - the rest is **fully readable, well-documented Go code**.

---

### Question

> The encoded spec is only used to validate, am I correct?

### Answer

### 14.5 Runtime OpenAPI Spec Usage

**Yes, primarily for validation** - but also for **response filtering**.

#### Two Uses of the Embedded OpenAPI Spec

```go
// internal/service/resources/serve.go
func Serve(config *api.ResourceServerConfig) error {
    // Load embedded spec
    swagger, err := generated.GetSwagger()

    // 1️⃣ SELF-VALIDATION: Validate the spec itself is correct
    if err := swagger.Validate(context.Background(),
        openapi3.EnableSchemaDefaultsValidation(),
        openapi3.EnableSchemaFormatValidation(),
        openapi3.EnableSchemaPatternValidation(),
        openapi3.EnableExamplesValidation(),
        openapi3.ProhibitExtensionsWithRef(),
    ); err != nil {
        return fmt.Errorf("failed validate swagger: %w", err)
    }

    // 2️⃣ RESPONSE FILTERING: Build filter adapter from spec
    filterAdapter, err := middleware.NewFilterAdapterFromSwagger(logger, swagger)

    opt := generated.StdHTTPServerOptions{
        BaseRouter: baseRouter,
        Middlewares: []generated.MiddlewareFunc{
            // 3️⃣ REQUEST VALIDATION: Validate incoming requests against spec
            middleware.OpenAPIValidation(swagger),

            // 4️⃣ RESPONSE FILTERING: Apply ?fields, ?exclude_fields params
            middleware.ResponseFilter(filterAdapter),

            authz,
            authn,
            middleware.LogDuration(),
        },
    }
}
```

#### Summary of Embedded Spec Usage

| Use | What It Does |
|-----|--------------|
| **1. Self-validation** | Validates the OpenAPI spec itself is well-formed at startup |
| **2. Request validation** | `OpenAPIValidation(swagger)` - Validates incoming HTTP requests match the spec |
| **3. Response filtering** | `NewFilterAdapterFromSwagger()` - Supports O-RAN `?fields=`, `?exclude_fields=`, `?filter=` query params |

#### What Uses the Go Structs Instead

| Task | Uses |
|------|------|
| JSON serialization/deserialization | Go structs (`ResourcePool`, `Resource`, etc.) |
| Handler type safety | `StrictServerInterface` |
| Database model conversion | Go structs + your converter functions |
| Business logic | Go structs in `server.go` |

#### Visual Summary

```
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                           WHAT USES WHAT AT RUNTIME                                          │
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                             │
│   ENCODED OPENAPI SPEC (GetSwagger())                                                       │
│   └── Used by:                                                                              │
│       ├── middleware.OpenAPIValidation()  → Validate request params, body, types           │
│       └── middleware.NewFilterAdapterFromSwagger() → Know field names for ?fields=         │
│                                                                                             │
│   GO STRUCTS (ResourcePool, Resource, etc.)                                                │
│   └── Used by:                                                                              │
│       ├── json.Marshal/Unmarshal → Serialize/deserialize JSON                              │
│       ├── Handler functions → Type-safe request/response handling                          │
│       ├── Database layer → Convert to/from DB models                                        │
│       └── Business logic → All your actual code                                            │
│                                                                                             │
└─────────────────────────────────────────────────────────────────────────────────────────────┘
```

The encoded spec is **only for validation and field introspection**, not for the actual data handling. The Go structs do all the real work!

---

## 15. StrictServerInterface Deep Dive and Testing

*Added January 26, 2026*

### Question

> You mentioned that it is possible to Mock `StrictServerInterface` easily. I am interested in diving deeper:
>
> 1. Explain the StrictServerInterface motivation, how it was created.
> 2. Explain why it is good for testability.
> 3. Explain how can I use it to test my new inventory update according to v11 spec. Is there any test examples the current code use to leverage it? Could you provide a concise example of a possible test using that interface?

### Answer

#### 15.1 StrictServerInterface Motivation and Creation

##### The Problem with Plain ServerInterface

When `oapi-codegen` generates code with `std-http-server: true`, it creates a `ServerInterface` that uses raw HTTP primitives:

```go
// ServerInterface - Low-level HTTP handlers (NOT what we implement)
type ServerInterface interface {
    // Methods deal with raw HTTP
    GetResourcePool(w http.ResponseWriter, r *http.Request, resourcePoolId ResourcePoolId)
    GetResources(w http.ResponseWriter, r *http.Request, params GetResourcesParams)
    // ...
}
```

This interface is **difficult to implement** because:
1. You must manually parse path parameters, query parameters, and request bodies
2. You must manually write HTTP responses with correct status codes and content types
3. Error handling is mixed with business logic
4. Testing requires constructing `http.Request` and `httptest.ResponseRecorder`

##### The Solution: StrictServerInterface

When you add `strict-server: true` to the oapi-codegen config (as this project does), it generates a **cleaner interface**:

```go
// StrictServerInterface - Clean business logic handlers (what we implement!)
type StrictServerInterface interface {
    // Methods use strongly-typed Request/Response objects
    GetResourcePool(ctx context.Context, request GetResourcePoolRequestObject) (GetResourcePoolResponseObject, error)
    GetResources(ctx context.Context, request GetResourcesRequestObject) (GetResourcesResponseObject, error)
    // ...
}
```

##### How It Works - The Adapter Pattern

The generated code creates a `strictHandler` struct that **adapts** between the two interfaces:

```
┌────────────────────────────────────────────────────────────────────────────────────────────┐
│                        STRICT SERVER ADAPTER PATTERN                                        │
├────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                            │
│  HTTP Request                                                                              │
│       │                                                                                    │
│       ▼                                                                                    │
│  ┌─────────────────┐                                                                       │
│  │  ServerInterface│ ◄── Generated wrapper that Go's http.ServeMux calls                   │
│  │  (raw HTTP)     │                                                                       │
│  └────────┬────────┘                                                                       │
│           │                                                                                │
│           │ strictHandler.GetResourcePool(w, r, resourcePoolId)                            │
│           │   1. Parses path params → GetResourcePoolRequestObject{ResourcePoolId: uuid}   │
│           │   2. Calls your implementation                                                 │
│           │   3. Handles response serialization                                            │
│           │                                                                                │
│           ▼                                                                                │
│  ┌──────────────────────────┐                                                              │
│  │ StrictServerInterface    │ ◄── YOUR implementation (ResourceServer)                     │
│  │ (typed Go objects)       │                                                              │
│  └──────────────────────────┘                                                              │
│           │                                                                                │
│           │ Returns GetResourcePool200JSONResponse{...} or                                 │
│           │         GetResourcePool404ApplicationProblemPlusJSONResponse{...}              │
│           │                                                                                │
│           ▼                                                                                │
│  strictHandler serializes response → HTTP Response                                         │
│                                                                                            │
└────────────────────────────────────────────────────────────────────────────────────────────┘
```

##### The Configuration That Enables This

From `internal/service/resources/api/tools/oapi-codegen.yaml`:

```yaml
generate:
  std-http-server: true   # Generates ServerInterface
  strict-server: true     # ALSO generates StrictServerInterface + adapter
  models: true            # Generate Go structs
```

##### Wiring It Together

In `internal/service/resources/serve.go`:

```go
// Your server implements StrictServerInterface
server := api.ResourceServer{
    Config: config,
    Repo:   repository,
    // ...
}

// Wrap it with the strict handler adapter
serverStrictHandler := generated.NewStrictHandlerWithOptions(&server, nil,
    generated.StrictHTTPServerOptions{
        RequestErrorHandlerFunc:  middleware.GetOranReqErrFunc(),
        ResponseErrorHandlerFunc: middleware.GetOranRespErrFunc(),
    },
)

// Register with HTTP router (serverStrictHandler implements ServerInterface)
generated.HandlerWithOptions(serverStrictHandler, opt)
```

#### 15.2 Why StrictServerInterface is Excellent for Testability

##### 1. Pure Business Logic Testing (No HTTP Ceremony)

```go
// Without StrictServerInterface - testing is painful
func TestGetResourcePool_OldWay(t *testing.T) {
    req := httptest.NewRequest("GET", "/resourcePools/123", nil)
    w := httptest.NewRecorder()
    
    server.GetResourcePool(w, req, uuid.MustParse("123..."))
    
    // Must parse response body
    var result api.ResourcePool
    json.Unmarshal(w.Body.Bytes(), &result)
    // Must check status code
    assert.Equal(t, 200, w.Code)
}

// With StrictServerInterface - testing is clean
func TestGetResourcePool_StrictWay(t *testing.T) {
    resp, err := server.GetResourcePool(ctx, api.GetResourcePoolRequestObject{
        ResourcePoolId: testUUID,
    })
    
    // Type-safe response!
    assert.IsType(t, api.GetResourcePool200JSONResponse{}, resp)
    assert.Equal(t, testUUID, resp.(api.GetResourcePool200JSONResponse).ResourcePoolId)
}
```

##### 2. Type-Safe Response Assertions

Each possible response has its own Go type:

```go
// These are DIFFERENT types - compiler helps you!
GetResourcePool200JSONResponse{}                           // Success
GetResourcePool404ApplicationProblemPlusJSONResponse{}     // Not found
GetResourcePool500ApplicationProblemPlusJSONResponse{}     // Server error
```

You can use `BeAssignableToTypeOf()` in tests to verify the exact response type.

##### 3. No HTTP Infrastructure Required

You don't need:
- HTTP server running
- Network sockets
- `httptest.NewRecorder()`
- URL parsing

Just call methods directly with Go objects.

##### 4. Easy Mocking of Dependencies

The real power comes from mocking the **repository layer**, not the server itself:

```
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                           TESTING ARCHITECTURE                                              │
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                             │
│  TEST CODE                                                                                  │
│       │                                                                                     │
│       ▼                                                                                     │
│  ┌──────────────────────────────────┐                                                       │
│  │  ResourceServer                   │ ◄── Real server struct                               │
│  │  (implements StrictServerInterface│                                                      │
│  │                                   │                                                      │
│  │  Repo: MockRepositoryInterface   │ ◄── MOCK injected here!                              │
│  └──────────────────────────────────┘                                                       │
│                                                                                             │
│  The mock repository controls what the server "sees" from the database                      │
│                                                                                             │
└─────────────────────────────────────────────────────────────────────────────────────────────┘
```

#### 15.3 Existing Test Examples in the Codebase

The project already has excellent test examples. Let me show you the patterns:

##### Example 1: ClusterServer Tests

From `internal/service/cluster/api/server_test.go`:

```go
var _ = Describe("Cluster Server", func() {
    var (
        ctrl     *gomock.Controller
        mockRepo *generated.MockRepositoryInterface  // Generated mock!
        server   *ClusterServer
        ctx      context.Context
        testUUID uuid.UUID
    )

    BeforeEach(func() {
        ctrl = gomock.NewController(GinkgoT())
        mockRepo = generated.NewMockRepositoryInterface(ctrl)
        server = &ClusterServer{
            Repo: mockRepo,  // Inject mock
        }
        ctx = context.Background()
        testUUID = uuid.New()
    })

    Describe("GetNodeClusterTypeAlarmDictionary", func() {
        When("repository returns error", func() {
            It("returns internal server error", func() {
                // ARRANGE: Set up mock expectation
                mockRepo.EXPECT().
                    GetNodeClusterTypeAlarmDictionary(ctx, testUUID).
                    Return(nil, fmt.Errorf("db error"))

                // ACT: Call the method directly (no HTTP!)
                resp, err := server.GetNodeClusterTypeAlarmDictionary(ctx,
                    apigenerated.GetNodeClusterTypeAlarmDictionaryRequestObject{
                        NodeClusterTypeId: testUUID,
                    })

                // ASSERT: Type-safe checks
                Expect(err).To(BeNil())
                Expect(resp).To(BeAssignableToTypeOf(
                    apigenerated.GetNodeClusterTypeAlarmDictionary500ApplicationProblemPlusJSONResponse{}))
                Expect(resp.(apigenerated.GetNodeClusterTypeAlarmDictionary500ApplicationProblemPlusJSONResponse).
                    Status).To(Equal(http.StatusInternalServerError))
            })
        })

        When("repository does not have the alarm dictionary", func() {
            It("returns 404 not found response", func() {
                mockRepo.EXPECT().
                    GetNodeClusterTypeAlarmDictionary(ctx, testUUID).
                    Return([]models.AlarmDictionary{}, nil)  // Empty = not found

                resp, err := server.GetNodeClusterTypeAlarmDictionary(ctx,
                    apigenerated.GetNodeClusterTypeAlarmDictionaryRequestObject{
                        NodeClusterTypeId: testUUID,
                    })

                Expect(err).To(BeNil())
                Expect(resp).To(BeAssignableToTypeOf(
                    apigenerated.GetNodeClusterTypeAlarmDictionary404ApplicationProblemPlusJSONResponse{}))
            })
        })

        When("alarm dictionary and definitions are found", func() {
            It("returns 200 OK", func() {
                alarmDefinitionUUID := uuid.New()
                
                // First call returns dictionary
                mockRepo.EXPECT().
                    GetNodeClusterTypeAlarmDictionary(ctx, testUUID).
                    Return([]models.AlarmDictionary{
                        {AlarmDictionaryID: testUUID},
                    }, nil)
                
                // Second call returns definitions
                mockRepo.EXPECT().
                    GetAlarmDefinitionsByAlarmDictionaryID(ctx, testUUID).
                    Return([]models.AlarmDefinition{
                        {AlarmDefinitionID: alarmDefinitionUUID},
                    }, nil)

                resp, err := server.GetNodeClusterTypeAlarmDictionary(ctx,
                    apigenerated.GetNodeClusterTypeAlarmDictionaryRequestObject{
                        NodeClusterTypeId: testUUID,
                    })

                Expect(err).To(BeNil())
                Expect(resp).To(BeAssignableToTypeOf(
                    apigenerated.GetNodeClusterTypeAlarmDictionary200JSONResponse{}))
                // Type assertion gives you full access to response fields!
                okResp := resp.(apigenerated.GetNodeClusterTypeAlarmDictionary200JSONResponse)
                Expect(okResp.AlarmDictionaryId).To(Equal(testUUID))
                Expect(okResp.AlarmDefinition).To(HaveLen(1))
            })
        })
    })
})
```

##### Example 2: AlarmsServer Tests

From `internal/service/alarms/api/server_test.go`:

```go
var _ = Describe("AlarmsServer", func() {
    var (
        ctrl     *gomock.Controller
        mockRepo *generated.MockAlarmRepositoryInterface
        server   *api.AlarmsServer
        ctx      context.Context
        testUUID uuid.UUID
    )

    BeforeEach(func() {
        ctrl = gomock.NewController(GinkgoT())
        mockRepo = generated.NewMockAlarmRepositoryInterface(ctrl)
        server = &api.AlarmsServer{AlarmsRepository: mockRepo}
        ctx = context.Background()
        testUUID = uuid.New()
    })

    Describe("GetAlarm", func() {
        When("alarm not found", func() {
            It("returns 404 response", func() {
                mockRepo.EXPECT().
                    GetAlarmEventRecord(ctx, testUUID).
                    Return(nil, svcutils.ErrNotFound)  // Special sentinel error

                resp, err := server.GetAlarm(ctx, alarmapi.GetAlarmRequestObject{
                    AlarmEventRecordId: testUUID,
                })

                Expect(err).NotTo(HaveOccurred())
                problemResp := resp.(alarmapi.GetAlarm404ApplicationProblemPlusJSONResponse)
                Expect(problemResp.Status).To(Equal(http.StatusNotFound))
            })
        })

        When("repository is unavailable", func() {
            It("returns error", func() {
                mockRepo.EXPECT().
                    GetAlarmEventRecord(ctx, testUUID).
                    Return(nil, fmt.Errorf("db error"))

                resp, err := server.GetAlarm(ctx, alarmapi.GetAlarmRequestObject{
                    AlarmEventRecordId: testUUID,
                })

                Expect(err).To(HaveOccurred())  // Non-recoverable errors
                Expect(resp).To(BeNil())
            })
        })
    })
})
```

#### 15.4 How to Test Your V11 Inventory Updates

##### Step 1: Define Repository Interface (if not exists)

The resource server currently uses a concrete `*repo.ResourcesRepository` struct. For testability, you should define an interface:

```go
// internal/service/resources/db/repo/repository_interface.go
package repo

//go:generate mockgen -source=repository_interface.go -destination=generated/mock_repo.generated.go -package=generated

type ResourcesRepositoryInterface interface {
    GetResourcePools(ctx context.Context) ([]models.ResourcePool, error)
    GetResourcePool(ctx context.Context, id uuid.UUID) (*models.ResourcePool, error)
    GetResources(ctx context.Context, poolId uuid.UUID) ([]models.Resource, error)
    GetResource(ctx context.Context, id uuid.UUID) (*models.Resource, error)
    // Add all methods you need...
}
```

##### Step 2: Update Server to Use Interface

```go
// internal/service/resources/api/server.go
type ResourceServer struct {
    Config                   *ResourceServerConfig
    Info                     api.OCloudInfo
    Repo                     repo.ResourcesRepositoryInterface  // Interface, not concrete!
    SubscriptionEventHandler notifier.SubscriptionEventHandler
}
```

##### Step 3: Generate Mocks

```bash
cd internal/service/resources/db/repo
go generate ./...
# Creates generated/mock_repo.generated.go
```

##### Step 4: Write Your Tests

Here's a **complete example** for testing a new v11 endpoint:

```go
// internal/service/resources/api/server_test.go
package api_test

import (
    "context"
    "fmt"
    "net/http"
    "testing"

    "github.com/google/uuid"
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    "go.uber.org/mock/gomock"

    "github.com/openshift-kni/oran-o2ims/internal/service/resources/api"
    apigenerated "github.com/openshift-kni/oran-o2ims/internal/service/resources/api/generated"
    "github.com/openshift-kni/oran-o2ims/internal/service/resources/db/models"
    "github.com/openshift-kni/oran-o2ims/internal/service/resources/db/repo/generated"
    svcutils "github.com/openshift-kni/oran-o2ims/internal/service/common/utils"
)

var _ = Describe("ResourceServer", func() {
    var (
        ctrl     *gomock.Controller
        mockRepo *generated.MockResourcesRepositoryInterface
        server   *api.ResourceServer
        ctx      context.Context
    )

    BeforeEach(func() {
        ctrl = gomock.NewController(GinkgoT())
        mockRepo = generated.NewMockResourcesRepositoryInterface(ctrl)
        server = &api.ResourceServer{
            Repo: mockRepo,
            Info: apigenerated.OCloudInfo{
                OCloudId: uuid.New(),
            },
        }
        ctx = context.Background()
    })

    AfterEach(func() {
        ctrl.Finish()
    })

    // ==========================================================================
    // EXAMPLE: Testing GetResourcePool (existing endpoint)
    // ==========================================================================
    Describe("GetResourcePool", func() {
        var testPoolID uuid.UUID

        BeforeEach(func() {
            testPoolID = uuid.New()
        })

        When("resource pool exists", func() {
            It("returns 200 with the pool data", func() {
                // ARRANGE
                expectedPool := &models.ResourcePool{
                    ResourcePoolID: testPoolID,
                    Name:           "test-pool",
                    Description:    "Test pool description",
                    OCloudID:       uuid.New(),
                }
                mockRepo.EXPECT().
                    GetResourcePool(ctx, testPoolID).
                    Return(expectedPool, nil)

                // ACT
                resp, err := server.GetResourcePool(ctx,
                    apigenerated.GetResourcePoolRequestObject{
                        ResourcePoolId: testPoolID,
                    })

                // ASSERT
                Expect(err).ToNot(HaveOccurred())
                Expect(resp).To(BeAssignableToTypeOf(apigenerated.GetResourcePool200JSONResponse{}))
                
                okResp := resp.(apigenerated.GetResourcePool200JSONResponse)
                Expect(okResp.ResourcePoolId).To(Equal(testPoolID))
                Expect(okResp.Name).To(Equal("test-pool"))
            })
        })

        When("resource pool does not exist", func() {
            It("returns 404 with problem details", func() {
                // ARRANGE
                mockRepo.EXPECT().
                    GetResourcePool(ctx, testPoolID).
                    Return(nil, svcutils.ErrNotFound)

                // ACT
                resp, err := server.GetResourcePool(ctx,
                    apigenerated.GetResourcePoolRequestObject{
                        ResourcePoolId: testPoolID,
                    })

                // ASSERT
                Expect(err).ToNot(HaveOccurred())
                Expect(resp).To(BeAssignableToTypeOf(
                    apigenerated.GetResourcePool404ApplicationProblemPlusJSONResponse{}))
                
                notFoundResp := resp.(apigenerated.GetResourcePool404ApplicationProblemPlusJSONResponse)
                Expect(notFoundResp.Status).To(Equal(http.StatusNotFound))
                Expect(notFoundResp.Detail).To(ContainSubstring("not found"))
            })
        })

        When("database returns an error", func() {
            It("returns 500 with problem details", func() {
                // ARRANGE
                mockRepo.EXPECT().
                    GetResourcePool(ctx, testPoolID).
                    Return(nil, fmt.Errorf("connection refused"))

                // ACT
                resp, err := server.GetResourcePool(ctx,
                    apigenerated.GetResourcePoolRequestObject{
                        ResourcePoolId: testPoolID,
                    })

                // ASSERT
                Expect(err).ToNot(HaveOccurred())
                Expect(resp).To(BeAssignableToTypeOf(
                    apigenerated.GetResourcePool500ApplicationProblemPlusJSONResponse{}))
            })
        })
    })

    // ==========================================================================
    // EXAMPLE: Testing a NEW v11 endpoint (hypothetical)
    // ==========================================================================
    Describe("GetResourcePoolCapacity (v11 new endpoint)", func() {
        // Assuming v11 adds a new endpoint: GET /resourcePools/{id}/capacity
        
        var testPoolID uuid.UUID

        BeforeEach(func() {
            testPoolID = uuid.New()
        })

        When("resource pool exists and has capacity info", func() {
            It("returns 200 with capacity data", func() {
                // ARRANGE - Mock returns pool with capacity extensions
                mockRepo.EXPECT().
                    GetResourcePool(ctx, testPoolID).
                    Return(&models.ResourcePool{
                        ResourcePoolID: testPoolID,
                        Extensions: map[string]interface{}{
                            "capacity": map[string]interface{}{
                                "cpu":    "1000",
                                "memory": "2048Gi",
                            },
                        },
                    }, nil)

                // ACT
                resp, err := server.GetResourcePoolCapacity(ctx,
                    apigenerated.GetResourcePoolCapacityRequestObject{
                        ResourcePoolId: testPoolID,
                    })

                // ASSERT
                Expect(err).ToNot(HaveOccurred())
                Expect(resp).To(BeAssignableToTypeOf(
                    apigenerated.GetResourcePoolCapacity200JSONResponse{}))
                
                okResp := resp.(apigenerated.GetResourcePoolCapacity200JSONResponse)
                Expect(okResp.Capacity["cpu"]).To(Equal("1000"))
            })
        })
    })
})

// Don't forget the test suite setup!
func TestResourceServer(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Resource Server Suite")
}
```

##### Step 5: Run Your Tests

```bash
# Run all tests in the package
cd internal/service/resources/api
go test -v ./...

# Run with Ginkgo for better output
ginkgo -v ./...

# Run specific test
ginkgo -v --focus="GetResourcePool" ./...
```

#### 15.5 Testing Pattern Summary

| Layer | What to Test | How to Test |
|-------|--------------|-------------|
| **StrictServerInterface implementation** | Business logic | Mock repository, call methods directly |
| **Repository** | SQL queries | Integration tests with test DB |
| **Models transformation** | ToModel/FromModel | Unit tests with static data |
| **OpenAPI validation** | Request/response format | Integration tests via HTTP |

#### 15.6 Key Files for Reference

| Purpose | File |
|---------|------|
| ClusterServer tests | `internal/service/cluster/api/server_test.go` |
| AlarmsServer tests | `internal/service/alarms/api/server_test.go` |
| Mock generation directive | `internal/service/cluster/db/repo/repository_interface.go` |
| Generated mock | `internal/service/cluster/db/repo/generated/mock_repo.generated.go` |
| Test suite setup | `internal/service/cluster/api/suite_test.go` |

#### Visual Summary

```
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                    STRICTSERVERINTERFACE TESTING WORKFLOW                                   │
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                             │
│  1. DEFINE INTERFACE                                                                        │
│     repository_interface.go                                                                 │
│     └── //go:generate mockgen ...                                                           │
│                                                                                             │
│  2. GENERATE MOCK                                                                           │
│     $ go generate ./...                                                                     │
│     └── Creates mock_repo.generated.go                                                      │
│                                                                                             │
│  3. WRITE TEST                                                                              │
│     ┌─────────────────────────────────────────────────────────────────────────────────┐     │
│     │  BeforeEach:                                                                    │     │
│     │    mockRepo = generated.NewMockRepositoryInterface(ctrl)                        │     │
│     │    server = &ResourceServer{Repo: mockRepo}                                     │     │
│     │                                                                                 │     │
│     │  It("returns 200..."):                                                          │     │
│     │    mockRepo.EXPECT().GetResourcePool(ctx, id).Return(pool, nil)                 │     │
│     │    resp, err := server.GetResourcePool(ctx, request)                            │     │
│     │    Expect(resp).To(BeAssignableToTypeOf(Get...200JSONResponse{}))               │     │
│     └─────────────────────────────────────────────────────────────────────────────────┘     │
│                                                                                             │
│  4. RUN                                                                                     │
│     $ ginkgo -v ./...                                                                       │
│                                                                                             │
└─────────────────────────────────────────────────────────────────────────────────────────────┘
```

The `StrictServerInterface` pattern transforms API testing from "HTTP integration tests" into "pure Go function tests", making them faster, more reliable, and easier to write!

---

## 16. Inventory Testing Assessment and Integration Testing Strategy

*Added January 26, 2026*

### Question

> Examine the current inventory tests, I mean, everything what can be testing the o2ims_infrastructureInventory. Specifically, I want to know:
>
> 1. Type of test: is that unit test using mocks exclusively? And external libraries we might be using for testing purposes.
> 2. Do we have integration testing in this project? Wondering if there is something to start up a postgres db (maybe a container) so the inventory rest interface can be tested entirely (rest request, db query and rest response).
> 3. I want you to assess the testcontainers library: https://testcontainers.com/ to that end in the previous item.

### Answer

#### 16.1 Current Testing Status for Resource Server (Inventory API)

##### Critical Finding: Almost No Tests for the Inventory API Handlers!

The `internal/service/resources/` directory has **only 2 test files**, both in the `listener/` subdirectory:

```
internal/service/resources/
├── listener/
│   ├── suite_test.go        # Ginkgo test suite setup
│   └── alarms_sync_test.go  # Tests for alarm dictionary sync functions
├── api/
│   └── server.go            # ⚠️ NO server_test.go!
├── db/repo/
│   └── repository.go        # ⚠️ NO repository_test.go!
```

**This is a significant gap** compared to other services:

| Service | Has `server_test.go` | Has `repository_test.go` | Test Coverage |
|---------|---------------------|-------------------------|---------------|
| **resource-server** (Inventory) | ❌ NO | ❌ NO | **Very Low** |
| **cluster-server** | ✅ YES | ✅ YES (with mocks) | Good |
| **alarms-server** | ✅ YES | ✅ YES (with pgxmock) | Good |
| **provisioning-server** | ❌ NO | N/A (no DB) | Low |

#### 16.2 Types of Tests and Libraries Used

##### Testing Framework Stack

| Library | Purpose | go.mod Version |
|---------|---------|----------------|
| **Ginkgo v2** | BDD-style test framework | `github.com/onsi/ginkgo/v2 v2.27.3` |
| **Gomega** | Assertion/matcher library | `github.com/onsi/gomega v1.38.0` |
| **gomock** | Interface mocking | `go.uber.org/mock v0.5.2` |
| **pgxmock** | PostgreSQL mock for pgx | `github.com/pashagolub/pgxmock/v4 v4.9.0` |

##### Test Types Currently Used

| Test Type | Used? | Where | Pattern |
|-----------|-------|-------|---------|
| **Unit tests with mocks** | ✅ Yes | `cluster/api/server_test.go`, `alarms/api/server_test.go` | Mock repository, test StrictServerInterface |
| **Repository tests with pgxmock** | ✅ Yes | `alarms/internal/db/repo/alarms_repository_test.go` | Mock SQL queries/results |
| **Controller tests with envtest** | ✅ Yes | `internal/controllers/*_test.go` | Fake K8s API server |
| **Integration tests with real DB** | ❌ NO | - | Not implemented |
| **E2E tests** | ✅ Yes | `test/e2e/e2e_suite_test.go` | Full cluster required |

##### Example: How Mocking Works in This Project

**1. Repository Interface Mocking (gomock)**

```go
// internal/service/cluster/db/repo/repository_interface.go
//go:generate mockgen -source=repository_interface.go -destination=generated/mock_repo.generated.go -package=generated

type RepositoryInterface interface {
    GetNodeClusterTypes(ctx context.Context) ([]models.NodeClusterType, error)
    GetNodeClusterType(ctx context.Context, id uuid.UUID) (*models.NodeClusterType, error)
    // ...
}
```

**2. SQL Mocking (pgxmock)**

```go
// internal/service/alarms/internal/db/repo/alarms_repository_test.go
mock, err := pgxmock.NewPool()
mock.ExpectQuery(`SELECT .* FROM alarm_event_record`).
    WithArgs(testUUID).
    WillReturnRows(pgxmock.NewRows(columns).AddRow(values...))
```

#### 16.3 Integration Testing Status

##### Current State: No Database Integration Tests

The project currently has **NO integration tests** that:
- Start a real PostgreSQL database
- Run the actual server code against it
- Test the full request → handler → repository → DB → response flow

##### Why This Matters for Your V11 Work

When you update the Inventory API for v11:
- **Schema changes** (new columns, tables) need DB migration testing
- **Query changes** need validation against real PostgreSQL behavior
- **JSON/JSONB handling** behaves differently in real DB vs mocks
- **Concurrent access patterns** can't be tested with mocks

#### 16.4 Testcontainers Assessment and Implementation Guide

##### What is Testcontainers?

[Testcontainers](https://testcontainers.com/) is a library that provides lightweight, throwaway instances of databases, message brokers, or any service that can run in a Docker container. For Go, it's available as `testcontainers-go`.

##### Why Testcontainers is Excellent for This Project

| Benefit | How It Helps O2IMS |
|---------|-------------------|
| **Real PostgreSQL** | Test actual SQL queries, JSONB operations, migrations |
| **Isolated per test** | Each test gets a fresh database |
| **CI/CD friendly** | Works in GitHub Actions, Tekton pipelines |
| **No external dependencies** | Just needs Docker |
| **Fast startup** | PostgreSQL container starts in ~2-3 seconds |

##### Testcontainers-Go PostgreSQL Module

```go
import (
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/wait"
)
```

#### 16.5 Detailed Implementation Guidance

##### Step 1: Add Testcontainers Dependency

```bash
# Add to go.mod
go get github.com/testcontainers/testcontainers-go
go get github.com/testcontainers/testcontainers-go/modules/postgres

# Update vendor
go mod tidy
go mod vendor
```

##### Step 2: Create Test Helper for PostgreSQL Container

Create a new file `internal/service/resources/db/testhelpers/postgres_container.go`:

```go
package testhelpers

import (
    "context"
    "fmt"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/wait"
)

// PostgresContainer wraps a testcontainers PostgreSQL instance
type PostgresContainer struct {
    Container testcontainers.Container
    Pool      *pgxpool.Pool
    ConnStr   string
}

// NewPostgresContainer creates a new PostgreSQL container for testing
func NewPostgresContainer(ctx context.Context) (*PostgresContainer, error) {
    // Start PostgreSQL container
    pgContainer, err := postgres.Run(ctx,
        "docker.io/postgres:16-alpine",
        postgres.WithDatabase("resources_test"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2).
                WithStartupTimeout(30*time.Second),
        ),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to start postgres container: %w", err)
    }

    // Get connection string
    connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
    if err != nil {
        pgContainer.Terminate(ctx)
        return nil, fmt.Errorf("failed to get connection string: %w", err)
    }

    // Create connection pool
    pool, err := pgxpool.New(ctx, connStr)
    if err != nil {
        pgContainer.Terminate(ctx)
        return nil, fmt.Errorf("failed to create pool: %w", err)
    }

    return &PostgresContainer{
        Container: pgContainer,
        Pool:      pool,
        ConnStr:   connStr,
    }, nil
}

// RunMigrations applies database migrations
func (pc *PostgresContainer) RunMigrations(ctx context.Context, migrationsPath string) error {
    // Use the same migration logic as the production code
    // This ensures migrations are tested too!
    migrator, err := migrate.New(
        "file://"+migrationsPath,
        pc.ConnStr,
    )
    if err != nil {
        return fmt.Errorf("failed to create migrator: %w", err)
    }
    defer migrator.Close()

    if err := migrator.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("failed to run migrations: %w", err)
    }
    return nil
}

// Cleanup terminates the container and closes connections
func (pc *PostgresContainer) Cleanup(ctx context.Context) error {
    if pc.Pool != nil {
        pc.Pool.Close()
    }
    if pc.Container != nil {
        return pc.Container.Terminate(ctx)
    }
    return nil
}

// TruncateTables clears all data for test isolation
func (pc *PostgresContainer) TruncateTables(ctx context.Context, tables ...string) error {
    for _, table := range tables {
        _, err := pc.Pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
        if err != nil {
            return fmt.Errorf("failed to truncate %s: %w", table, err)
        }
    }
    return nil
}
```

##### Step 3: Create Integration Test Suite

Create `internal/service/resources/api/integration_test.go`:

```go
//go:build integration
// +build integration

package api_test

import (
    "context"
    "net/http"
    "testing"
    "time"

    "github.com/google/uuid"
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"

    "github.com/openshift-kni/oran-o2ims/internal/service/resources/api"
    "github.com/openshift-kni/oran-o2ims/internal/service/resources/api/generated"
    "github.com/openshift-kni/oran-o2ims/internal/service/resources/db/models"
    "github.com/openshift-kni/oran-o2ims/internal/service/resources/db/repo"
    "github.com/openshift-kni/oran-o2ims/internal/service/resources/db/testhelpers"
)

var (
    pgContainer *testhelpers.PostgresContainer
    repository  *repo.ResourcesRepository
    server      *api.ResourceServer
    ctx         context.Context
)

var _ = BeforeSuite(func() {
    var err error
    ctx = context.Background()

    // Start PostgreSQL container (once for all tests)
    pgContainer, err = testhelpers.NewPostgresContainer(ctx)
    Expect(err).NotTo(HaveOccurred())

    // Run migrations
    err = pgContainer.RunMigrations(ctx, "../db/migrations")
    Expect(err).NotTo(HaveOccurred())

    // Create repository with real DB connection
    repository = repo.NewResourcesRepository(pgContainer.Pool)

    // Create server with real repository
    server = &api.ResourceServer{
        Repo: repository,
        Info: generated.OCloudInfo{
            OCloudId:      uuid.New(),
            GlobalcloudId: uuid.New(),
            Name:          "test-cloud",
            Description:   "Test O-Cloud",
            ServiceUri:    "https://test.example.com",
        },
    }
})

var _ = AfterSuite(func() {
    if pgContainer != nil {
        pgContainer.Cleanup(ctx)
    }
})

var _ = Describe("ResourceServer Integration Tests", func() {
    // Clean database before each test
    BeforeEach(func() {
        err := pgContainer.TruncateTables(ctx,
            "resource",
            "resource_pool",
            "resource_type",
            "data_source",
        )
        Expect(err).NotTo(HaveOccurred())
    })

    Describe("GetResourcePools", func() {
        When("database is empty", func() {
            It("returns empty list", func() {
                resp, err := server.GetResourcePools(ctx,
                    generated.GetResourcePoolsRequestObject{})

                Expect(err).NotTo(HaveOccurred())
                Expect(resp).To(BeAssignableToTypeOf(
                    generated.GetResourcePools200JSONResponse{}))

                okResp := resp.(generated.GetResourcePools200JSONResponse)
                Expect(okResp).To(BeEmpty())
            })
        })

        When("database has resource pools", func() {
            var testPoolID uuid.UUID

            BeforeEach(func() {
                testPoolID = uuid.New()
                // Insert test data directly into DB
                _, err := pgContainer.Pool.Exec(ctx, `
                    INSERT INTO data_source (data_source_id, name)
                    VALUES ($1, 'test-source')
                `, uuid.New())
                Expect(err).NotTo(HaveOccurred())

                _, err = pgContainer.Pool.Exec(ctx, `
                    INSERT INTO resource_pool (
                        resource_pool_id, name, description,
                        o_cloud_id, global_location_id,
                        data_source_id, generation_id, external_id
                    ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
                `, testPoolID, "test-pool", "Test Pool",
                    server.Info.OCloudId, uuid.New(),
                    uuid.New(), 1, "ext-1")
                Expect(err).NotTo(HaveOccurred())
            })

            It("returns the resource pools", func() {
                resp, err := server.GetResourcePools(ctx,
                    generated.GetResourcePoolsRequestObject{})

                Expect(err).NotTo(HaveOccurred())
                okResp := resp.(generated.GetResourcePools200JSONResponse)
                Expect(okResp).To(HaveLen(1))
                Expect(okResp[0].ResourcePoolId).To(Equal(testPoolID))
                Expect(okResp[0].Name).To(Equal("test-pool"))
            })
        })
    })

    Describe("GetResourcePool", func() {
        var testPoolID uuid.UUID

        BeforeEach(func() {
            testPoolID = uuid.New()
            // Setup test data...
        })

        When("resource pool exists", func() {
            It("returns the pool with all fields", func() {
                resp, err := server.GetResourcePool(ctx,
                    generated.GetResourcePoolRequestObject{
                        ResourcePoolId: testPoolID,
                    })

                Expect(err).NotTo(HaveOccurred())
                Expect(resp).To(BeAssignableToTypeOf(
                    generated.GetResourcePool200JSONResponse{}))
            })
        })

        When("resource pool does not exist", func() {
            It("returns 404", func() {
                resp, err := server.GetResourcePool(ctx,
                    generated.GetResourcePoolRequestObject{
                        ResourcePoolId: uuid.New(), // Non-existent
                    })

                Expect(err).NotTo(HaveOccurred())
                Expect(resp).To(BeAssignableToTypeOf(
                    generated.GetResourcePool404ApplicationProblemPlusJSONResponse{}))
            })
        })
    })

    // Test V11 specific features
    Describe("V11 Features", func() {
        Describe("ResourcePool with capacity fields", func() {
            It("correctly stores and retrieves capacity in extensions", func() {
                // Test new v11 fields like capacity, usedCapacity
                // This validates both the schema and the handler logic
            })
        })

        Describe("ResourceType with resourceClass enum", func() {
            It("validates resourceClass values", func() {
                // Test COMPUTE, NETWORKING, STORAGE, UNDEFINED
            })
        })
    })
})

func TestIntegration(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Resource Server Integration Suite")
}
```

##### Step 4: Add Makefile Target for Integration Tests

Add to `Makefile`:

```makefile
.PHONY: test-integration
test-integration: ## Run integration tests (requires Docker)
	@echo "Running integration tests with testcontainers..."
	go test -tags=integration -v ./internal/service/resources/... -timeout 5m

.PHONY: test-all
test-all: test test-integration ## Run all tests (unit + integration)
```

##### Step 5: CI/CD Configuration

For GitHub Actions (`.github/workflows/test.yaml`):

```yaml
jobs:
  integration-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Run integration tests
        run: make test-integration
```

For Tekton (`.tekton/integration-test.yaml`):

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: integration-test
spec:
  steps:
    - name: test
      image: golang:1.24
      script: |
        # Docker-in-Docker or use a sidecar
        make test-integration
      volumeMounts:
        - name: docker-socket
          mountPath: /var/run/docker.sock
```

##### Step 6: Testing the Full HTTP Stack (Optional Advanced)

For testing the complete HTTP middleware chain:

```go
package api_test

import (
    "net/http/httptest"
    "encoding/json"
)

var _ = Describe("Full HTTP Integration", func() {
    var httpServer *httptest.Server

    BeforeEach(func() {
        // Create the full HTTP handler with middleware
        swagger, _ := generated.GetSwagger()
        handler := generated.HandlerWithOptions(
            generated.NewStrictHandler(server, nil),
            generated.StdHTTPServerOptions{
                Middlewares: []generated.MiddlewareFunc{
                    middleware.OpenAPIValidation(swagger),
                },
            },
        )
        httpServer = httptest.NewServer(handler)
    })

    AfterEach(func() {
        httpServer.Close()
    })

    It("validates request against OpenAPI spec", func() {
        resp, err := http.Get(httpServer.URL +
            "/o2ims-infrastructureInventory/v1/resourcePools")

        Expect(err).NotTo(HaveOccurred())
        Expect(resp.StatusCode).To(Equal(http.StatusOK))

        var pools []generated.ResourcePool
        json.NewDecoder(resp.Body).Decode(&pools)
        // Assert...
    })
})
```

#### 16.6 Test Pattern Attribution

Based on git history analysis, here are the team members who introduced each testing pattern:

| Test Pattern | Introduced By | PR/Commit | Date |
|--------------|---------------|-----------|------|
| **Ginkgo/Gomega framework** | Juan Hernandez | Initial project setup | Early 2024 |
| **gomock for repository mocking** | Nahian Pathan | `#498` - "introduce DB and repository layer mocking" | 2024 |
| **StrictServerInterface testing** | Marcelo Guerrero Viveros | `#540` - "Add unit tests for alarms in cluster server" | 2024 |
| **pgxmock for SQL mocking** | Nahian Pathan | `#346` - "Use query builder for GetAlarmRecordID endpoint" | 2024 |
| **Controller tests with envtest** | Multiple contributors | Various PRs | Ongoing |
| **E2E test framework** | Brent Rowsell | `#887` - "Fix and enable e2e-tests" | 2025 |

##### Key Contributors to Testing Infrastructure

| Contributor | Email | Primary Contributions |
|-------------|-------|----------------------|
| **Nahian Pathan** | npathan@redhat.com | Repository mocking, pgxmock patterns, alarms tests |
| **Marcelo Guerrero Viveros** | marguerr@redhat.com | Server unit tests, StrictServerInterface patterns |
| **Don Penney** | dpenney@redhat.com | Test infrastructure, Ginkgo setup, linting |
| **Sharat Akhoury** | - | Controller tests, ClusterInstance handling |
| **Brent Rowsell** | - | E2E test framework |

#### Summary: Recommendations for Your V11 Work

| Priority | Action | Effort |
|----------|--------|--------|
| **1. HIGH** | Add `server_test.go` for resource-server using existing mock patterns | 1-2 days |
| **2. HIGH** | Add `repository_test.go` with pgxmock for SQL validation | 1 day |
| **3. MEDIUM** | Implement testcontainers integration tests | 2-3 days |
| **4. LOW** | Add full HTTP integration tests | 1 day |

The testcontainers approach is **highly recommended** because:
1. It validates your v11 schema changes against real PostgreSQL
2. It catches SQL syntax errors that mocks miss
3. It tests migrations in isolation
4. It's the industry standard for database integration testing

---

## 17. E2E Test Framework Deep Dive (Brent Rowsell)

*Added January 26, 2026*

### Question

> Talk about the E2E test framework (Brent) you have just been investigating.

### Answer

#### 17.1 E2E Test Framework Overview

The E2E test framework was **introduced by Brent Rowsell** in PR `#887` ("Fix and enable e2e-tests") and has been significantly enhanced by multiple contributors including **Irina Mihai**, **Sharat Akhoury**, **Don Penney**, and **Tao Liu**.

##### Architecture

The E2E tests are located in `test/e2e/e2e_suite_test.go` (~1400 lines) and use **controller-runtime's envtest** to create a **fake Kubernetes API server** with real CRDs.

```
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                              E2E TEST ARCHITECTURE                                           │
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────────────────────┐   │
│  │                              envtest (Fake K8s API Server)                            │   │
│  │  • Real CRDs installed from config/crd/bases/                                        │   │
│  │  • Real external CRDs (ACM, SiteConfig, Metal3) from test/utils/vars.go             │   │
│  │  • No actual cluster required                                                        │   │
│  └──────────────────────────────────────────────────────────────────────────────────────┘   │
│                                          │                                                   │
│                                          ▼                                                   │
│  ┌──────────────────────────────────────────────────────────────────────────────────────┐   │
│  │                          Controller Manager (in-process)                              │   │
│  │  • ProvisioningRequestReconciler running in test                                     │   │
│  │  • Watches real CRs created by tests                                                 │   │
│  └──────────────────────────────────────────────────────────────────────────────────────┘   │
│                                          │                                                   │
│                                          ▼                                                   │
│  ┌──────────────────────────────────────────────────────────────────────────────────────┐   │
│  │                        MockHardwarePluginServer (httptest.Server)                     │   │
│  │  • Simulates hardware plugin REST API                                                │   │
│  │  • Creates real K8s NodeAllocationRequest CRs                                        │   │
│  │  • Returns AllocatedNode details                                                     │   │
│  └──────────────────────────────────────────────────────────────────────────────────────┘   │
│                                                                                              │
└─────────────────────────────────────────────────────────────────────────────────────────────┘
```

##### Key Components

| Component | Purpose |
|-----------|---------|
| `envtest.Environment` | Fake Kubernetes API server with real CRDs |
| `ProvReqTestReconciler` | Real ProvisioningRequest reconciler running in-process |
| `MockHardwarePluginServer` | Simulates hardware plugin REST API |
| `testutils.CreateNodeResources()` | Helper to create test AllocatedNode CRs |
| `testutils.VerifyStatusCondition()` | Helper to assert CR status conditions |

##### How to Run E2E Tests

```bash
# Run E2E tests
make test-e2e

# Or directly with ginkgo
cd test/e2e && ginkgo -v ./...
```

---

### Question

> What kind of changes did Brent introduce firstly?

### Answer

#### 17.2 Brent's Initial Contributions

Brent made **two major contributions** that established the testing infrastructure:

##### PR #777: "Update unit tests due to restructuring" (July 28, 2025)

**Massive test infrastructure overhaul** - Added **14,288 lines** across **27 files**:

| Category | Files Added/Modified |
|----------|---------------------|
| **Mock Hardware Plugin Server** | `internal/controllers/mock_hardware_plugin_server.go` (535 lines - **NEW**) |
| **Metal3 Plugin Tests** | 10 new `*_test.go` files in `hwmgr-plugins/metal3/` |
| **Controller Tests** | Major enhancements to `provisioningrequest_*_test.go` |
| **Suite Setup** | Restructured `internal/controllers/suite_test.go` |

##### PR #887: "Fix and enable e2e-tests" (August 18, 2025)

**Enhanced the mock server for E2E tests** with Kubernetes integration:

| File Changed | Lines Changed | Purpose |
|--------------|---------------|---------|
| `mock_hardware_plugin_server.go` | +137 | Added K8s client integration |
| `e2e_suite_test.go` | +135 | Fixed and enabled E2E tests |
| `provisioningrequest_controller.go` | +17 | Added callback config |
| `suite_test.go` | +95/-95 | Restructured test setup |

#### 17.3 Mock Hardware Plugin Server

##### First Version (PR #777) - Core Mock Server

The initial mock server used Go's `httptest.Server` to simulate the Hardware Plugin REST API:

```go
// mock_hardware_plugin_server.go - First version
type MockHardwarePluginServer struct {
    server                 *httptest.Server
    nodeAllocationRequests map[string]*hwmgrpluginapi.NodeAllocationRequestResponse
    allocatedNodes         map[string][]hwmgrpluginapi.AllocatedNode
}
```

This mock server:
- Uses Go's `httptest.Server` to simulate the Hardware Plugin REST API
- Stores NodeAllocationRequests and AllocatedNodes in memory
- Provides endpoints: `GET/POST /nodeAllocationRequests`, `GET /allocatedNodes`
- Allows tests to run **without a real hardware plugin**

##### Enhanced Version (PR #887) - K8s Integration

Added Kubernetes client integration for E2E tests:

```go
// New constructor with K8s client
func NewMockHardwarePluginServerWithClient(k8sClient client.Client) *MockHardwarePluginServer {
    mock := &MockHardwarePluginServer{
        nodeAllocationRequests: make(map[string]*hwmgrpluginapi.NodeAllocationRequestResponse),
        allocatedNodes:         make(map[string][]hwmgrpluginapi.AllocatedNode),
        k8sClient:              k8sClient,  // NEW: Kubernetes client
    }
    // ...
}
```

##### Key Enhancement: Mock Server Creates Real K8s Resources

When the mock server receives a `POST /nodeAllocationRequests`, it now creates a real Kubernetes CR:

```go
// When mock server receives POST /nodeAllocationRequests, it creates a real K8s CR
func (m *MockHardwarePluginServer) createKubernetesNodeAllocationRequest(ctx, request, requestID) error {
    k8sNodeAllocationRequest := &pluginsv1alpha1.NodeAllocationRequest{
        ObjectMeta: metav1.ObjectMeta{
            Name:      requestID,
            Namespace: ctlrutils.UnitTestHwmgrNamespace,
        },
        // ... convert API request to K8s CR
    }
    return m.k8sClient.Create(ctx, k8sNodeAllocationRequest)
}
```

##### E2E Test Suite Setup (PR #887)

```go
// From test/e2e/e2e_suite_test.go - BeforeSuite
var _ = BeforeSuite(func() {
    // Setup envtest with real CRDs
    testEnv = &envtest.Environment{
        CRDDirectoryPaths: []string{
            filepath.Join("..", "..", "config", "crd", "bases"),
        },
    }

    // Setup the ProvisioningRequest Reconciler
    ProvReqTestReconciler = &provisioningcontrollers.ProvisioningRequestReconciler{
        Client:         K8SClient,
        Logger:         logger,
        CallbackConfig: ctlrutils.NewNarCallbackConfig(constants.DefaultNarCallbackServicePort),
    }

    // Start mock hardware plugin server for e2e tests with Kubernetes client
    mockServer := provisioningcontrollers.NewMockHardwarePluginServerWithClient(K8SClient)

    // Create HardwarePlugin CR pointing to mock server
    &hwmgmtv1alpha1.HardwarePlugin{
        ObjectMeta: metav1.ObjectMeta{Name: testHardwarePluginRef},
        Spec: hwmgmtv1alpha1.HardwarePluginSpec{
            ApiRoot: mockServer.GetURL(),  // Points to mock!
            AuthClientConfig: &common.AuthClientConfig{
                Type:            common.Basic,
                BasicAuthSecret: stringPtr("test-hwmgr-auth-secret"),
            },
        },
    }
})
```

#### 17.4 E2E Test Architecture Summary

```
┌─────────────────────────────────────────────────────────────────┐
│                    E2E Test Architecture (Brent)                │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────────────┐     ┌───────────────────────────────┐│
│  │   Ginkgo Test Suite  │     │   envtest (Fake K8s API)     ││
│  │  (e2e_suite_test.go) │────▶│   - Real CRDs installed      ││
│  └──────────┬───────────┘     │   - Real reconcilers         ││
│             │                 └───────────────────────────────┘│
│             │                                                   │
│             │ Creates CRs                                       │
│             ▼                                                   │
│  ┌──────────────────────┐     ┌───────────────────────────────┐│
│  │  ProvisioningRequest │────▶│   ProvReqTestReconciler      ││
│  │  (test CR)           │     │   (calls hardware plugin)    ││
│  └──────────────────────┘     └───────────┬───────────────────┘│
│                                           │                     │
│                                           │ REST calls to       │
│                                           ▼                     │
│                               ┌───────────────────────────────┐│
│                               │  MockHardwarePluginServer    ││
│                               │  (httptest.Server)            ││
│                               │  - Simulates REST API         ││
│                               │  - Creates K8s CRs (v2)       ││
│                               └───────────────────────────────┘│
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

##### Why Brent's Contributions Matter

| Before Brent | After Brent |
|--------------|-------------|
| E2E tests were broken/disabled | E2E tests run automatically |
| No way to test hardware provisioning flow | Mock server simulates full flow |
| Tests required real hardware plugin | Tests run with no external dependencies |
| ~2,800 lines in controller tests | ~14,000+ lines with comprehensive coverage |

##### Key Files from Brent's Contributions

| File | Purpose |
|------|---------|
| `internal/controllers/mock_hardware_plugin_server.go` | Mock hardware plugin REST API |
| `test/e2e/e2e_suite_test.go` | E2E test suite with envtest |
| `internal/controllers/suite_test.go` | Controller unit test setup |
| `hwmgr-plugins/metal3/controller/*_test.go` | Metal3 plugin tests |

The mock server is **essential** for testing the provisioning flow without requiring actual hardware infrastructure (Metal3, BMC, etc.).

---

## Document Information

- **Generated:** January 12, 2026
- **Reviewed:** January 19, 2026 (corrections based on official docs)
- **Updated:** January 21, 2026 (added Server Pattern section)
- **Updated:** January 26, 2026 (added StrictServerInterface and Testing section)
- **Updated:** January 26, 2026 (added Inventory Testing Assessment and Testcontainers guide)
- **Updated:** January 26, 2026 (added E2E Test Framework Deep Dive - Brent Rowsell)
- **Project:** O-RAN O2IMS Operator
- **Repository:** `github.com/openshift-kni/oran-o2ims`
