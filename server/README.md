# harpoon-share

The tiny Cloudflare Worker behind `harpoon share`: hosts HTML reports and
serves them at public URLs. Free tier, no maintenance, ~200 lines of JS.

## API

| Method | Path | Purpose |
|---|---|---|
| `POST /api/reports` | Upload. Body: `{"html": "<!doctype html>...", "collection": "optional-name"}` → `{id, url, delete_token, expires_at}` |
| `GET /r/{id}` | View the report (served with `Content-Security-Policy: sandbox`) |
| `DELETE /r/{id}` | Revoke. Header: `X-Delete-Token: <delete_token from upload>` |

Limits: 1 MiB per report, 20 uploads per hour per IP, reports expire after 90 days.

## Deploy your own instance

1. Create a free [Cloudflare](https://dash.cloudflare.com/sign-up) account (no credit card needed).
2. Install the deploy tool: `npm install -g wrangler`
3. Log in: `wrangler login`
4. Create the storage namespace:
   ```sh
   cd server
   wrangler kv namespace create REPORTS
   ```
5. Paste the returned `id` into `wrangler.toml` (replace `REPLACE_WITH_NAMESPACE_ID`).
6. Deploy:
   ```sh
   wrangler deploy
   ```
7. Note the printed URL (e.g. `https://harpoon-share.<your-subdomain>.workers.dev`) —
   that's what `harpoon share --server` and the GitHub Action's `share-server`
   input point to.

## Smoke test after deploy

```sh
curl -sf -X POST "https://<your-worker>/api/reports" \
  -H "Content-Type: application/json" \
  -d "$(jq -n --rawfile html ../docs/ci-report.png.html 2>/dev/null \
      || echo '{"html":"<!doctype html><html><body>smoke</body></html>"}')" \
  | jq .
```

Simpler: run `harpoon share <some-report.html> --server https://<your-worker>`.

## Self-hosting notes

The client defaults to the maintainer's instance but accepts `--server` /
`HARPOON_SHARE_SERVER`, so anyone can run this Worker on their own account —
no code changes needed.
