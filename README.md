# Labra

A self-hosted platform for managing GitHub App deployments and cloud infrastructure. The backend is a Go API using GitHub OAuth for auth, the frontend is SvelteKit, and the infra layer is Terraform on AWS.

```text
labra-backend/   — Go API (port 8080 by default)
labra-frontend/  — SvelteKit app
labra-infra/     — Terraform AWS stack
```

---

## Prerequisites

- Go 1.22+
- A C compiler — required by the SQLite driver (`go-sqlite3` uses CGo)
  - macOS: `xcode-select --install`
  - Linux: `sudo apt install build-essential` (Debian/Ubuntu) or `sudo dnf install gcc` (Fedora/RHEL)
- Node 20+ and npm
- A GitHub account to create the OAuth App and GitHub App below
- AWS CLI + Terraform (only needed for cloud deploy)

---

## Local Setup

### 1. Create a GitHub OAuth App

Go to **GitHub → Settings → Developer settings → OAuth Apps → New OAuth App**.

| Field | Value |
| --- | --- |
| Homepage URL | `http://localhost:5173` |
| Authorization callback URL | `http://localhost:8080/auth/github/callback` |

Save the **Client ID** and generate a **Client Secret**.

### 2. Create a GitHub App

Go to **GitHub → Settings → Developer settings → GitHub Apps → New GitHub App**.

- Set the callback URL to `http://localhost:8080`
- Grant whatever repository permissions your workflows need
- Generate and download a **private key** (`.pem` file)

Note the **App ID** and **App slug** (the URL-safe name shown in the app's settings URL).

### 3. Configure the backend

```bash
cd labra-backend
cp .env.example .env
```

Fill in the values:

```env
GH_CLIENT_ID=           # OAuth App client ID
GH_CLIENT_SECRET=       # OAuth App client secret
GH_APP_ID=              # GitHub App ID (numeric)
GH_APP_SLUG=            # GitHub App slug
GH_APP_PRIVATE_KEY_PEM= # Private key contents — see note below
DB_URL=./labra.db       # SQLite path; default is fine for local dev
GITHUB_WEBHOOK_SECRET=  # Any random string — set the same value in your GitHub App webhook config
GITHUB_OAUTH_REDIRECT_URL=http://localhost:8080/auth/github/callback

JWT_ISSUER=labra-local
JWT_AUDIENCE=labra-local
JWT_SIGNING_SECRET=     # Any random string; used to sign session tokens

AI_DISABLED=true        # Remove or set to false if you have AWS Bedrock access
```

**Inlining the PEM key:** the value must be a single line with literal `\n` between each line of the key:

```bash
awk 'NF {printf "%s\\n", $0}' your-app.private-key.pem
```

Paste the output as the value of `GH_APP_PRIVATE_KEY_PEM`.

### 4. Run the backend

```bash
cd labra-backend
go run ./cmd
# Listening on http://localhost:8080
```

Migrations run automatically on startup — no manual DB setup needed.

### 5. Configure the frontend

Create `labra-frontend/.env.local`:

```env
VITE_BACKEND_BASE_URL=http://localhost:8080
```

`VITE_PLATFORM_PRINCIPAL_ARN` and `VITE_CLOUDFORMATION_TEMPLATE_S3_URL` are only needed once you have AWS infra running; leave them unset for local dev.

### 6. Run the frontend

```bash
cd labra-frontend
npm install
npm run dev
# App at http://localhost:5173
```

---

## Tests & Checks

```bash
# Backend
cd labra-backend
go test ./...
go vet ./...

# Frontend
cd labra-frontend
npm run check
npm test -- --run
npm run build
```

---

## Cloud Deploy

Provision the AWS infra baseline first, then deploy the full stack:

```bash
./infra-up.sh --yes
./cloud-up.sh --yes
```

Omit `--yes` to step through each confirmation interactively.

---

## OpenAPI

The spec is generated in CI and not committed to git. To generate it locally:

```bash
./labra-backend/scripts/generate-openapi.sh
# Output: labra-backend/dist/openapi.json
```
