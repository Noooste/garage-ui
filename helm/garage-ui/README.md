# Garage UI Helm Chart

A Helm chart for deploying Garage UI, a web interface for [Garage](https://garagehq.deuxfleurs.fr/) S3 object storage.

## Introduction

This chart bootstraps a Garage UI deployment on a Kubernetes cluster using the Helm package manager. Garage UI provides a user-friendly web interface for managing your Garage S3 storage, including buckets, objects, users, and cluster monitoring.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- A running Garage S3 instance with Admin API access

## Installing the Chart

To install the chart with the release name `my-garage-ui`:

```bash
helm install my-garage-ui ./helm/garage-ui
```

The command deploys Garage UI on the Kubernetes cluster with default configuration. The [Parameters](#parameters) section lists the parameters that can be configured during installation.

## Uninstalling the Chart

To uninstall/delete the `my-garage-ui` deployment:

```bash
helm uninstall my-garage-ui
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Parameters

### Common Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of Garage UI replicas | `1` |
| `image.repository` | Garage UI image repository | `noooste/garage-ui` |
| `image.tag` | Garage UI image tag (overrides Chart appVersion) | `""` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `imagePullSecrets` | Image pull secrets | `[]` |
| `nameOverride` | Override chart name | `""` |
| `fullnameOverride` | Override full resource names | `""` |

### Service Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.type` | Kubernetes service type | `ClusterIP` |
| `service.port` | Service port | `80` |

### Configuration Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config` | Complete config.yaml content as multiline string | See `values.yaml` |

**Important:** The `config` parameter contains the entire application configuration including Garage endpoints, authentication settings, CORS, and logging. You must customize this section with your Garage S3 endpoints and admin token.

### Ingress Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ingress.enabled` | Enable ingress controller resource | `false` |
| `ingress.className` | Ingress class name | `nginx` |
| `ingress.annotations` | Ingress annotations | `{}` |
| `ingress.hosts` | Ingress hosts configuration | See `values.yaml` |
| `ingress.tls` | Ingress TLS configuration | `[]` |

### Monitoring Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `serviceMonitor.enabled` | Create ServiceMonitor resource (requires Prometheus Operator) | `false` |
| `serviceMonitor.interval` | Scrape interval | `30s` |
| `serviceMonitor.path` | Metrics endpoint path | `/api/v1/monitoring/metrics` |
| `serviceMonitor.labels` | Additional labels for ServiceMonitor | `{}` |

### Network Policy Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `networkPolicy.enabled` | Enable NetworkPolicy | `false` |
| `networkPolicy.policyTypes` | Policy types | `["Ingress", "Egress"]` |

### Resource Management Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `512Mi` |
| `resources.requests.cpu` | CPU request | `100m` |
| `resources.requests.memory` | Memory request | `128Mi` |

### Health Check Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `livenessProbe.enabled` | Enable liveness probe | `true` |
| `readinessProbe.enabled` | Enable readiness probe | `true` |

### Other Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `podAnnotations` | Pod annotations | `{}` |
| `podSecurityContext` | Pod security context | See `values.yaml` |
| `securityContext` | Container security context | See `values.yaml` |
| `nodeSelector` | Node labels for pod assignment | `{}` |
| `tolerations` | Tolerations for pod assignment | `[]` |
| `affinity` | Affinity for pod assignment | `{}` |

## Configuration Examples

### Example 1: Basic Installation with Custom Garage Endpoints

Create a file named `custom-values.yaml`:

```yaml
config: |
  server:
    host: "0.0.0.0"
    port: 8080
    environment: "production"

  garage:
    endpoint: "http://garage-s3.garage.svc.cluster.local:3900"
    region: "us-east-1"
    admin_endpoint: "http://garage-admin.garage.svc.cluster.local:3903"
    admin_token: "YOUR_ADMIN_TOKEN_HERE"

  auth:
    mode: "none"

  cors:
    enabled: true
    allowed_origins:
      - "*"

  logging:
    level: "info"
    format: "json"
```

Install the chart:

```bash
helm install my-garage-ui ./helm/garage-ui -f custom-values.yaml
```

### Example 2: With Ingress and TLS

```yaml
config: |
  server:
    host: "0.0.0.0"
    port: 8080
    environment: "production"

  garage:
    endpoint: "http://garage:3900"
    region: "garage"
    admin_endpoint: "http://garage:3903"
    admin_token: "YOUR_ADMIN_TOKEN"

  auth:
    mode: "none"

  cors:
    enabled: true

  logging:
    level: "info"
    format: "json"

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: garage-ui.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: garage-ui-tls
      hosts:
        - garage-ui.example.com
```

### Example 3: With Basic Authentication

```yaml
config: |
  server:
    host: "0.0.0.0"
    port: 8080
    environment: "production"

  garage:
    endpoint: "http://garage:3900"
    region: "garage"
    admin_endpoint: "http://garage:3903"
    admin_token: "YOUR_ADMIN_TOKEN"

  auth:
    mode: "basic"
    basic:
      username: "admin"
      password: "your-secure-password"

  cors:
    enabled: true

  logging:
    level: "info"
    format: "json"
```

### Example 4: With OIDC Authentication (Keycloak)

```yaml
config: |
  server:
    host: "0.0.0.0"
    port: 8080
    environment: "production"

  garage:
    endpoint: "http://garage:3900"
    region: "garage"
    admin_endpoint: "http://garage:3903"
    admin_token: "YOUR_ADMIN_TOKEN"

  auth:
    mode: "oidc"
    oidc:
      enabled: true
      provider_name: "Keycloak"
      client_id: "garage-ui"
      client_secret: "YOUR_OIDC_CLIENT_SECRET"
      scopes:
        - openid
        - email
        - profile
      issuer_url: "https://keycloak.example.com/realms/master"
      auth_url: "https://keycloak.example.com/realms/master/protocol/openid-connect/auth"
      token_url: "https://keycloak.example.com/realms/master/protocol/openid-connect/token"
      userinfo_url: "https://keycloak.example.com/realms/master/protocol/openid-connect/userinfo"
      cookie_secure: true

  cors:
    enabled: true

  logging:
    level: "info"
    format: "json"
```

### Example 5: With Prometheus Monitoring

```yaml
config: |
  server:
    host: "0.0.0.0"
    port: 8080
    environment: "production"

  garage:
    endpoint: "http://garage:3900"
    region: "garage"
    admin_endpoint: "http://garage:3903"
    admin_token: "YOUR_ADMIN_TOKEN"

  auth:
    mode: "none"

  cors:
    enabled: true

  logging:
    level: "info"
    format: "json"

serviceMonitor:
  enabled: true
  interval: 30s
  labels:
    prometheus: kube-prometheus
```

## Accessing the Application

### Via Port Forward (Development)

```bash
kubectl port-forward svc/my-garage-ui 8080:80
```

Then visit http://localhost:8080 in your browser.

### Via Ingress (Production)

If you've enabled Ingress, access the application at the configured hostname (e.g., https://garage-ui.example.com).

## Upgrading

To upgrade the `my-garage-ui` deployment:

```bash
helm upgrade my-garage-ui ./helm/garage-ui -f custom-values.yaml
```

## Configuration Details

### Garage Endpoints

The application requires two Garage endpoints:

- **S3 API Endpoint** (`garage.endpoint`): The Garage S3 API endpoint (default port 3900)
- **Admin API Endpoint** (`garage.admin_endpoint`): The Garage Admin API endpoint (default port 3903)
- **Admin Token** (`garage.admin_token`): Bearer token for Admin API authentication (required)

### Authentication Modes

The application supports three authentication modes:

1. **None** (`auth.mode: "none"`): No authentication required
2. **Basic** (`auth.mode: "basic"`): HTTP Basic authentication with username/password
3. **OIDC** (`auth.mode: "oidc"`): OpenID Connect for enterprise SSO

### Health Checks

The application exposes a health check endpoint at `/health` which is used for:
- Kubernetes liveness probes
- Kubernetes readiness probes
- Manual health verification

### Metrics

When ServiceMonitor is enabled, Prometheus will scrape metrics from `/api/v1/monitoring/metrics`. This endpoint proxies metrics from the Garage Admin API.

## Troubleshooting

### Pods not starting

Check if the config is valid:
```bash
kubectl logs -l app.kubernetes.io/name=garage-ui
```

Common issues:
- Missing or invalid `garage.admin_token`
- Unreachable Garage endpoints
- Invalid OIDC configuration when using `auth.mode: "oidc"`

### Can't access the UI

If using Ingress:
```bash
kubectl get ingress
kubectl describe ingress <ingress-name>
```

If using port-forward:
```bash
kubectl get pods
kubectl port-forward <pod-name> 8080:8080
```

### Configuration not updating

The deployment includes a checksum annotation for the ConfigMap. Changes to the config will automatically trigger a pod restart. If not:

```bash
kubectl rollout restart deployment/my-garage-ui
```

## License

This Helm chart is open source and available under the same license as Garage UI.

## Support

For issues and questions:
- GitHub Issues: https://github.com/Noooste/garage-ui/issues
- Garage Documentation: https://garagehq.deuxfleurs.fr/

## Contributing

Contributions are welcome! Please submit pull requests to the Garage UI repository.
