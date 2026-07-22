(() => {
  const $ = (id) => document.getElementById(id);

  const state = {
    mode: "normal", // normal | text | apps | roku | help
    device: null,
    apps: [],
    devices: [],
    filtered: [],
    selected: 0,
    textEcho: "",
  };

  const el = {
    deviceLabel: $("device-label"),
    modeTag: $("mode-tag"),
    status: $("status"),
    statusMsg: $("status-msg"),
    textBar: $("text-bar"),
    textEcho: $("text-echo"),
    appsPanel: $("apps-panel"),
    appsInput: $("apps-input"),
    appsList: $("apps-list"),
    rokuPanel: $("roku-panel"),
    rokuInput: $("roku-input"),
    rokuList: $("roku-list"),
    helpPanel: $("help-panel"),
    btnHelp: $("btn-help"),
  };

  // --- api ---

  async function api(path, opts = {}) {
    const res = await fetch(path, {
      headers: { "Content-Type": "application/json", ...(opts.headers || {}) },
      ...opts,
    });
    let data = null;
    try {
      data = await res.json();
    } catch {
      data = {};
    }
    if (!res.ok) {
      const err = new Error(data.error || res.statusText || "request failed");
      err.status = res.status;
      throw err;
    }
    return data;
  }

  async function sendKey(key) {
    flashKey(key);
    try {
      await api("/api/key", { method: "POST", body: JSON.stringify({ key }) });
      setStatus("ok", key);
    } catch (e) {
      setStatus(e.status === 403 || /403/.test(e.message) ? "warn" : "err", e.message);
    }
  }

  function litKey(ch) {
    // match server: QueryEscape with + → %20
    let esc = encodeURIComponent(ch).replace(/[!'()*]/g, (c) =>
      "%" + c.charCodeAt(0).toString(16).toUpperCase()
    );
    return "Lit_" + esc;
  }

  async function sendLit(ch) {
    try {
      await api("/api/key", { method: "POST", body: JSON.stringify({ key: litKey(ch) }) });
      state.textEcho += ch;
      el.textEcho.textContent = state.textEcho.slice(-40);
      setStatus("ok", "Lit " + JSON.stringify(ch));
    } catch (e) {
      setStatus("err", e.message);
    }
  }

  // --- status / device ---

  function setStatus(kind, msg) {
    el.status.classList.remove("ok", "warn", "err");
    if (kind) el.status.classList.add(kind);
    el.statusMsg.textContent = msg;
  }

  function renderDevice() {
    const d = state.device;
    if (!d || !d.location) {
      el.deviceLabel.textContent = "no device · press r";
      return;
    }
    const name = d.name || "Roku";
    let host = d.location;
    try {
      host = new URL(d.location).hostname;
    } catch {}
    const mode = d.ecpMode && d.ecpMode !== "enabled" ? ` · ecp:${d.ecpMode}` : "";
    el.deviceLabel.textContent = `${name} · ${host}${mode}`;
    el.deviceLabel.title = d.location + (d.activeApp ? ` · ${d.activeApp}` : "");
  }

  async function refreshDevice() {
    try {
      state.device = await api("/api/device");
      renderDevice();
      if (state.device.ecpMode && state.device.ecpMode !== "enabled") {
        setStatus("warn", "ECP is " + state.device.ecpMode + " — enable Control by mobile apps");
      }
    } catch (e) {
      state.device = null;
      renderDevice();
      setStatus("warn", e.message + " · press r");
    }
  }

  // --- modes ---

  function setMode(mode) {
    state.mode = mode;
    document.body.className = mode === "normal" ? "" : "mode-" + mode;
    el.modeTag.textContent = mode.toUpperCase();

    el.textBar.classList.toggle("hidden", mode !== "text");
    el.appsPanel.classList.toggle("hidden", mode !== "apps");
    el.rokuPanel.classList.toggle("hidden", mode !== "roku");
    el.helpPanel.classList.toggle("hidden", mode !== "help");

    el.appsPanel.setAttribute("aria-hidden", mode !== "apps");
    el.rokuPanel.setAttribute("aria-hidden", mode !== "roku");
    el.helpPanel.setAttribute("aria-hidden", mode !== "help");

    if (mode === "text") {
      state.textEcho = "";
      el.textEcho.textContent = "";
      setStatus("", "text mode");
    }
    if (mode === "apps") openApps();
    if (mode === "roku") openRoku();
    if (mode === "help") setStatus("", "help");
    if (mode === "normal") setStatus("", "ready");
  }

  function exitMode() {
    if (state.mode === "normal") return;
    if (state.mode === "apps") el.appsInput.blur();
    if (state.mode === "roku") el.rokuInput.blur();
    setMode("normal");
  }

  // --- fuzzy ---

  function fuzzyScore(query, name) {
    if (!query) return 1;
    const q = query.toLowerCase();
    const n = name.toLowerCase();
    let qi = 0;
    let score = 0;
    let consec = 0;
    let prev = -2;
    for (let i = 0; i < n.length && qi < q.length; i++) {
      if (n[i] === q[qi]) {
        consec = i === prev + 1 ? consec + 1 : 1;
        score += 1 + consec * 2;
        if (i === 0) score += 4;
        if (i > 0 && /[\s\-_.]/.test(n[i - 1])) score += 3;
        prev = i;
        qi++;
      }
    }
    if (qi < q.length) return 0;
    score -= Math.max(0, n.length - q.length) * 0.01;
    return score;
  }

  function fuzzyFilter(items, query, nameOf) {
    if (!query) return items.map((it, i) => ({ it, i, score: 1 }));
    const out = [];
    for (let i = 0; i < items.length; i++) {
      const score = fuzzyScore(query, nameOf(items[i]));
      if (score > 0) out.push({ it: items[i], i, score });
    }
    out.sort((a, b) => b.score - a.score || nameOf(a.it).length - nameOf(b.it).length);
    return out;
  }

  // --- apps ---

  async function openApps() {
    el.appsInput.value = "";
    el.appsList.innerHTML = '<li class="empty">loading…</li>';
    state.selected = 0;
    try {
      const data = await api("/api/apps");
      state.apps = data.apps || [];
      renderApps();
      requestAnimationFrame(() => el.appsInput.focus());
    } catch (e) {
      el.appsList.innerHTML = `<li class="empty">${escapeHtml(e.message)}</li>`;
      setStatus("err", e.message);
    }
  }

  function renderApps() {
    const q = el.appsInput.value;
    const hits = fuzzyFilter(state.apps, q, (a) => a.name);
    state.filtered = hits.map((h) => h.it);
    if (state.selected >= state.filtered.length) state.selected = Math.max(0, state.filtered.length - 1);
    if (!state.filtered.length) {
      el.appsList.innerHTML = '<li class="empty">no matches</li>';
      return;
    }
    el.appsList.innerHTML = state.filtered
      .map(
        (a, i) =>
          `<li data-i="${i}" class="${i === state.selected ? "active" : ""}"><span class="name">${escapeHtml(
            a.name
          )}</span><span class="meta">${escapeHtml(a.id)}</span></li>`
      )
      .join("");
    const active = el.appsList.querySelector("li.active");
    if (active) active.scrollIntoView({ block: "nearest" });
  }

  async function launchSelected() {
    const app = state.filtered[state.selected];
    if (!app) return;
    try {
      await api("/api/launch", { method: "POST", body: JSON.stringify({ id: app.id }) });
      setStatus("ok", "launch " + app.name);
      setMode("normal");
    } catch (e) {
      setStatus("err", e.message);
    }
  }

  // --- roku devices ---

  async function openRoku() {
    el.rokuInput.value = "";
    el.rokuList.innerHTML = '<li class="empty">discovering…</li>';
    state.selected = 0;
    try {
      const data = await api("/api/devices");
      state.devices = data.devices || [];
      renderRoku();
      requestAnimationFrame(() => el.rokuInput.focus());
      if (!state.devices.length) setStatus("warn", "no devices — check LAN / multicast");
    } catch (e) {
      el.rokuList.innerHTML = `<li class="empty">${escapeHtml(e.message)}</li>`;
      setStatus("err", e.message);
    }
  }

  function renderRoku() {
    const q = el.rokuInput.value;
    const hits = fuzzyFilter(state.devices, q, (d) => `${d.name || ""} ${d.model || ""} ${d.location || ""}`);
    state.filtered = hits.map((h) => h.it);
    if (state.selected >= state.filtered.length) state.selected = Math.max(0, state.filtered.length - 1);
    if (!state.filtered.length) {
      el.rokuList.innerHTML = '<li class="empty">no devices</li>';
      return;
    }
    const cur = state.device && state.device.location;
    el.rokuList.innerHTML = state.filtered
      .map((d, i) => {
        let host = d.location || "";
        try {
          host = new URL(d.location).host;
        } catch {}
        const cls = [i === state.selected ? "active" : "", d.location === cur ? "current" : ""]
          .filter(Boolean)
          .join(" ");
        return `<li data-i="${i}" class="${cls}"><span class="name">${escapeHtml(
          d.name || "Roku"
        )}</span><span class="meta">${escapeHtml((d.model || "") + " " + host)}</span></li>`;
      })
      .join("");
    const active = el.rokuList.querySelector("li.active");
    if (active) active.scrollIntoView({ block: "nearest" });
  }

  async function selectRoku() {
    const d = state.filtered[state.selected];
    if (!d) return;
    try {
      state.device = await api("/api/device", {
        method: "PUT",
        body: JSON.stringify({ location: d.location }),
      });
      renderDevice();
      setStatus("ok", "selected " + (state.device.name || d.location));
      setMode("normal");
    } catch (e) {
      setStatus("err", e.message);
    }
  }

  // --- list nav shared ---

  function moveSelect(delta) {
    if (!state.filtered.length) return;
    state.selected = (state.selected + delta + state.filtered.length) % state.filtered.length;
    if (state.mode === "apps") renderApps();
    if (state.mode === "roku") renderRoku();
  }

  // --- keys ---

  const navMap = {
    h: "Left",
    j: "Down",
    k: "Up",
    l: "Right",
    ArrowLeft: "Left",
    ArrowDown: "Down",
    ArrowUp: "Up",
    ArrowRight: "Right",
  };

  function onKeyDown(e) {
    const inField = e.target.tagName === "INPUT" || e.target.tagName === "TEXTAREA";

    if (state.mode === "help") {
      if (e.key === "?" || e.key === "Escape") {
        e.preventDefault();
        setMode("normal");
      }
      return;
    }

    if (state.mode === "apps" || state.mode === "roku") {
      if (e.key === "Escape") {
        e.preventDefault();
        exitMode();
        return;
      }
      if (e.key === "ArrowDown" || (e.key === "j" && e.ctrlKey)) {
        e.preventDefault();
        moveSelect(1);
        return;
      }
      if (e.key === "ArrowUp" || (e.key === "k" && e.ctrlKey)) {
        e.preventDefault();
        moveSelect(-1);
        return;
      }
      // j/k without ctrl only when input empty (vim-ish)
      if (!e.ctrlKey && !e.metaKey && !e.altKey && inField && e.target.value === "") {
        if (e.key === "j") {
          e.preventDefault();
          moveSelect(1);
          return;
        }
        if (e.key === "k") {
          e.preventDefault();
          moveSelect(-1);
          return;
        }
      }
      if (e.key === "Enter") {
        e.preventDefault();
        if (state.mode === "apps") launchSelected();
        else selectRoku();
        return;
      }
      return; // let input handle typing
    }

    if (state.mode === "text") {
      if (e.key === "Escape") {
        e.preventDefault();
        exitMode();
        return;
      }
      if (e.key === "Backspace") {
        e.preventDefault();
        sendKey("Backspace");
        state.textEcho = state.textEcho.slice(0, -1);
        el.textEcho.textContent = state.textEcho.slice(-40);
        return;
      }
      if (e.key === "Enter") {
        e.preventDefault();
        sendKey("Enter");
        return;
      }
      // printable handled on keyup
      if (e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
        e.preventDefault();
      }
      return;
    }

    // normal mode
    if (inField) return;

    if (e.key === "?" && !e.ctrlKey && !e.metaKey) {
      e.preventDefault();
      setMode(state.mode === "help" ? "normal" : "help");
      return;
    }
    if (e.key === "Escape") {
      e.preventDefault();
      return;
    }
    if (e.key === " " || e.code === "Space") {
      e.preventDefault();
      setMode("text");
      return;
    }
    if (e.key === "a" && !e.ctrlKey && !e.metaKey && !e.altKey) {
      e.preventDefault();
      setMode("apps");
      return;
    }
    if (e.key === "r" && !e.ctrlKey && !e.metaKey && !e.altKey) {
      e.preventDefault();
      setMode("roku");
      return;
    }
    if (e.key === "p" && !e.shiftKey) {
      e.preventDefault();
      sendKey("Play");
      return;
    }
    if (e.key === "f") {
      e.preventDefault();
      sendKey("Fwd");
      return;
    }
    if (e.key === "d") {
      e.preventDefault();
      sendKey("Rev");
      return;
    }
    if (e.key === "b") {
      e.preventDefault();
      sendKey("InstantReplay");
      return;
    }
    if (e.key === "x") {
      e.preventDefault();
      sendKey("Info");
      return;
    }
    if (e.key === "H" || e.key === "Home") {
      e.preventDefault();
      sendKey("Home");
      return;
    }
    if (e.key === "Enter") {
      e.preventDefault();
      sendKey("Select");
      return;
    }
    if (e.key === "Backspace") {
      e.preventDefault();
      sendKey("Back");
      return;
    }
    if (navMap[e.key]) {
      e.preventDefault();
      sendKey(navMap[e.key]);
    }
  }

  function onKeyUp(e) {
    if (state.mode !== "text") return;
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    if (e.key.length === 1) {
      e.preventDefault();
      sendLit(e.key);
    }
  }

  function onPaste(e) {
    if (state.mode !== "text") return;
    const text = (e.clipboardData || window.clipboardData).getData("text");
    if (!text) return;
    e.preventDefault();
    api("/api/text", { method: "POST", body: JSON.stringify({ text }) })
      .then(() => {
        state.textEcho += text;
        el.textEcho.textContent = state.textEcho.slice(-40);
        setStatus("ok", "pasted " + text.length + " chars");
      })
      .catch((err) => setStatus("err", err.message));
  }

  // --- mouse ---

  function flashKey(key) {
    const btn = document.querySelector(`.key[data-key="${cssEscape(key)}"]`);
    if (!btn) return;
    btn.classList.add("flash");
    setTimeout(() => btn.classList.remove("flash"), 100);
  }

  function cssEscape(s) {
    return s.replace(/"/g, '\\"');
  }

  function escapeHtml(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  document.querySelectorAll(".key[data-key]").forEach((btn) => {
    btn.addEventListener("click", () => sendKey(btn.getAttribute("data-key")));
  });
  document.querySelectorAll(".mode-btn").forEach((btn) => {
    btn.addEventListener("click", () => setMode(btn.getAttribute("data-mode")));
  });
  el.btnHelp.addEventListener("click", () => setMode(state.mode === "help" ? "normal" : "help"));

  el.appsInput.addEventListener("input", () => {
    state.selected = 0;
    renderApps();
  });
  el.rokuInput.addEventListener("input", () => {
    state.selected = 0;
    renderRoku();
  });

  el.appsList.addEventListener("click", (e) => {
    const li = e.target.closest("li[data-i]");
    if (!li) return;
    state.selected = +li.getAttribute("data-i");
    launchSelected();
  });
  el.rokuList.addEventListener("click", (e) => {
    const li = e.target.closest("li[data-i]");
    if (!li) return;
    state.selected = +li.getAttribute("data-i");
    selectRoku();
  });

  window.addEventListener("keydown", onKeyDown);
  window.addEventListener("keyup", onKeyUp);
  window.addEventListener("paste", onPaste);

  // boot
  refreshDevice();
})();
