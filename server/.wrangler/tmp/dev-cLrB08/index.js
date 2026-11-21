var __defProp = Object.defineProperty;
var __name = (target, value) => __defProp(target, "name", { value, configurable: true });

// src/index.js
var MAX_HTML_BYTES = 1 << 20;
var TTL_SECONDS = 90 * 24 * 3600;
var RATE_LIMIT_PER_HOUR = 20;
var ID_LENGTH = 8;
var ID_ALPHABET = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";
var src_default = {
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
  }
};
async function upload(request, env) {
  const ip = request.headers.get("CF-Connecting-IP") || "unknown";
  if (!await allowUpload(env, ip)) {
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
      createdAt: (/* @__PURE__ */ new Date()).toISOString()
    }
  });
  const origin = new URL(request.url).origin;
  return json(
    {
      id,
      url: `${origin}/r/${id}`,
      delete_token: deleteToken,
      expires_at: new Date(Date.now() + TTL_SECONDS * 1e3).toISOString()
    },
    201
  );
}
__name(upload, "upload");
async function serve(env, id) {
  if (!validId(id)) return json({ error: "invalid id" }, 400);
  const html = await env.REPORTS.get("r:" + id);
  if (!html) return page("Report not found \u2014 it may have expired or been revoked.", 404);
  return new Response(html, {
    headers: {
      "Content-Type": "text/html; charset=utf-8",
      "Content-Security-Policy": "sandbox",
      "X-Content-Type-Options": "nosniff",
      "Cache-Control": "no-store"
    }
  });
}
__name(serve, "serve");
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
__name(revoke, "revoke");
async function allowUpload(env, ip) {
  const key = "rl:" + ip;
  const current = parseInt(await env.REPORTS.get(key) || "0", 10);
  if (current >= RATE_LIMIT_PER_HOUR) return false;
  await env.REPORTS.put(key, String(current + 1), { expirationTtl: 3600 });
  return true;
}
__name(allowUpload, "allowUpload");
async function freshId(env) {
  for (; ; ) {
    const id = randomToken(ID_LENGTH, ID_ALPHABET);
    if (!await env.REPORTS.get("r:" + id)) return id;
  }
}
__name(freshId, "freshId");
function randomToken(length, alphabet = ID_ALPHABET) {
  const bytes = crypto.getRandomValues(new Uint8Array(length));
  let out = "";
  for (const b of bytes) out += alphabet[b % alphabet.length];
  return out;
}
__name(randomToken, "randomToken");
function validId(id) {
  return new RegExp(`^[${ID_ALPHABET}]{${ID_LENGTH}}$`).test(id);
}
__name(validId, "validId");
function json(body, status = 200) {
  return new Response(JSON.stringify(body) + "\n", {
    status,
    headers: { "Content-Type": "application/json" }
  });
}
__name(json, "json");
function page(message, status) {
  return new Response(
    `<!doctype html><html><head><meta charset="utf-8"><title>harpoon share</title>
<style>body{background:#0d1117;color:#e6edf3;font-family:Consolas,monospace;display:grid;place-items:center;height:100vh;margin:0}p{color:#8b949e}</style>
</head><body><div><h1>\u{1F3AF} harpoon share</h1><p>${message}</p></div></body></html>`,
    { status, headers: { "Content-Type": "text/html; charset=utf-8" } }
  );
}
__name(page, "page");

// ../../../../../AppData/Roaming/npm/node_modules/wrangler/templates/middleware/middleware-ensure-req-body-drained.ts
var drainBody = /* @__PURE__ */ __name(async (request, env, _ctx, middlewareCtx) => {
  try {
    return await middlewareCtx.next(request, env);
  } finally {
    try {
      if (request.body !== null && !request.bodyUsed) {
        const reader = request.body.getReader();
        while (!(await reader.read()).done) {
        }
      }
    } catch (e) {
      console.error("Failed to drain the unused request body.", e);
    }
  }
}, "drainBody");
var middleware_ensure_req_body_drained_default = drainBody;

// ../../../../../AppData/Roaming/npm/node_modules/wrangler/templates/middleware/middleware-miniflare3-json-error.ts
function reduceError(e) {
  return {
    name: e?.name,
    message: e?.message ?? String(e),
    stack: e?.stack,
    cause: e?.cause === void 0 ? void 0 : reduceError(e.cause)
  };
}
__name(reduceError, "reduceError");
var jsonError = /* @__PURE__ */ __name(async (request, env, _ctx, middlewareCtx) => {
  try {
    return await middlewareCtx.next(request, env);
  } catch (e) {
    const error = reduceError(e);
    const body = JSON.stringify(error);
    const headers = {
      "Content-Type": "application/json",
      "MF-Experimental-Error-Stack": "true"
    };
    const encoded = encodeURIComponent(body);
    if (encoded.length <= 8192) {
      headers["MF-Experimental-Error-Stack-Payload"] = encoded;
    }
    return new Response(body, { status: 500, headers });
  }
}, "jsonError");
var middleware_miniflare3_json_error_default = jsonError;

// .wrangler/tmp/bundle-6EvLjg/middleware-insertion-facade.js
var __INTERNAL_WRANGLER_MIDDLEWARE__ = [
  middleware_ensure_req_body_drained_default,
  middleware_miniflare3_json_error_default
];
var middleware_insertion_facade_default = src_default;

// ../../../../../AppData/Roaming/npm/node_modules/wrangler/templates/middleware/common.ts
var __facade_middleware__ = [];
function __facade_register__(...args) {
  __facade_middleware__.push(...args.flat());
}
__name(__facade_register__, "__facade_register__");
function __facade_invokeChain__(request, env, ctx, dispatch, middlewareChain) {
  const [head, ...tail] = middlewareChain;
  const middlewareCtx = {
    dispatch,
    next(newRequest, newEnv) {
      return __facade_invokeChain__(newRequest, newEnv, ctx, dispatch, tail);
    }
  };
  return head(request, env, ctx, middlewareCtx);
}
__name(__facade_invokeChain__, "__facade_invokeChain__");
function __facade_invoke__(request, env, ctx, dispatch, finalMiddleware) {
  return __facade_invokeChain__(request, env, ctx, dispatch, [
    ...__facade_middleware__,
    finalMiddleware
  ]);
}
__name(__facade_invoke__, "__facade_invoke__");

// .wrangler/tmp/bundle-6EvLjg/middleware-loader.entry.ts
var __Facade_ScheduledController__ = class ___Facade_ScheduledController__ {
  constructor(scheduledTime, cron, noRetry) {
    this.scheduledTime = scheduledTime;
    this.cron = cron;
    this.#noRetry = noRetry;
  }
  scheduledTime;
  cron;
  static {
    __name(this, "__Facade_ScheduledController__");
  }
  #noRetry;
  noRetry() {
    if (!(this instanceof ___Facade_ScheduledController__)) {
      throw new TypeError("Illegal invocation");
    }
    this.#noRetry();
  }
};
function wrapExportedHandler(worker) {
  if (__INTERNAL_WRANGLER_MIDDLEWARE__ === void 0 || __INTERNAL_WRANGLER_MIDDLEWARE__.length === 0) {
    return worker;
  }
  for (const middleware of __INTERNAL_WRANGLER_MIDDLEWARE__) {
    __facade_register__(middleware);
  }
  const fetchDispatcher = /* @__PURE__ */ __name(function(request, env, ctx) {
    if (worker.fetch === void 0) {
      throw new Error("Handler does not export a fetch() function.");
    }
    return worker.fetch(request, env, ctx);
  }, "fetchDispatcher");
  return {
    ...worker,
    fetch(request, env, ctx) {
      const dispatcher = /* @__PURE__ */ __name(function(type, init) {
        if (type === "scheduled" && worker.scheduled !== void 0) {
          const controller = new __Facade_ScheduledController__(
            Date.now(),
            init.cron ?? "",
            () => {
            }
          );
          return worker.scheduled(controller, env, ctx);
        }
      }, "dispatcher");
      return __facade_invoke__(request, env, ctx, dispatcher, fetchDispatcher);
    }
  };
}
__name(wrapExportedHandler, "wrapExportedHandler");
function wrapWorkerEntrypoint(klass) {
  if (__INTERNAL_WRANGLER_MIDDLEWARE__ === void 0 || __INTERNAL_WRANGLER_MIDDLEWARE__.length === 0) {
    return klass;
  }
  for (const middleware of __INTERNAL_WRANGLER_MIDDLEWARE__) {
    __facade_register__(middleware);
  }
  return class extends klass {
    #fetchDispatcher = /* @__PURE__ */ __name((request, env, ctx) => {
      this.env = env;
      this.ctx = ctx;
      if (super.fetch === void 0) {
        throw new Error("Entrypoint class does not define a fetch() function.");
      }
      return super.fetch(request);
    }, "#fetchDispatcher");
    #dispatcher = /* @__PURE__ */ __name((type, init) => {
      if (type === "scheduled" && super.scheduled !== void 0) {
        const controller = new __Facade_ScheduledController__(
          Date.now(),
          init.cron ?? "",
          () => {
          }
        );
        return super.scheduled(controller);
      }
    }, "#dispatcher");
    fetch(request) {
      return __facade_invoke__(
        request,
        this.env,
        this.ctx,
        this.#dispatcher,
        this.#fetchDispatcher
      );
    }
  };
}
__name(wrapWorkerEntrypoint, "wrapWorkerEntrypoint");
var WRAPPED_ENTRY;
if (typeof middleware_insertion_facade_default === "object") {
  WRAPPED_ENTRY = wrapExportedHandler(middleware_insertion_facade_default);
} else if (typeof middleware_insertion_facade_default === "function") {
  WRAPPED_ENTRY = wrapWorkerEntrypoint(middleware_insertion_facade_default);
}
var middleware_loader_entry_default = WRAPPED_ENTRY;
export {
  __INTERNAL_WRANGLER_MIDDLEWARE__,
  middleware_loader_entry_default as default
};
//# sourceMappingURL=index.js.map
