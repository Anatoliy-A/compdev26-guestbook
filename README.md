# compdev26-guestbook

A small Go web app for the CompDev2026 AKS lab. Visitors sign a guestbook, and entries are stored in Azure Cosmos DB for NoSQL.

- Authentication is Microsoft Entra ID only, through `DefaultAzureCredential`. On AKS, Workload Identity supplies it. There are no keys or connection strings.
- The image is built and published to GHCR by GitHub Actions. This repository never deploys to Kubernetes; Argo CD does that from `compdev26-gitops`.
- After each push to `main` or a `v*` tag, the workflow opens a pull request in `compdev26-gitops` that sets the new image digest. It only proposes the change; merging that pull request is what deploys. Pushes to `main` that touch only `.github/` or Markdown files are skipped, because those files are excluded from the image; pull-request runs and `v*` tags always run in full. This needs the repository secret `GITOPS_PR_TOKEN`: a fine-grained personal access token limited to the `compdev26-gitops` repository with `Contents` and `Pull requests` read and write.

## Configuration

| Variable | Required | Description |
| --- | --- | --- |
| `COSMOS_ENDPOINT` | yes | Terraform output `cosmos_endpoint` |
| `COSMOS_DATABASE` | yes | Terraform output `cosmos_database_name` |
| `COSMOS_CONTAINER` | yes | Terraform output `cosmos_container_name` (partition key `/guestbookId`) |
| `PORT` | no | Listen port, default `8080` |
| `GUESTBOOK_STORE` | no | Set to `memory` to run without Cosmos DB (local testing only) |

Endpoints: `GET /` (page), `POST /` (form), `GET /api/entries` (JSON), `GET /healthz`, `GET /readyz`.

## Run locally with Docker (no Go install needed)

```powershell
docker build -t guestbook:local .
docker run --rm -p 8080:8080 -e GUESTBOOK_STORE=memory guestbook:local
```

Open <http://localhost:8080>
