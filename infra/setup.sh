#!/usr/bin/env bash
# =============================================================================
# Folio GCP Infrastructure Setup
# =============================================================================
#
# This script provisions the GCP resources needed to run folio.
# It is meant to be read and run step-by-step, not executed blindly.
#
# Prerequisites:
#   - gcloud CLI authenticated with a project owner account
#   - A GCP project with billing enabled
#
# =============================================================================

set -euo pipefail

# -----------------------------------------------------------------------------
# Configuration — edit these values
# -----------------------------------------------------------------------------

PROJECT_ID="your-gcp-project"
REGION="us-central1"
BUCKET_NAME="your-docs-bucket"
SERVICE_NAME="folio-server"
DOMAIN="docs.example.com"
GITHUB_ORG="your-github-username"

# Service account names
SERVER_SA="folio-server"
CI_SA="folio-ci"

# Workload Identity Federation
WIF_POOL="github-pool"
WIF_PROVIDER="github-provider"

# -----------------------------------------------------------------------------
# 1. Create GCS Bucket
# -----------------------------------------------------------------------------

echo "Creating GCS bucket..."
gcloud storage buckets create "gs://$BUCKET_NAME" \
  --project="$PROJECT_ID" \
  --location="$REGION" \
  --uniform-bucket-level-access

# -----------------------------------------------------------------------------
# 2. Create Secret Manager Secrets
# -----------------------------------------------------------------------------

echo "Creating secrets..."

# Secrets follow the {env}-{app}-{purpose} convention.
LOGIN_PASSWORD_SECRET="prod-folio-server-login-password"
HMAC_KEY_SECRET="prod-folio-server-hmac-key"

# Generate a strong HMAC key.
HMAC_KEY=$(openssl rand -hex 32)

read -rsp "Enter login password: " FOLIO_PASSWORD
echo
echo -n "$FOLIO_PASSWORD" | gcloud secrets create "$LOGIN_PASSWORD_SECRET" \
  --data-file=- \
  --project="$PROJECT_ID"

echo -n "$HMAC_KEY" | gcloud secrets create "$HMAC_KEY_SECRET" \
  --data-file=- \
  --project="$PROJECT_ID"

# -----------------------------------------------------------------------------
# 3. Create Server Service Account
# -----------------------------------------------------------------------------

echo "Creating server service account..."
gcloud iam service-accounts create "$SERVER_SA" \
  --display-name="Folio Server" \
  --project="$PROJECT_ID"

SERVER_SA_EMAIL="$SERVER_SA@$PROJECT_ID.iam.gserviceaccount.com"

# Grant GCS read access.
gcloud storage buckets add-iam-policy-binding "gs://$BUCKET_NAME" \
  --member="serviceAccount:$SERVER_SA_EMAIL" \
  --role="roles/storage.objectViewer"

# Grant secret access.
for SECRET in "$LOGIN_PASSWORD_SECRET" "$HMAC_KEY_SECRET"; do
  gcloud secrets add-iam-policy-binding "$SECRET" \
    --member="serviceAccount:$SERVER_SA_EMAIL" \
    --role="roles/secretmanager.secretAccessor" \
    --project="$PROJECT_ID"
done

# -----------------------------------------------------------------------------
# 4. Create CI Service Account (for GitHub Actions)
# -----------------------------------------------------------------------------

echo "Creating CI service account..."
gcloud iam service-accounts create "$CI_SA" \
  --display-name="Folio CI" \
  --project="$PROJECT_ID"

CI_SA_EMAIL="$CI_SA@$PROJECT_ID.iam.gserviceaccount.com"

# Grant GCS write access for publishing.
gcloud storage buckets add-iam-policy-binding "gs://$BUCKET_NAME" \
  --member="serviceAccount:$CI_SA_EMAIL" \
  --role="roles/storage.objectAdmin"

# Grant Cloud Run deploy access.
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:$CI_SA_EMAIL" \
  --role="roles/run.developer"

gcloud iam service-accounts add-iam-policy-binding "$SERVER_SA_EMAIL" \
  --member="serviceAccount:$CI_SA_EMAIL" \
  --role="roles/iam.serviceAccountUser"

# -----------------------------------------------------------------------------
# 5. Set Up Workload Identity Federation
# -----------------------------------------------------------------------------

echo "Setting up Workload Identity Federation..."

gcloud iam workload-identity-pools create "$WIF_POOL" \
  --location=global \
  --project="$PROJECT_ID"

gcloud iam workload-identity-pools providers create-oidc "$WIF_PROVIDER" \
  --location=global \
  --workload-identity-pool="$WIF_POOL" \
  --issuer-uri="https://token.actions.githubusercontent.com" \
  --attribute-mapping="google.subject=assertion.sub,attribute.repository_owner=assertion.repository_owner" \
  --attribute-condition="assertion.repository_owner == '$GITHUB_ORG'" \
  --project="$PROJECT_ID"

PROJECT_NUMBER=$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)')

gcloud iam service-accounts add-iam-policy-binding "$CI_SA_EMAIL" \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/projects/$PROJECT_NUMBER/locations/global/workloadIdentityPools/$WIF_POOL/attribute.repository_owner/$GITHUB_ORG"

# Print the WIF provider resource name for GitHub Actions secrets.
echo ""
echo "=== GitHub Actions Secrets ==="
echo "GCP_WIF_PROVIDER: projects/$PROJECT_NUMBER/locations/global/workloadIdentityPools/$WIF_POOL/providers/$WIF_PROVIDER"
echo "GCP_SA_EMAIL: $CI_SA_EMAIL"

# -----------------------------------------------------------------------------
# 6. Deploy to Cloud Run
# -----------------------------------------------------------------------------
#
# The service's env vars, secret bindings, service account, and resource
# limits are declared in deploy/cloud-run-service.yaml. If the SA email,
# bucket name, or secret names in this script differ from the values baked
# into that file, update the template before applying.

echo "Deploying to Cloud Run via deploy/cloud-run-service.yaml..."
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="ghcr.io/charliek/folio:latest"

sed "s|__IMAGE__|$IMAGE|" "$REPO_ROOT/deploy/cloud-run-service.yaml" \
  | gcloud run services replace - \
      --region="$REGION" \
      --project="$PROJECT_ID"

# Allow public ingress (required so the login page is reachable).
gcloud run services add-iam-policy-binding "$SERVICE_NAME" \
  --region="$REGION" --project="$PROJECT_ID" \
  --member=allUsers --role=roles/run.invoker

# -----------------------------------------------------------------------------
# 7. Set Up Domain Mapping
# -----------------------------------------------------------------------------

echo "Creating domain mapping..."
gcloud run domain-mappings create \
  --service="$SERVICE_NAME" \
  --domain="$DOMAIN" \
  --region="$REGION" \
  --project="$PROJECT_ID"

echo ""
echo "=== Done ==="
echo "Add a CNAME record for $DOMAIN pointing to the target shown above."
echo "Visit https://$DOMAIN after DNS propagation."
