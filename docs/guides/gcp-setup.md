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

```bash
echo -n "your-password" | gcloud secrets create folio-password \
  --data-file=- --project=$PROJECT_ID

echo -n "$(openssl rand -hex 32)" | gcloud secrets create folio-hmac-key \
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
gcloud secrets add-iam-policy-binding folio-password \
  --member="serviceAccount:$SA_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor" \
  --project=$PROJECT_ID

gcloud secrets add-iam-policy-binding folio-hmac-key \
  --member="serviceAccount:$SA_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor" \
  --project=$PROJECT_ID
```

## Deploy to Cloud Run

```bash
export SERVICE_NAME=folio-server

gcloud run deploy $SERVICE_NAME \
  --image=ghcr.io/charliek/folio:latest \
  --service-account=$SA_NAME@$PROJECT_ID.iam.gserviceaccount.com \
  --set-env-vars=GCS_BUCKET=$BUCKET_NAME \
  --set-secrets=LOGIN_PASSWORD=folio-password:latest,COOKIE_HMAC_KEY=folio-hmac-key:latest \
  --region=$REGION \
  --project=$PROJECT_ID \
  --min-instances=0 \
  --max-instances=2 \
  --memory=256Mi \
  --allow-unauthenticated
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
