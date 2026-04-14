# Deploy Command

Deploy the folio server to Cloud Run.

## Arguments

- Optional first argument: image tag to deploy.

## Steps

1. **Determine image tag**: If no image tag argument provided:
   - Look up the 3 most recent `v*` git tags: `git tag -l "v*" --sort=-v:refname | head -3`
   - Present them via AskUserQuestion with the most recent as the first option marked "(Recommended)"
   - The user can also type a custom tag via the "Other" option

2. **Verify image exists**: Check that the image tag exists on GHCR
   - Run: `gh api -H "Accept: application/vnd.github+json" /orgs/charliek/packages/container/folio/versions --jq '.[].metadata.container.tags[]' 2>/dev/null | grep -q "^TAG$" || gh api -H "Accept: application/vnd.github+json" /users/charliek/packages/container/folio/versions --jq '.[].metadata.container.tags[]' 2>/dev/null | grep -q "^TAG$"`
   - If the tag doesn't exist, inform the user and suggest pushing a `v*` tag to trigger the release workflow first

3. **Trigger deployment**: Start the deploy workflow
   - Run: `gh workflow run deploy.yml -f image_tag=<tag>`
   - Wait a moment for the run to register

4. **Monitor the run**: Watch the workflow execution
   - Get the run ID: `gh run list --workflow=deploy.yml -L 1 --json databaseId -q '.[0].databaseId'`
   - Watch it: `gh run watch <run_id>`

5. **Report result**: Show the deployment outcome
   - If successful: display the service URL
   - If failed: show the failure details and link to the Actions page

## Error Handling

- If `gh` CLI is not authenticated, inform the user to run `gh auth login`
- If the workflow file doesn't exist, inform the user the deploy workflow hasn't been set up yet
- If the deploy fails, provide the link to the GitHub Actions run for debugging
