# Garage UI Helm Chart

A Helm chart for deploying [Garage UI](https://github.com/Noooste/garage-ui), a modern web interface for managing [Garage](https://garagehq.deuxfleurs.fr/) distributed object storage systems.

[![Version](https://img.shields.io/badge/version-0.9.0?color=blue)](Chart.yaml) <!-- x-release-please-version -->
[![App Version](https://img.shields.io/badge/app%20version-v0.9.0?color=green)](Chart.yaml) <!-- x-release-please-version -->

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Configuration](#configuration)
- [Examples](#examples)
- [Upgrading](#upgrading)
- [Accessing the Application](#accessing-the-application)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)
- [Uninstalling](#uninstalling)

## Overview

Garage UI provides an intuitive web interface for managing your Garage S3 storage cluster, featuring:

- **Bucket Management** - Create, delete, and configure S3 buckets
- **Object Operations** - Upload, download, and delete objects through the UI
- **User Access Control** - Manage users, access keys, and permissions
- **Cluster Monitoring** - View cluster health, status, and statistics
- **Authentication** - Support for no auth, basic auth, and OIDC/SSO

## Prerequisites

Before installing this chart, ensure you have:

- **Kubernetes** `1.19+` or later
- **Helm** `3.0+` or later
- **Garage S3** instance running and accessible
- **Garage Admin Token** - Required for administrative operations

### Required Information

You will need the following information from your Garage installation:

1. **Garage S3 Endpoint** - The Garage S3 API endpoint (default port: `3900`)
2. **Garage Admin Endpoint** - The Garage Admin API endpoint (default port: `3903`)
3. **Admin Token** - Bearer token for authenticating with the Admin API

To find your admin token, check your Garage server's configuration file.

## Quick Start

The fastest way to get started with Garage UI:

### Step 1: Create a values file

Create a file named `my-values.yaml` with your Garage configuration:

```yaml
config:
  garage:
    endpoint: "http://garage:3900"              # Your Garage S3 endpoint
    admin_endpoint: "http://garage:3903"        # Your Garage Admin endpoint
    admin_token: "YOUR_ADMIN_TOKEN_HERE"        # Your admin token
    region: "garage"                            # S3 region (can be any value)
```

### Step 2: Install the chart

```bash
helm install garage-ui ./helm/garage-ui -f my-values.yaml
```

### Step 3: Access the UI

```bash
# Forward port to access locally
kubectl port-forward svc/garage-ui 8080:80

# Open in your browser
open http://localhost:8080
```

That's it! You should now have Garage UI running and accessible.

## Installation

### Installing from local chart

If you've cloned the repository:

```bash
helm install garage-ui ./helm/garage-ui -f my-values.yaml
```

### Installing from the OCI registry (ghcr.io)

The chart is published as an OCI artifact to GitHub Container Registry. No
`helm repo add` is required:

```bash
helm install garage-ui oci://ghcr.io/noooste/charts/garage-ui \
  --version <x.y.z> -f my-values.yaml
```

The chart is signed with [cosign](https://docs.sigstore.dev/) using keyless
signing. To verify the signature before installing:

```bash
cosign verify ghcr.io/noooste/charts/garage-ui:<x.y.z> \
  --certificate-identity-regexp 'https://github.com/Noooste/garage-ui/.+' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

The chart also remains available from the classic Helm repository at
`https://helm.noste.dev`.

### Installing with inline values

You can also set values directly on the command line:

```bash
helm install garage-ui ./helm/garage-ui \
  --set config.garage.endpoint=http://garage:3900 \
  --set config.garage.admin_endpoint=http://garage:3903 \
  --set config.garage.admin_token=your-token-here
```

### Verify installation

Check that the pod is running:

```bash
kubectl get pods -l app.kubernetes.io/name=garage-ui
```

View the logs:

```bash
kubectl logs -l app.kubernetes.io/name=garage-ui
```

## Configuration

### Configuration Structure

The chart uses a structured `config` section that maps directly to the application's configuration file. You can override any value using Helm's standard methods.

### Minimal Configuration

The absolute minimum configuration requires only the Garage endpoints and admin token:

```yaml
config:
  garage:
    endpoint: "http://garage:3900"
    admin_endpoint: "http://garage:3903"
    admin_token: "your-admin-token"
```

### Common Configuration Options

#### Server Settings

```yaml
config:
  server:
    port: 8080              # Application port
    environment: production # Environment (production/development/staging)
```

#### Authentication Configuration

**No Authentication** (default, suitable for private networks):
```yaml
config:
  auth:
    admin:
      enabled: false
    oidc:
      enabled: false
```

**Admin Authentication** (username/password with JWT):
```yaml
config:
  auth:
    admin:
      enabled: true
      username: "admin"
      password: "your-secure-password"
    oidc:
      enabled: false
```

**OIDC/SSO** (recommended for production):
```yaml
config:
  auth:
    admin:
      enabled: false
    oidc:
      enabled: true
      provider_name: "Keycloak"
      client_id: "garage-ui"
      client_secret: "your-oidc-secret"
      issuer_url: "https://auth.example.com/realms/master"
      # ... additional OIDC settings
```

**Both Authentication Methods** (admin and OIDC simultaneously):
```yaml
config:
  auth:
    admin:
      enabled: true
      username: "admin"
      password: "your-secure-password"
    oidc:
      enabled: true
      provider_name: "Keycloak"
      client_id: "garage-ui"
      client_secret: "your-oidc-secret"
      issuer_url: "https://auth.example.com/realms/master"
      # ... additional OIDC settings
```

#### Multi-User Access Control (optional)

Scope what each OIDC user can see and do, based on the teams in their token
claims. **Absent by default**, so every authenticated user keeps full access.
When set, authorization becomes default-deny.

> **Not a security boundary.** This is UI-layer policy only. Anyone holding the
> Garage admin token or raw S3 keys bypasses it. See
> [docs/access-control.md](../../docs/access-control.md) for the full model and
> permission vocabulary.

```yaml
config:
  auth:
    oidc:
      enabled: true
      # OIDC claim (go-jmespath) listing the user's teams.
      team_attribute_path: "groups"
      # admin_role stays optional once access_control is set: unmatched users
      # are denied rather than promoted to admin.
  access_control:
    presets:
      bucket_readonly: [bucket.list, bucket.read, object.list, object.read]
      bucket_owner: ["preset:bucket_readonly", bucket.create, bucket.update,
                     bucket.delete, object.write, object.delete]
    teams:
      - name: backend
        claim_values: ["garage-team-backend"]   # matched against the team_attribute_path claim
        bindings:
          - bucket_prefixes: ["backend-"]
            permissions: ["preset:bucket_owner"]
          - bucket_prefixes: ["shared-"]
            permissions: ["preset:bucket_readonly"]
        cluster_permissions: [cluster.status, cluster.health]
```

Admin-password and Garage-admin-token logins are always full admin in v1; only
OIDC users can be scoped to a team.

#### CORS Configuration

```yaml
config:
  cors:
    enabled: true
    allowed_origins:
      - "https://garage-ui.example.com"  # Recommended: specific origins
      # - "*"                            # Or: allow all origins (less secure)
```

#### Logging

```yaml
config:
  logging:
    level: info    # debug, info, warn, error
    format: json   # json (recommended for production), text
```

### Kubernetes Resource Configuration

#### Ingress

Enable external access with Ingress:

```yaml
ingress:
  enabled: true
  className: nginx
  hosts:
    - host: garage-ui.example.com
      paths:
        - path: /
          pathType: Prefix
```

With TLS:

```yaml
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

#### Resource Limits

```yaml
resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

#### Replicas and High Availability

```yaml
replicaCount: 3

affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchExpressions:
              - key: app.kubernetes.io/name
                operator: In
                values:
                  - garage-ui
          topologyKey: kubernetes.io/hostname
```

### Complete Parameters Reference

For a complete list of all available parameters, see the [values.yaml](values.yaml) file which includes detailed comments for every configuration option.

## Examples

### Example 1: Basic Installation (In-Cluster Garage)

For Garage running in the same Kubernetes cluster:

```yaml
# values-basic.yaml
config:
  garage:
    endpoint: "http://garage.garage-system.svc.cluster.local:3900"
    admin_endpoint: "http://garage.garage-system.svc.cluster.local:3903"
    admin_token: "GgJLd9J...your-token-here"
    region: "garage"
```

Install:
```bash
helm install garage-ui ./helm/garage-ui -f values-basic.yaml
```

### Example 2: External Access with Ingress

```yaml
# values-ingress.yaml
config:
  garage:
    endpoint: "http://garage:3900"
    admin_endpoint: "http://garage:3903"
    admin_token: "your-admin-token"

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
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

Install:
```bash
helm install garage-ui ./helm/garage-ui -f values-ingress.yaml
```

### Example 3: With Admin Authentication

```yaml
# values-admin-auth.yaml
config:
  garage:
    endpoint: "http://garage:3900"
    admin_endpoint: "http://garage:3903"
    admin_token: "your-admin-token"

  auth:
    admin:
      enabled: true
      username: "admin"
      password: "super-secret-password-change-me"
    oidc:
      enabled: false

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: garage.local
      paths:
        - path: /
          pathType: Prefix
```

Install:
```bash
helm install garage-ui ./helm/garage-ui -f values-admin-auth.yaml
```

### Example 4: Production Setup with OIDC (Keycloak)

```yaml
# values-production.yaml
replicaCount: 3

config:
  server:
    environment: production

  garage:
    endpoint: "https://s3.garage.example.com"
    admin_endpoint: "https://admin.garage.example.com"
    admin_token: "your-admin-token"
    region: "us-east-1"

  auth:
    admin:
      enabled: false
    oidc:
      enabled: true
      provider_name: "Keycloak"
      client_id: "garage-ui"
      client_secret: "your-oidc-client-secret"
      scopes:
        - openid
        - email
        - profile
      issuer_url: "https://auth.example.com/realms/production"
      cookie_secure: true
      cookie_http_only: true
      cookie_same_site: "lax"

  cors:
    enabled: true
    allowed_origins:
      - "https://garage-ui.example.com"

  logging:
    level: info
    format: json

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
    nginx.ingress.kubernetes.io/proxy-body-size: "100m"
  hosts:
    - host: garage-ui.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: garage-ui-tls
      hosts:
        - garage-ui.example.com

resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 200m
    memory: 256Mi

affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchExpressions:
              - key: app.kubernetes.io/name
                operator: In
                values:
                  - garage-ui
          topologyKey: kubernetes.io/hostname
```

Install:
```bash
helm install garage-ui ./helm/garage-ui -f values-production.yaml
```

### Example 5: With Prometheus Monitoring

```yaml
# values-monitoring.yaml
config:
  garage:
    endpoint: "http://garage:3900"
    admin_endpoint: "http://garage:3903"
    admin_token: "your-admin-token"

serviceMonitor:
  enabled: true
  interval: 30s
  labels:
    prometheus: kube-prometheus
```

Install:
```bash
helm install garage-ui ./helm/garage-ui -f values-monitoring.yaml
```

### Example 6: Using Kubernetes Secrets for Admin Token (Recommended)

For improved security, store the admin token in a Kubernetes secret instead of in values files:

**Option A: Use an existing secret**

First, create a Kubernetes secret:
```bash
kubectl create secret generic garage-admin-token \
  --from-literal=admin-token='your-admin-token-here'
```

Then configure the chart to use it:
```yaml
# values-with-secret.yaml
config:
  garage:
    endpoint: "http://garage:3900"
    admin_endpoint: "http://garage:3903"
    # Leave admin_token empty when using existingSecret
    admin_token: ""

    # Reference the existing secret
    existingSecret:
      name: "garage-admin-token"
      key: "admin-token"
```

Install:
```bash
helm install garage-ui ./helm/garage-ui -f values-with-secret.yaml
```

**Option B: Let the chart create the secret**

If you provide `admin_token` in values but don't configure `existingSecret`, the chart will automatically create a secret for you:

```yaml
# values-auto-secret.yaml
config:
  garage:
    endpoint: "http://garage:3900"
    admin_endpoint: "http://garage:3903"
    # Chart will create a secret with this value
    admin_token: "your-admin-token"
```

This approach keeps the token out of the ConfigMap while still allowing you to manage it through Helm values.

## Upgrading

### Upgrade to a new version

```bash
helm upgrade garage-ui ./helm/garage-ui -f my-values.yaml
```

### Upgrade with new values

```bash
helm upgrade garage-ui ./helm/garage-ui \
  --set config.logging.level=debug \
  --reuse-values
```

### Check upgrade history

```bash
helm history garage-ui
```

### Rollback to previous version

```bash
helm rollback garage-ui
```

## Accessing the Application

### Method 1: Port Forward (Development)

Quick access for testing:

```bash
kubectl port-forward svc/garage-ui 8080:80
```

Then open: http://localhost:8080

### Method 2: Ingress (Production)

If you've enabled Ingress with a domain:

```bash
# Access via your configured domain
open https://garage-ui.example.com
```

### Method 3: NodePort (Alternative)

Change service type to NodePort:

```yaml
service:
  type: NodePort
  port: 80
```

Find the assigned port:

```bash
kubectl get svc garage-ui
```

### Method 4: LoadBalancer (Cloud Environments)

For cloud providers with LoadBalancer support:

```yaml
service:
  type: LoadBalancer
  port: 80
```

Get the external IP:

```bash
kubectl get svc garage-ui
```

## Monitoring

### Prometheus ServiceMonitor

Enable Prometheus metrics scraping (requires Prometheus Operator):

```yaml
serviceMonitor:
  enabled: true
  interval: 30s
  # /metrics is served only when config.auth.metrics_public is true
  path: /metrics
  labels:
    prometheus: kube-prometheus
```

### Metrics Endpoint

The application exposes Prometheus-format metrics (proxying the Garage Admin API) at:
- `/api/v1/monitoring/metrics`: always registered, requires authentication.
- `/metrics`: top-level, unauthenticated. Served ONLY when `config.auth.metrics_public` is `true`. Use this for Prometheus scraping when authentication is enabled, and restrict access with a NetworkPolicy / trusted scrape network.

### Health Checks

Health endpoint available at:
- Path: `/health`
- Response: `{"status": "ok", "version": "0.1.0"}`

Used by Kubernetes liveness and readiness probes.

## Troubleshooting

### Pods Not Starting

Check pod status and logs:

```bash
kubectl get pods -l app.kubernetes.io/name=garage-ui
kubectl describe pod -l app.kubernetes.io/name=garage-ui
kubectl logs -l app.kubernetes.io/name=garage-ui
```

Common issues:
- **Missing admin token**: Ensure `config.garage.admin_token` is set or `config.garage.existingSecret.name` points to a valid secret
- **Secret not found**: If using `existingSecret`, verify the secret exists: `kubectl get secret <secret-name>`
- **Unreachable Garage**: Verify endpoints are accessible from within the cluster
- **Invalid OIDC config**: Check all OIDC URLs and credentials when using `auth.oidc.enabled: true`

### Cannot Access the UI

For Ingress issues:

```bash
kubectl get ingress
kubectl describe ingress garage-ui
```

Check Ingress controller logs:

```bash
kubectl logs -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx
```

Common issues:
- **DNS not configured**: Ensure your domain points to the Ingress controller
- **Certificate issues**: Check cert-manager logs if using automatic TLS
- **Ingress class mismatch**: Verify `ingress.className` matches your controller

### Configuration Not Updating

The deployment includes a ConfigMap checksum annotation that automatically triggers pod restarts when configuration changes. If it's not working:

```bash
kubectl rollout restart deployment/garage-ui
```

### Connection to Garage Fails

Test connectivity from within the pod:

```bash
kubectl exec -it deployment/garage-ui -- sh

# Inside the pod:
curl http://garage:3900
curl http://garage:3903/health
```

### Authentication Issues

**Admin Auth:**
- Verify username/password in `config.auth.admin`
- Ensure `config.auth.admin.enabled` is set to `true`
- Check browser developer tools for 401 errors
- Verify JWT token is being sent in Authorization header

**OIDC:**
- Ensure `config.auth.oidc.enabled` is set to `true`
- Verify all OIDC URLs are accessible
- Check OIDC provider logs
- Ensure redirect URI is registered: `https://your-domain/auth/oidc/callback`
- Verify client ID and secret

### View Application Logs

```bash
# Follow logs
kubectl logs -f -l app.kubernetes.io/name=garage-ui

# View recent logs
kubectl logs --tail=100 -l app.kubernetes.io/name=garage-ui

# Export logs
kubectl logs -l app.kubernetes.io/name=garage-ui > garage-ui.log
```

### Debug Mode

Enable debug logging:

```yaml
config:
  logging:
    level: debug
```

## Uninstalling

### Remove the release

```bash
helm uninstall garage-ui
```

This removes all Kubernetes components associated with the chart.

### Clean up completely

If you want to remove all associated resources:

```bash
# Uninstall the release
helm uninstall garage-ui

# Remove any remaining ConfigMaps
kubectl delete configmap -l app.kubernetes.io/name=garage-ui

# Remove any remaining Secrets
kubectl delete secret -l app.kubernetes.io/name=garage-ui
```

## Advanced Configuration

### Using Secrets for Sensitive Data

The chart supports using Kubernetes secrets for sensitive data to avoid storing credentials in values files.

#### Admin Token from Secret

The chart has built-in support for storing the admin token in a Kubernetes secret:

**Method 1: Use an existing secret** (recommended)

```bash
# Create the secret
kubectl create secret generic garage-admin-token \
  --from-literal=admin-token='your-admin-token-here'
```

Configure in values:
```yaml
config:
  garage:
    existingSecret:
      name: "garage-admin-token"
      key: "admin-token"
```

**Method 2: Let the chart create the secret**

Simply provide the `admin_token` in values, and the chart will automatically create a secret:

```yaml
config:
  garage:
    admin_token: "your-admin-token"
```

The chart will:
1. Create a secret named `<release-name>-admin-token` with the token
2. Remove the token from the ConfigMap
3. Inject it into the pod via environment variable

This keeps sensitive data out of ConfigMaps while maintaining easy Helm-based management.

#### JWT Private Key for Session Tokens

The chart automatically manages JWT private keys for signing session tokens. You have three options:

**Method 1: Auto-generation** (recommended for most cases)

By default, if you don't provide a JWT private key, the chart will automatically generate an Ed25519 private key on first install and persist it in a Kubernetes secret. This key will be reused across upgrades, ensuring session tokens remain valid.

```yaml
config:
  auth:
    # Leave both empty - chart will auto-generate and persist a key
    jwt_private_key: ""
    jwt_private_key_secret:
      name: ""
```

The chart will:
1. Run a pre-install/pre-upgrade Job to generate an Ed25519 key
2. Store it in a secret named `<release-name>-jwt-key`
3. Preserve the secret across chart upgrades (using `helm.sh/resource-policy: keep`)

**Method 2: Provide your own key inline**

Generate a key manually and include it in values:

```bash
# Generate Ed25519 private key
openssl genpkey -algorithm ED25519 -out jwt-key.pem
```

```yaml
config:
  auth:
    jwt_private_key: |
      -----BEGIN PRIVATE KEY-----
      MC4CAQAwBQYDK2VwBCIEI...
      -----END PRIVATE KEY-----
    jwt_private_key_secret:
      name: ""
```

**Method 3: Use an existing Kubernetes secret**

Create the secret manually:

```bash
# Generate key
openssl genpkey -algorithm ED25519 -out jwt-key.pem

# Create secret
kubectl create secret generic my-jwt-key \
  --from-file=jwt-key.pem=jwt-key.pem
```

Configure in values:
```yaml
config:
  auth:
    jwt_private_key: ""
    jwt_private_key_secret:
      name: "my-jwt-key"
      key: "jwt-key.pem"
```

**Important Notes:**
- Ed25519 keys are recommended over RSA for better performance and security
- The auto-generated key persists across upgrades, so tokens remain valid
- For multi-replica deployments, all pods share the same key from the secret
- The secret is marked with `helm.sh/resource-policy: keep` to prevent deletion during uninstall

#### OIDC Client Secret

For OIDC credentials, you can similarly use secrets:

**Method 1: Let the chart create the secret**

```yaml
config:
  auth:
    oidc:
      enabled: true
      client_secret: "your-oidc-secret"
      # Chart will create a secret automatically
```

**Method 2: Use an existing secret**

```bash
kubectl create secret generic garage-oidc \
  --from-literal=client-secret='your-oidc-secret'
```

```yaml
config:
  auth:
    oidc:
      enabled: true
      client_secret: ""  # Leave empty when using existingSecret
      existingSecret:
        name: "garage-oidc"
        key: "client-secret"
```

### Network Policies

Enable network policies for enhanced security:

```yaml
networkPolicy:
  enabled: true
  policyTypes:
    - Ingress
    - Egress
```

This restricts network traffic to/from the pods. Requires a CNI that supports NetworkPolicy (e.g., Calico, Cilium).

### Custom Container Images

Use a custom or private registry:

```yaml
image:
  repository: myregistry.example.com/garage-ui
  tag: "v1.0.0"
  pullPolicy: Always

imagePullSecrets:
  - name: myregistrykey
```

## Version Compatibility

| Chart Version | App Version | Kubernetes | Garage |
|--------------|-------------|------------|--------|
| 0.1.1        | v0.0.6      | 1.19+      | 0.8+   |

## Additional Resources

- **GitHub Repository**: https://github.com/Noooste/garage-ui
- **Garage Documentation**: https://garagehq.deuxfleurs.fr/
- **Issue Tracker**: https://github.com/Noooste/garage-ui/issues
- **Helm Documentation**: https://helm.sh/docs/

## Support

For help and support:

1. **Documentation**: Check this README and the [values.yaml](values.yaml) file
2. **GitHub Issues**: Report bugs or request features at https://github.com/Noooste/garage-ui/issues
3. **Garage Community**: Visit https://garagehq.deuxfleurs.fr/ for Garage-specific questions

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## License

This Helm chart is open source and distributed under the same license as Garage UI.
