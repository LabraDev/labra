# Labra

Monorepo:
- `labra-backend` (Go API + GitHub OAuth session auth)
- `labra-frontend` (SvelteKit app)
- `labra-infra` (Terraform AWS stack)

## Run Locally

```bash
cd labra-backend
cp .env.example .env
go run ./cmd
```

```bash
cd labra-frontend
npm install
npm run dev
```

## Validate

```bash
cd labra-backend
go test ./...
go vet ./...

cd ../labra-frontend
npm run check
npm test -- --run
npm run build
```

## Deploy

Infra baseline:
```bash
./infra-up.sh --yes
```

Full cloud deploy:
```bash
./cloud-up.sh --yes
```

## OpenAPI

OpenAPI is generated in CI as an artifact (not versioned in git). Manual generation:

```bash
./labra-backend/scripts/generate-openapi.sh
```
