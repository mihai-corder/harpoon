/* harpoon web UI — vanilla JS, no build step, embedded in the binary. */

const $ = (sel) => document.querySelector(sel);

let collection = null;
let selected = null;
let results = {};

const STATUS_TEXT = {
  200: "OK", 201: "Created", 204: "No Content", 301: "Moved Permanently",
  302: "Found", 304: "Not Modified", 400: "Bad Request", 401: "Unauthorized",
  403: "Forbidden", 404: "Not Found", 405: "Method Not Allowed",
  409: "Conflict", 422: "Unprocessable Entity", 429: "Too Many Requests",
  500: "Internal Server Error", 502: "Bad Gateway", 503: "Service Unavailable",
};

const escapeHtml = (s) => s.replace(/[&<>"']/g,
  (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));

/* Tokenize raw JSON, escape every piece, then colorize. */
function highlightJSON(src) {
  const re = /("(?:\\.|[^"\\])*")(\s*:)?|\b(true|false|null)\b|(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/g;
  let out = "";
  let last = 0;
  let m;
  while ((m = re.exec(src)) !== null) {
    out += escapeHtml(src.slice(last, m.index));
    if (m[1] !== undefined) {
      if (m[2] !== undefined) {
        out += `<span class="j-key">${escapeHtml(m[1])}</span>${m[2]}`;
      } else {
        out += `<span class="j-str">${escapeHtml(m[1])}</span>`;
      }
    } else if (m[3] !== undefined) {
      out += `<span class="j-bool">${m[3]}</span>`;
    } else {
      out += `<span class="j-num">${m[4]}</span>`;
    }
    last = re.lastIndex;
  }
  return out + escapeHtml(src.slice(last));
}

const statusClass = (s) =>
  s >= 500 ? "status-5xx" : s >= 400 ? "status-4xx" : s >= 300 ? "status-3xx" : "status-2xx";

const fmtBytes = (n) =>
  n < 1024 ? `${n}B` : n < 1048576 ? `${(n / 1024).toFixed(1)}KB` : `${(n / 1048576).toFixed(1)}MB`;

async function init() {
  const res = await fetch("/api/collection");
  collection = await res.json();
  document.title = `harpoon · ${collection.name}`;
  $("#coll-name").textContent = `· ${collection.name} · ${collection.requests.length} requests`;

  const ul = $("#request-list");
  for (const r of collection.requests) {
    const li = document.createElement("li");
    li.dataset.name = r.name;
    li.innerHTML =
      `<span class="method ${r.method}">${r.method}</span>` +
      `<span class="dot" data-dot>·</span>` +
      `<span title="${escapeHtml(r.url)}">${escapeHtml(r.name)}</span>`;
    li.addEventListener("click", () => select(r.name));
    ul.appendChild(li);
  }
  if (collection.requests.length) select(collection.requests[0].name);
}

function select(name) {
  selected = name;
  document.querySelectorAll("#request-list li").forEach((li) =>
    li.classList.toggle("selected", li.dataset.name === name));
  const r = results[name];
  if (r) renderResult(r); else renderEmpty();
}

function renderEmpty() {
  $("#response-body").innerHTML =
    `<p class="dim">No response yet — press Run for "${escapeHtml(selected)}".</p>`;
}

function renderResult(r) {
  const el = $("#response-body");
  let html = `<div><span class="method ${r.method}">${r.method}</span> <span>${escapeHtml(r.url)}</span></div>`;

  if (r.err) {
    html += `<div class="section"><div class="check fail">✗ ${escapeHtml(r.err)}</div></div>`;
    el.innerHTML = html;
    return;
  }

  html +=
    `<div style="margin-top:12px">` +
    `<span class="status-badge ${statusClass(r.status)}">${r.status} ${STATUS_TEXT[r.status] || ""}</span>` +
    `<span class="meta">${r.duration_ms.toFixed(0)}ms · ${fmtBytes((r.body || "").length)}</span></div>`;

  if (r.checks && r.checks.length) {
    html += `<div class="section"><h3>ASSERTIONS</h3>` + r.checks.map((c) =>
      `<div class="check ${c.ok ? "ok" : "fail"}">${c.ok ? "✓" : "✗"} ${escapeHtml(c.detail)}</div>`).join("") + `</div>`;
  }

  if (r.extracted && Object.keys(r.extracted).length) {
    html += `<div class="section extracts"><h3>EXTRACTS</h3>` + Object.entries(r.extracted).map(([k, v]) =>
      `<div><span class="k">+ ${escapeHtml(k)}</span> <span class="dim">=</span> ${escapeHtml(v)}</div>`).join("") + `</div>`;
  }

  if (r.headers && Object.keys(r.headers).length) {
    html += `<div class="section"><h3>HEADERS</h3><table class="headers-table">` +
      Object.entries(r.headers).map(([k, vs]) =>
        `<tr><td>${escapeHtml(k.toLowerCase())}</td><td>${escapeHtml(vs.join(", "))}</td></tr>`).join("") +
      `</table></div>`;
  }

  html += `<div class="section"><h3>BODY</h3>${renderBody(r)}</div>`;
  el.innerHTML = html;
}

function renderBody(r) {
  if (!r.body) return `<pre class="body dim">(empty)</pre>`;
  if (r.body_json) {
    try {
      return `<pre class="body">${highlightJSON(JSON.stringify(JSON.parse(r.body), null, 2))}</pre>`;
    } catch { /* fall through to plain text */ }
  }
  return `<pre class="body">${escapeHtml(r.body)}</pre>`;
}

function setDot(name, r) {
  const li = document.querySelector(`#request-list li[data-name="${CSS.escape(name)}"]`);
  if (!li) return;
  const dot = li.querySelector("[data-dot]");
  const ok = !r.err && !(r.failures || []).length;
  dot.textContent = ok ? "✓" : "✗";
  dot.className = `dot ${ok ? "ok" : "fail"}`;
}

function setStatus(text) {
  $("#status").textContent = text;
}

function setBusy(b) {
  $("#busy").classList.toggle("hidden", !b);
  $("#btn-run").disabled = b;
  $("#btn-run-all").disabled = b;
}

async function run(requests, all) {
  setBusy(true);
  setStatus("running…");
  try {
    const res = await fetch("/api/run", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(all ? { all: true } : { requests }),
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      setStatus(`error: ${body.error || res.status}`);
      return;
    }
    const data = await res.json();
    let passed = 0;
    for (const r of data.results) {
      results[r.name] = r;
      setDot(r.name, r);
      if (!r.err && !(r.failures || []).length) passed++;
    }
    setStatus(`${passed}/${data.results.length} passed`);

    if (all && data.results.length) {
      select(data.results[0].name);
    } else if (selected && results[selected]) {
      renderResult(results[selected]);
    }
  } catch (e) {
    setStatus(`error: ${e}`);
  } finally {
    setBusy(false);
  }
}

async function toggleHistory() {
  const drawer = $("#history-drawer");
  if (!drawer.classList.contains("hidden")) {
    drawer.classList.add("hidden");
    return;
  }

  const res = await fetch("/api/history");
  const entries = await res.json();
  const list = $("#history-list");
  list.innerHTML = entries.length ? "" : `<p class="dim">no runs yet</p>`;

  for (const entry of entries) {
    const div = document.createElement("div");
    div.className = "hist-entry";
    const time = new Date(entry.at).toLocaleTimeString();
    div.innerHTML =
      `<div class="hist-time">${time}</div>` +
      entry.results.map((r) => {
        const ok = !r.err && !(r.failures || []).length;
        return `<div class="check ${ok ? "ok" : "fail"}">` +
          `${ok ? "✓" : "✗"} ${r.method} ${escapeHtml(r.name)} · ${r.status || "-"} · ${r.duration_ms.toFixed(0)}ms</div>`;
      }).join("");
    list.appendChild(div);
  }
  drawer.classList.remove("hidden");
}

$("#btn-run").addEventListener("click", () => selected && run([selected], false));
$("#btn-run-all").addEventListener("click", () => run(null, true));
$("#btn-history").addEventListener("click", toggleHistory);
$("#btn-close-history").addEventListener("click", () => $("#history-drawer").classList.add("hidden"));

init();
