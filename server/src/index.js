// harpoon-share — a tiny Cloudflare Worker that hosts shared harpoon reports.
//
// Design notes:
//   - HTML reports are stored in KV with a 90-day TTL; nothing to maintain.
//   - Uploads are rate-limited per IP and size-capped: this is an open
//     endpoint, so abuse control comes first.
//   - Reports are served with `Content-Security-Policy: sandbox`: uploaded
//     content can never run scripts or touch this origin. harpoon reports
//     carry no JavaScript anyway.
//   - Uploads return a delete token so authors can revoke their own reports.

const MAX_HTML_BYTES = 1 << 20; // 1 MiB
const TTL_SECONDS = 90 * 24 * 3600; // 90 days
const RATE_LIMIT_PER_HOUR = 20;
const ID_LENGTH = 8;
const ID_ALPHABET = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    try {
      if (request.method === "GET" && url.pathname === "/") return landing();
      if (request.method === "POST" && url.pathname === "/api/reports") return await upload(request, env);
      if (request.method === "GET" && url.pathname.startsWith("/r/")) return await serve(env, url.pathname.slice(3));
      if (request.method === "DELETE" && url.pathname.startsWith("/r/")) return await revoke(request, env, url.pathname.slice(3));
      return json({ error: "not found" }, 404);
    } catch {
      return json({ error: "internal error" }, 500);
    }
  },
};

// --- handlers -------------------------------------------------------------

async function upload(request, env) {
  const ip = request.headers.get("CF-Connecting-IP") || "unknown";
  if (!(await allowUpload(env, ip))) {
    return json({ error: "rate limit exceeded (20 uploads/hour), try again later" }, 429);
  }

  const body = await request.json().catch(() => null);
  if (!body || typeof body.html !== "string" || body.html.length === 0) {
    return json({ error: 'body must be JSON: {"html": "<!doctype html>..."}' }, 400);
  }
  if (body.html.length > MAX_HTML_BYTES) {
    return json({ error: `html exceeds the ${MAX_HTML_BYTES} byte limit` }, 413);
  }
  if (!/<html[\s>]/i.test(body.html)) {
    return json({ error: "html must contain an <html> element" }, 400);
  }

  const id = await freshId(env);
  const deleteToken = randomToken(24);
  await env.REPORTS.put("r:" + id, body.html, {
    expirationTtl: TTL_SECONDS,
    metadata: {
      collection: String(body.collection || "").slice(0, 100),
      deleteToken,
      createdAt: new Date().toISOString(),
    },
  });

  const origin = new URL(request.url).origin;
  return json(
    {
      id,
      url: `${origin}/r/${id}`,
      delete_token: deleteToken,
      expires_at: new Date(Date.now() + TTL_SECONDS * 1000).toISOString(),
    },
    201,
  );
}

async function serve(env, id) {
  if (!validId(id)) return json({ error: "invalid id" }, 400);
  const html = await env.REPORTS.get("r:" + id);
  if (!html) return page("Report not found — it may have expired or been revoked.", 404);

  return new Response(html, {
    headers: {
      "Content-Type": "text/html; charset=utf-8",
      "Content-Security-Policy": "sandbox",
      "X-Content-Type-Options": "nosniff",
      "Cache-Control": "no-store",
    },
  });
}

async function revoke(request, env, id) {
  if (!validId(id)) return json({ error: "invalid id" }, 400);
  const stored = await env.REPORTS.getWithMetadata("r:" + id);
  if (!stored.value) return json({ error: "not found" }, 404);

  const token = request.headers.get("X-Delete-Token") || "";
  if (!token || token !== stored.metadata.deleteToken) {
    return json({ error: "forbidden: missing or wrong X-Delete-Token" }, 403);
  }
  await env.REPORTS.delete("r:" + id);
  return json({ deleted: id });
}

// --- helpers --------------------------------------------------------------

// Per-IP hourly counter in KV: good enough for a free-tier public endpoint.
async function allowUpload(env, ip) {
  const key = "rl:" + ip;
  const current = parseInt((await env.REPORTS.get(key)) || "0", 10);
  if (current >= RATE_LIMIT_PER_HOUR) return false;
  await env.REPORTS.put(key, String(current + 1), { expirationTtl: 3600 });
  return true;
}

// 62^8 ≈ 2.3e14 ids; collisions are checked best-effort against KV.
async function freshId(env) {
  for (;;) {
    const id = randomToken(ID_LENGTH, ID_ALPHABET);
    if (!(await env.REPORTS.get("r:" + id))) return id;
  }
}

function randomToken(length, alphabet = ID_ALPHABET) {
  const bytes = crypto.getRandomValues(new Uint8Array(length));
  let out = "";
  for (const b of bytes) out += alphabet[b % alphabet.length];
  return out;
}

function validId(id) {
  return new RegExp(`^[${ID_ALPHABET}]{${ID_LENGTH}}$`).test(id);
}

function json(body, status = 200) {
  return new Response(JSON.stringify(body) + "\n", {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function page(message, status) {
  return new Response(
    `<!doctype html><html><head><meta charset="utf-8"><title>harpoon share</title>
<style>body{background:#0d1117;color:#e6edf3;font-family:Consolas,monospace;display:grid;place-items:center;height:100vh;margin:0}p{color:#8b949e}</style>
</head><body><div><h1>🎯 harpoon share</h1><p>${message}</p></div></body></html>`,
    { status, headers: { "Content-Type": "text/html; charset=utf-8" } },
  );
}
