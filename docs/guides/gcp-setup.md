# GCP Setup

Provision the GCP resources needed to run folio.

## Resources

| Resource | Purpose |
|----------|---------|
| GCS Bucket | Stores all documentation sites |
| Cloud Run Service | Runs the folio server |
| Secret Manager | Stores login password and HMAC key |
| Service Account | Used by Cloud Run and GitHub Actions |
| Workload Identity Federation | GitHub Actions OIDC authentication |

## Create the Bucket

```bash
export PROJECT_ID=your-project
export REGION=us-central1
export BUCKET_NAME=your-docs-bucket

gcloud storage buckets create gs://$BUCKET_NAME \
  --project=$PROJECT_ID \
  --location=$REGION \
  --uniform-bucket-level-access
```

## Create Secrets

Secrets follow the `{env}-{app}-{purpose}` naming convention.

```bash
echo -n "your-password" | gcloud secrets create prod-folio-server-login-password \
  --data-file=- --project=$PROJECT_ID

echo -n "$(openssl rand -hex 32)" | gcloud secrets create prod-folio-server-hmac-key \
  --data-file=- --project=$PROJECT_ID
```

## Create Service Account

```bash
export SA_NAME=folio-server

gcloud iam service-accounts create $SA_NAME \
  --display-name="Folio Server" \
  --project=$PROJECT_ID

# Grant GCS read access.
gcloud storage buckets add-iam-policy-binding gs://$BUCKET_NAME \
  --member="serviceAccount:$SA_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/storage.objectViewer"

# Grant secret access.
for SECRET in prod-folio-server-login-password prod-folio-server-hmac-key; do
  gcloud secrets add-iam-policy-binding $SECRET \
    --member="serviceAccount:$SA_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/secretmanager.secretAccessor" \
    --project=$PROJECT_ID
done
```

## Deploy to Cloud Run

The service's env vars, secret bindings, service account, and resource
limits are defined declaratively in [`deploy/cloud-run-service.yaml`](https://github.com/charliek/folio/blob/main/deploy/cloud-run-service.yaml).
Render the template with the target image tag and apply it with
`gcloud run services replace`:

```bash
export IMAGE=ghcr.io/charliek/folio:latest

sed "s|__IMAGE__|$IMAGE|" deploy/cloud-run-service.yaml \
  | gcloud run services replace - --region=$REGION --project=$PROJECT_ID
```

On subsequent deploys, use the `Deploy` GitHub Actions workflow —
it runs the same `sed` + `replace` flow against the tag you pick.

If the service account email, GCS bucket name, or secret names in
your environment differ from the defaults baked into the template,
edit `deploy/cloud-run-service.yaml` before applying.

Allow public ingress (required so the login page can be reached):

```bash
gcloud run services add-iam-policy-binding folio-server \
  --region=$REGION --project=$PROJECT_ID \
  --member=allUsers --role=roles/run.invoker
```

## Set Up Domain Mapping

```bash
export DOMAIN=docs.example.com

gcloud beta run domain-mappings create \
  --service=$SERVICE_NAME \
  --domain=$DOMAIN \
  --region=$REGION \
  --project=$PROJECT_ID
```

Add a CNAME record for your domain pointing to the target shown in the output.

## Workload Identity Federation

Set up WIF so GitHub Actions can authenticate to GCP without service account keys.

```bash
export POOL_NAME=github-pool
export PROVIDER_NAME=github-provider
export GITHUB_ORG=your-github-username

# Create pool.
gcloud iam workload-identity-pools create $POOL_NAME \
  --location=global \
  --project=$PROJECT_ID

# Create provider.
gcloud iam workload-identity-pools providers create-oidc $PROVIDER_NAME \
  --location=global \
  --workload-identity-pool=$POOL_NAME \
  --issuer-uri="https://token.actions.githubusercontent.com" \
  --attribute-mapping="google.subject=assertion.sub,attribute.repository_owner=assertion.repository_owner" \
  --attribute-condition="assertion.repository_owner == '$GITHUB_ORG'" \
  --project=$PROJECT_ID
```

Create a service account for CI and grant it GCS write access:

```bash
export CI_SA_NAME=folio-ci

gcloud iam service-accounts create $CI_SA_NAME \
  --display-name="Folio CI" \
  --project=$PROJECT_ID

gcloud storage buckets add-iam-policy-binding gs://$BUCKET_NAME \
  --member="serviceAccount:$CI_SA_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/storage.objectAdmin"

# Grant Cloud Run deploy access (for the deploy workflow).
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:$CI_SA_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/run.developer"

gcloud iam service-accounts add-iam-policy-binding \
  $SA_NAME@$PROJECT_ID.iam.gserviceaccount.com \
  --member="serviceAccount:$CI_SA_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/iam.serviceAccountUser"

# Allow GitHub Actions to impersonate this SA.
gcloud iam service-accounts add-iam-policy-binding \
  $CI_SA_NAME@$PROJECT_ID.iam.gserviceaccount.com \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/projects/$(gcloud projects describe $PROJECT_ID --format='value(projectNumber)')/locations/global/workloadIdentityPools/$POOL_NAME/attribute.repository_owner/$GITHUB_ORG"
```

Set these as GitHub Actions secrets and variables in your repos:

**Secrets:**

| Secret | Value |
|--------|-------|
| `GCP_WIF_PROVIDER` | `projects/{project-number}/locations/global/workloadIdentityPools/{pool}/providers/{provider}` |
| `GCP_SA_EMAIL` | `folio-ci@{project}.iam.gserviceaccount.com` |

**Variables:**

| Variable | Value |
|----------|-------|
| `GCS_BUCKET` | Your GCS bucket name |

## Subsequent Deployments

After the initial setup, deploy new server versions using the `deploy.yml` workflow:

```bash
gh workflow run deploy.yml -f image_tag=v0.1.0
```

This triggers a manual deployment that updates the Cloud Run service image via WIF authentication.

See the [infra/setup.sh](https://github.com/charliek/folio/blob/main/infra/setup.sh) script for a complete provisioning example.
