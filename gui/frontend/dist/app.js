/* moor — GUI mantığı */
// Wails bağıntıları — tembel (lazy) çözüm: her çağrıda window.go.main.App'ten
// alınır, böylece script yükleme sırasına hiç bağımlı olmaz.
const api = new Proxy({}, {
  get(_, name) {
    return (...args) => window.go.gui.App[name](...args);
  },
});

function jlog(msg) {
  try { api.Log(String(msg)); } catch (_) { /* erken aşamada sessiz kal */ }
}

window.addEventListener("error", (e) => {
  jlog("JS HATASI: " + e.message + " @ " + (e.filename || "?") + ":" + e.lineno);
  toast("JS hatası: " + e.message, "err");
});
window.addEventListener("unhandledrejection", (e) => {
  jlog("İŞLENMEMİŞ RED: " + String(e.reason));
  toast("Beklenmeyen hata: " + String(e.reason), "err");
});

let servers = [];
let selectedId = null;
let pendingPing = false;
let editingId = null;
let toastTimer = null;

const $ = (s) => document.querySelector(s);

function el(tag, cls, text) {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text !== undefined && text !== null) e.textContent = text;
  return e;
}

function msg(e) {
  return (e && e.message) ? e.message : String(e);
}

function toast(text, kind) {
  const t = $("#toast");
  t.textContent = text;
  t.className = "toast" + (kind ? " " + kind : "");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => t.classList.add("hidden"), 3000);
}

/* ---------------- veri ---------------- */

async function load(keepSelection) {
  servers = (await api.Servers()) || [];
  if (!servers.some((s) => s.id === selectedId)) {
    selectedId = servers.length ? servers[0].id : null;
  }
  renderAll();
}

async function doPing() {
  if (pendingPing || !servers.length) return;
  pendingPing = true;
  renderAll();
  try {
    const sum = await api.PingAll();
    servers = (await api.Servers()) || [];
    toast(`${sum.online}/${sum.total} sunucu ayakta`, sum.online === sum.total ? "ok" : "");
  } catch (e) {
    toast("ping hatası: " + msg(e), "err");
  } finally {
    pendingPing = false;
    renderAll();
  }
}

function visibleServers() {
  const q = $("#search").value.trim().toLowerCase();
  if (!q) return servers;
  return servers.filter((s) =>
    [s.name, s.host, s.target, (s.tags || []).join(" ")].join(" ").toLowerCase().includes(q));
}

/* ---------------- render ---------------- */

function dotClass(s) {
  if (pendingPing) return "pending";
  if (!s.healthKnown) return "";
  return s.healthy ? "ok" : "fail";
}

function renderAll() {
  renderList();
  renderDetail();
}

function renderList() {
  const wrap = $("#server-list");
  wrap.innerHTML = "";
  const items = visibleServers();

  if (!servers.length) {
    wrap.appendChild(el("div", "empty", "Henüz sunucu yok. “Yeni Sunucu” ile ekleyin."));
    return;
  }
  if (!items.length) {
    wrap.appendChild(el("div", "empty", "Eşleşen sunucu yok"));
    return;
  }

  for (const s of items) {
    const row = el("div", "server-row" + (s.id === selectedId ? " selected" : ""));
    row.dataset.id = s.id;

    row.appendChild(el("span", "dot " + dotClass(s)));

    const body = el("div", "row-body");
    body.appendChild(el("div", "row-name", s.name));
    body.appendChild(el("div", "row-sub", `${s.user}@${s.displayHost}`));
    row.appendChild(body);

    const meta = el("div", "row-meta");
    meta.appendChild(el("span", "badge " + s.authType, s.authLabel));
    if (!pendingPing && s.healthKnown) {
      meta.appendChild(el("span", "latency" + (s.healthy ? "" : " fail"), s.healthy ? s.latencyMs + " ms" : "kapalı"));
    }
    row.appendChild(meta);

    row.onclick = () => { selectedId = s.id; renderAll(); };
    wrap.appendChild(row);
  }
}

function renderDetail() {
  const d = $("#detail");
  const s = servers.find((x) => x.id === selectedId);
  if (!s) {
    d.innerHTML = '<div class="empty-big">Bir sunucu seçin</div>';
    return;
  }

  const statusText = pendingPing ? "pingleniyor…"
    : s.healthKnown ? (s.healthy ? `ayakta · ${s.latencyMs} ms gecikme` : "erişilemiyor")
    : "durum henüz bilinmiyor";

  d.innerHTML = "";
  const card = el("div", "detail-card");

  const head = el("div", "detail-head");
  head.appendChild(el("span", "dot " + dotClass(s)));
  head.appendChild(el("h2", null, s.name));
  card.appendChild(head);

  const st = el("div", "status-line", statusText);
  card.appendChild(st);

  const grid = el("div", "info-grid");
  const add = (k, v) => {
    const i = el("div", "info-item");
    i.appendChild(el("div", "k", k));
    i.appendChild(el("div", "v mono-soft", v || "—"));
    grid.appendChild(i);
  };
  add("Host", s.host);
  add("Port", String(s.port || 22));
  add("Kullanıcı", s.user);
  add("Kimlik", s.authLabel + (s.keyPath ? " · " + s.keyPath : ""));
  if (s.extraSSHArgs) add("Ek SSH arg.", s.extraSSHArgs);
  card.appendChild(grid);

  const tags = s.tags || [];
  if (tags.length) {
    const tr = el("div", "tags-row");
    tags.forEach((t) => tr.appendChild(el("span", "tag", t)));
    card.appendChild(tr);
  }

  if (s.notes) card.appendChild(el("div", "notes-box", s.notes));

  const cmd = el("div", "ssh-cmd");
  cmd.appendChild(el("code", null, s.sshCommand));
  const copyBtn = el("button", "btn ghost", "kopyala");
  copyBtn.onclick = async () => {
    try {
      await window.runtime.ClipboardSetText(s.sshCommand);
      toast("SSH komutu panoya kopyalandı", "ok");
    } catch (e) { toast("kopyalanamadı: " + msg(e), "err"); }
  };
  cmd.appendChild(copyBtn);
  card.appendChild(cmd);

  const actions = el("div", "detail-actions");
  const bTerm = el("button", "btn primary", "⚡ SSH Bağlan");
  bTerm.onclick = () => openEmbeddedTerminal(s);
  const bOpen = el("button", "btn ghost", "Sistem terminalinde aç");
  bOpen.onclick = async () => {
    try { await api.OpenInTerminal(s.id); toast("terminal açılıyor…", "ok"); }
    catch (e) { toast("terminal açılamadı: " + msg(e), "err"); }
  };
  const bTest = el("button", "btn ghost", "✓ SSH testi");
  bTest.onclick = async () => {
    bTest.disabled = true;
    try {
      const r = await api.TestSSH(s.id);
      toast(r.ok ? "SSH oturumu kuruldu ✓" : "bağlanamadı: " + (r.detail || "bilinmeyen hata"), r.ok ? "ok" : "err");
    } catch (e) { toast("test hatası: " + msg(e), "err"); }
    finally { bTest.disabled = false; }
  };
  const bEdit = el("button", "btn ghost", "✎ Düzenle");
  bEdit.onclick = () => openForm(s.id);
  const bDel = el("button", "btn danger", "🗑 Sil");
  bDel.onclick = () => confirmDelete(s);
  actions.append(bTerm, bOpen, bTest, bEdit, bDel);
  card.appendChild(actions);

  d.appendChild(card);
}

/* ---------------- form ---------------- */

function openForm(id) {
  editingId = id || null;
  const form = $("#server-form");
  form.reset();
  $("#form-err").classList.add("hidden");
  $("#form-title").textContent = id ? "Sunucuyu Düzenle" : "Yeni Sunucu";

  const set = (name, value) => { if (value) form.elements[name].value = value; };
  if (!id) {
    form.elements.port.value = "22";
  } else {
    api.GetServer(id).then((full) => {
      set("name", full.name);
      set("host", full.host);
      set("port", String(full.port || 22));
      set("user", full.user);
      set("authType", full.authType);
      set("keyPath", full.keyPath);
      set("password", full.password);
      set("extraSSHArgs", full.extraSSHArgs);
      set("tags", (full.tags || []).join(", "));
      set("notes", full.notes);
    });
  }
  $("#form-overlay").classList.remove("hidden");
  form.elements.name.focus();
}

function closeOverlays() {
  $("#form-overlay").classList.add("hidden");
  $("#confirm-overlay").classList.add("hidden");
  $("#sync-overlay").classList.add("hidden");
}

async function submitForm(e) {
  e.preventDefault();
  jlog("submit başladı");
  try {
    const f = new FormData($("#server-form"));
    // FormData.get() zaten DEĞERİ döndürür (string), element değil.
    const input = {
      id: editingId || "",
      name: f.get("name") || "",
      host: f.get("host") || "",
      port: parseInt(f.get("port") || "0", 10) || 22,
      user: f.get("user") || "",
      authType: f.get("authType") || "",
      keyPath: f.get("keyPath") || "",
      password: f.get("password") || "",
      extraSSHArgs: f.get("extraSSHArgs") || "",
      tags: String(f.get("tags") || "").split(",").map((t) => t.trim()).filter(Boolean),
      notes: f.get("notes") || "",
    };
    jlog("SaveServer çağrılıyor: " + JSON.stringify(input));
    await api.SaveServer(input);
    jlog("SaveServer tamam");
    if (!editingId) selectedId = null; // yeniyi seç (liste başında olacak)
    closeOverlays();
    await load(false);
    toast("Kaydedildi", "ok");
    scheduleAutoSync();
  } catch (err) {
    jlog("SaveServer HATA: " + msg(err));
    const box = $("#form-err");
    box.textContent = msg(err);
    box.classList.remove("hidden");
  }
}

/* ---------------- genel onay ---------------- */

let confirmCb = null;

function askConfirm(title, textHtml, yesLabel, cb) {
  confirmCb = cb || null;
  $("#confirm-title").textContent = title;
  $("#confirm-text").innerHTML = textHtml;
  $("#btn-confirm-yes").textContent = yesLabel || "Onayla";
  $("#confirm-overlay").classList.remove("hidden");
}

async function doConfirmYes() {
  const cb = confirmCb;
  confirmCb = null;
  closeOverlays();
  if (cb) await cb();
}

/* ---------------- silme ---------------- */

function confirmDelete(s) {
  askConfirm("Sunucuyu sil", `<b>${s.name}</b> kalıcı olarak silinecek. Devam edilsin mi?`, "Sil", async () => {
    try {
      await api.DeleteServer(s.id);
      if (selectedId === s.id) selectedId = null;
      await load(false);
      toast("Sunucu silindi", "ok");
      scheduleAutoSync();
    } catch (e) {
      toast("silinemedi: " + msg(e), "err");
    }
  });
}

/* ---------------- gömülü terminal ---------------- */

const XTERM_THEME = {
  background: "#0f1217",
  foreground: "#e6ebf2",
  cursor: "#4c8dff",
  cursorAccent: "#0f1217",
  selectionBackground: "rgba(76,141,255,.30)",
  black: "#1b222c", red: "#f87171", green: "#34d399", yellow: "#fbbf24",
  blue: "#60a5fa", magenta: "#c084fc", cyan: "#22d3ee", white: "#e5e7eb",
  brightBlack: "#64748b", brightRed: "#fca5a5", brightGreen: "#6ee7b7",
  brightYellow: "#fde68a", brightBlue: "#93c5fd", brightMagenta: "#d8b4fe",
  brightCyan: "#67e8f9", brightWhite: "#f8fafc",
};

// Açık terminal sekmeleri. Her sekme kendi PTY oturumunu canlı tutar;
// sekme değişirken diğer oturumlar KAPANMAZ (örn. bir sunucuda güncelleme
// çalışırken diğerine geçilebilir).
const terms = { tabs: [], activeId: null };
// sekme: { serverId, serverName, sessId, xterm, fit, ro, roTimer, handlers, exited, tabEl, paneEl }

function termTabFor(serverId) {
  return terms.tabs.find((t) => t.serverId === serverId) || null;
}
function activeTermTab() {
  return terms.tabs.find((t) => t.sessId === terms.activeId) || null;
}
function syncTermPanes() {
  for (const t of terms.tabs) t.paneEl.classList.toggle("hidden", t.sessId !== terms.activeId);
}

function showTerminalView() {
  document.querySelector(".detail-pane").classList.add("hidden");
  $("#terminal-view").classList.remove("hidden");
}

function hideTerminalView() {
  $("#terminal-view").classList.add("hidden");
  document.querySelector(".detail-pane").classList.remove("hidden");
}

function updateTermButtons() {
  const b = $("#btn-termview");
  const n = terms.tabs.length;
  if (n === 0) { b.classList.add("hidden"); b.textContent = "⌨ Terminal"; }
  else { b.classList.remove("hidden"); b.textContent = `⌨ Terminal (${n})`; }
}

function renderTermTabs() {
  const wrap = $("#term-tabs");
  wrap.innerHTML = "";
  for (const t of terms.tabs) {
    const tabEl = el("div", "term-tab"
      + (t.sessId === terms.activeId ? " active" : "")
      + (t.exited ? " exited" : ""));
    tabEl.appendChild(el("span", "t-name", t.serverName));
    const x = el("button", "t-close", "×");
    x.title = "Bu oturumu kapat";
    x.onclick = (e) => { e.stopPropagation(); closeTermTab(t.sessId); };
    tabEl.appendChild(x);
    tabEl.onclick = () => activateTermTab(t);
    t.tabEl = tabEl;
    wrap.appendChild(tabEl);
  }
}

// Sekmeyi öne al: pane'yi göster, boyutlandır, odak ver. (Oturum zaten canlı.)
function activateTermTab(t) {
  terms.activeId = t.sessId;
  renderTermTabs();
  syncTermPanes();
  requestAnimationFrame(() => {
    try { t.fit.fit(); } catch (_) {}
    api.ResizeTerminal(t.sessId, t.xterm.cols, t.xterm.rows).catch(() => {});
    try { t.xterm.focus(); } catch (_) {}
  });
}

async function startTermSession(s) {
  const host = $("#term-host");
  const paneEl = el("div", "term-pane");
  host.appendChild(paneEl);
  const x = new Terminal({
    theme: XTERM_THEME,
    fontFamily: "'JetBrains Mono', 'Cascadia Code', Menlo, Consolas, monospace",
    fontSize: 13,
    cursorBlink: true,
    scrollback: 5000,
  });
  const fit = new FitAddon.FitAddon();
  x.loadAddon(fit);
  x.open(paneEl);
  try { fit.fit(); } catch (_) {}

  // Kopyala: seçim değiştiğinde panoya (iTerm tarzı) + Ctrl+C (seçim varsa).
  x.onSelectionChange(() => {
    const sel = x.getSelection();
    if (sel) window.runtime.ClipboardSetText(sel).catch(() => {});
  });
  // Yapıştır/kopyala kısayolları (klavye yerleşiminden bağımsız: code tabanlı).
  paneEl.addEventListener("keydown", (e) => {
    if ((!e.ctrlKey && !e.metaKey) || e.altKey) return;
    if (e.code === "KeyV") {
      e.preventDefault();
      e.stopPropagation();
      window.runtime.ClipboardGetText().then((t) => { if (t) x.paste(t); }).catch(() => {});
    } else if (e.code === "KeyC") {
      const sel = x.getSelection();
      if (sel) {
        e.preventDefault();
        e.stopPropagation();
        window.runtime.ClipboardSetText(sel).catch(() => {});
      }
    }
  }, true);
  // Sağ tık menüsü (Wails varsayılan menüyü kapalı tutar; bu tek menüdür).
  paneEl.addEventListener("contextmenu", (e) => {
    e.preventDefault();
    showTermMenu(x, e.clientX, e.clientY);
  });

  let sessId = null;
  try {
    sessId = await api.StartSSHTerminal(s.id, x.cols || 80, x.rows || 24);
  } catch (e) {
    jlog("StartSSHTerminal HATA: " + msg(e));
    // ConPTY başlatılamadıysa (ör. bu Windows sürümünde sahte konsol
    // hatası) kullanıcıyı kilitleme: sistem terminalini aç.
    toast("Gömülü terminal başlatılamadı — sistem terminalinde açılıyor", "err");
    api.OpenInTerminal(s.id).catch((e2) => toast("terminal açılamadı: " + msg(e2), "err"));
    try { x.dispose(); } catch (_) {}
    paneEl.remove();
    return null;
  }

  const t = {
    serverId: s.id, serverName: s.name, sessId, xterm: x, fit,
    ro: null, roTimer: null, handlers: [], exited: false, tabEl: null, paneEl,
  };
  terms.tabs.push(t);
  terms.activeId = sessId;

  // ConPTY başlatma hatası tespiti: sahte konsolun çocuk süreci
    // başlatma sırasında ölürse (ör. 0xC0000142 STATUS_DLL_INIT_FAILED)
    // veri gelmez ve süreç hemen çıkar. Böyle durumlarda oturum asla
    // kurulmaz; kullanıcıyı kilitlememek için sistem terminaline düşeriz.
  let gotData = false;
  const startedAt = Date.now();
  const onData = (d) => { if (d) { gotData = true; x.write(d); } };
  const onExit = (info) => {
    t.exited = true;
    const code = (info && typeof info.code === "number") ? info.code : null;
    const quick = (Date.now() - startedAt) < 2000;
    // 0xC0000142 = ConPTY çocuk başlatma hatası. 255 = ssh'nin kendi
    // hata kodu (gerçek SSH hatası — dış terminal de başarısız olur,
    // otomatik açma). 0 = temiz çıkış.
    const conptyFail = (code === 0xC0000142)
      || (quick && !gotData && code !== null && code !== 0 && code !== 255);
    if (conptyFail) {
      renderTermTabs();
      try {
        x.write("\r\n\x1b[31mGömülü terminal (ConPTY) bu Windows sürümünde başlatılamadı (kod " + code + ").\x1b[0m\r\n\x1b[90mSistem terminalinde açılıyor…\x1b[0m\r\n");
      } catch (_) {}
      toast("Gömülü terminal (ConPTY) çalışmıyor — sistem terminalinde açılıyor", "err");
      api.OpenInTerminal(s.id).catch((e2) => toast("terminal açılamadı: " + msg(e2), "err"));
      return;
    }
    renderTermTabs();
    try {
      x.write("\r\n\x1b[90m— oturum sonlandı" + (code !== null ? ` (kod ${code})` : "") + " — kapatmak için sekmedeki ×\x1b[0m\r\n");
    } catch (_) {}
  };
  window.runtime.EventsOn("term:data:" + sessId, onData);
  t.handlers.push("term:data:" + sessId);
  window.runtime.EventsOn("term:exit:" + sessId, onExit);
  t.handlers.push("term:exit:" + sessId);

  x.onData((d) => { api.WriteTerminal(sessId, d).catch(() => {}); });

  const ro = new ResizeObserver(() => {
    clearTimeout(t.roTimer);
    t.roTimer = setTimeout(() => {
      if (t.sessId !== terms.activeId) return; // gizli pane'leri boyutlandırma
      try { fit.fit(); } catch (_) { return; }
      api.ResizeTerminal(sessId, x.cols, x.rows).catch(() => {});
    }, 100);
  });
  ro.observe(paneEl);
  t.ro = ro;

  renderTermTabs();
  syncTermPanes();
  updateTermButtons();
  requestAnimationFrame(() => {
    try { fit.fit(); } catch (_) {}
    api.ResizeTerminal(sessId, x.cols, x.rows).catch(() => {});
    try { x.focus(); } catch (_) {}
  });
  return t;
}

async function closeTermTab(sessId) {
  const i = terms.tabs.findIndex((t) => t.sessId === sessId);
  if (i < 0) return;
  const t = terms.tabs[i];
  terms.tabs.splice(i, 1);
  for (const ev of t.handlers) { try { window.runtime.EventsOff(ev); } catch (_) {} }
  if (t.ro) t.ro.disconnect();
  clearTimeout(t.roTimer);
  if (t.sessId === terms.activeId) {
    const next = terms.tabs[Math.min(i, terms.tabs.length - 1)];
    terms.activeId = next ? next.sessId : null;
  }
  try { t.xterm.dispose(); } catch (_) {}
  t.paneEl.remove();
  try { await api.CloseTerminal(sessId); } catch (_) {}
  renderTermTabs();
  syncTermPanes();
  updateTermButtons();
  if (terms.activeId) {
    const at = activeTermTab();
    requestAnimationFrame(() => {
      try { at.fit.fit(); } catch (_) {}
      api.ResizeTerminal(at.sessId, at.xterm.cols, at.xterm.rows).catch(() => {});
    });
  } else {
    hideTerminalView();
  }
}

// Terminal sağ tık menüsü: Kopyala / Yapıştır / Temizle.
function showTermMenu(x, cx, cy) {
  const m = $("#ctx-menu");
  m.innerHTML = "";
  const add = (label, fn) => {
    const b = el("button", "ctx-item", label);
    b.onclick = () => { m.classList.add("hidden"); fn(); };
    m.appendChild(b);
  };
  add("Kopyala", () => {
    const s = x.getSelection();
    if (s) window.runtime.ClipboardSetText(s).catch(() => {});
  });
  add("Yapıştır", () => window.runtime.ClipboardGetText().then((t) => { if (t) x.paste(t); }).catch(() => {}));
  add("Temizle", () => { try { x.clear(); } catch (_) {} });
  m.classList.remove("hidden");
  const w = m.offsetWidth || 150, h = m.offsetHeight || 110;
  m.style.left = Math.max(4, Math.min(cx, window.innerWidth - w - 8)) + "px";
  m.style.top = Math.max(4, Math.min(cy, window.innerHeight - h - 8)) + "px";
}

async function openEmbeddedTerminal(s) {
  jlog("openEmbeddedTerminal: " + s.name);
  const existing = termTabFor(s.id);
  showTerminalView();
  if (existing) { activateTermTab(existing); return; } // oturum canlı, sadece sekmeye geç
  await startTermSession(s);
}

// “Oturumu kapat” = aktif sekmeyi kapat (diğer sekmeler etkilenmez).
async function closeEmbeddedTerminal() {
  const at = activeTermTab();
  if (at) await closeTermTab(at.sessId);
  else hideTerminalView();
}

/* ---------------- içe / dışa aktarma ---------------- */

async function importConfig() {
  try {
    const n = await api.ImportSSHConfig();
    toast(n > 0 ? `${n} sunucu ~/.ssh/config'ten eklendi` : "yeni sunucu yok (hepsi zaten listede)", "ok");
    if (n > 0) scheduleAutoSync();
    await load(false);
  } catch (e) {
    toast("içe aktarma hatası: " + msg(e), "err");
  }
}

function restoreFromFile(file) {
  const reader = new FileReader();
  reader.onload = async () => {
    try {
      const res = await api.ImportJSON(String(reader.result));
      toast(`${res.added} geri yüklendi, ${res.skipped} atlandı`, "ok");
      if (res.added > 0) scheduleAutoSync();
      await load(false);
    } catch (e) {
      toast("geri yükleme hatası: " + msg(e), "err");
    }
  };
  reader.readAsText(file, "utf-8");
}

async function exportJSON() {
  try {
    const text = await api.ExportJSON();
    const blob = new Blob([text], { type: "application/json" });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = "moor-sunucular.json";
    document.body.appendChild(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(a.href), 5000);
    toast("JSON yedeği indirildi", "ok");
  } catch (e) {
    toast("dışa aktarma hatası: " + msg(e), "err");
  }
}

/* ---------------- bulut senkron ---------------- */

async function openSyncModal() {
  $("#sync-overlay").classList.remove("hidden");
  try {
    const acc = await api.AccountInfo();
    renderSyncView(acc);
  } catch (e) {
    toast("hesap durumu alınamadı: " + msg(e), "err");
  }
}

// API adresi alanı varsayılan GİZLİ kalır; yalnızca self-hosted (özel)
// adres kullanılıyorsa açılır. Buton iki durum arasında geçiş yapar.
function setUrlFieldVisible(show) {
  $("#url-field").classList.toggle("hidden", !show);
  $("#btn-sync-urltoggle").textContent = show
    ? "moor'un varsayılan API'sini kullan"
    : "Kendi API sunucunuzu kullanın (self-hosted)…";
}

function renderSyncView(acc) {
  const guest = $("#sync-guest");
  const authed = $("#sync-authed");
  $("#sync-err").classList.add("hidden");
  $("#sync-err2").classList.add("hidden");
  $("#sync-pass-form").classList.add("hidden");
  if (!acc.loggedIn) {
    guest.classList.remove("hidden");
    authed.classList.add("hidden");
    const f = $("#sync-login-form");
    const customUrl = !!(acc.serverUrl && acc.defaultServerUrl && acc.serverUrl !== acc.defaultServerUrl);
    if (!f.elements.url.value || customUrl) f.elements.url.value = acc.serverUrl || acc.defaultServerUrl || "";
    setUrlFieldVisible(customUrl);
    if (!f.elements.username.value && acc.username) f.elements.username.value = acc.username;
    return;
  }
  guest.classList.add("hidden");
  authed.classList.remove("hidden");
  let line = "Oturum: <b>" + acc.username + "</b> @ " + acc.serverUrl;
  if (acc.lastSyncAt) {
    line += ` <small>(son eşitleme: ${acc.lastSyncAt}, sürüm ${acc.lastSyncVersion})</small>`;
  }
  $("#sync-account-line").innerHTML = line;
}

async function syncLogin(asRegister) {
  const f = $("#sync-login-form");
  let url = (f.elements.url.value || "").trim();
  if (!url) {
    // Alan gizliyken bile geçerli adresi öğren (varsayılan/saklı).
    try { const acc = await api.AccountInfo(); url = (acc.serverUrl || acc.defaultServerUrl || "").trim(); } catch (_) {}
  }
  const username = (f.elements.username.value || "").trim();
  const password = f.elements.password.value || "";
  if (!url || !username || !password) {
    $("#sync-err").textContent = "tüm alanları doldurun";
    $("#sync-err").classList.remove("hidden");
    return;
  }
  const btns = f.querySelectorAll("button[type=submit], #btn-sync-register");
  btns.forEach((b) => (b.disabled = true));
  try {
    if (asRegister) await api.Register(url, username, password);
    else await api.Login(url, username, password);
    toast(asRegister ? "hesap oluşturuldu ve giriş yapıldı" : "giriş yapıldı", "ok");
    // Yeni oturumda hemen bir eşitleme turu: buluttaki kayıtlı
    // sunucular bu cihaza gelsin (açılışta da autoSyncStartup var).
    try {
      const r = await api.SyncNow();
      if (r.ok) { await load(false); toast("☁ eşitleme: " + r.message, "ok"); }
      else toast("giriş tamam, eşitleme: " + r.message, "err");
    } catch (se) {
      toast("giriş tamam, eşitleme hatası: " + msg(se), "err");
    }
    const acc = await api.AccountInfo();
    renderSyncView(acc);
  } catch (e) {
    $("#sync-err").textContent = msg(e);
    $("#sync-err").classList.remove("hidden");
  } finally {
    btns.forEach((b) => (b.disabled = false));
  }
}

async function syncLogout() {
  try {
    await api.Logout();
    toast("çıkış yapıldı", "ok");
    const acc = await api.AccountInfo();
    renderSyncView(acc);
  } catch (e) {
    toast("çıkış hatası: " + msg(e), "err");
  }
}

async function syncNow() {
  const b = $("#btn-sync-now");
  b.disabled = true;
  b.textContent = "Eşitleniyor…";
  try {
    const r = await api.SyncNow();
    toast(r.message, r.ok ? "ok" : "err");
    if (r.ok) await load(false);
    const acc = await api.AccountInfo();
    renderSyncView(acc);
  } catch (e) {
    toast("eşitleme hatası: " + msg(e), "err");
  } finally {
    b.disabled = false;
    b.textContent = "Şimdi eşitle";
  }
}

function showSyncErr2(text) {
  const b = $("#sync-err2");
  b.textContent = text;
  b.classList.remove("hidden");
}

function togglePassForm() {
  const f = $("#sync-pass-form");
  f.classList.toggle("hidden");
  if (!f.classList.contains("hidden")) {
    f.elements.oldpass.value = "";
    f.elements.newpass.value = "";
    showSyncErr2("");
    $("#sync-err2").classList.add("hidden");
    f.elements.oldpass.focus();
  }
}

async function syncChangePass(e) {
  e.preventDefault();
  const f = $("#sync-pass-form");
  const oldpass = f.elements.oldpass.value || "";
  const newpass = f.elements.newpass.value || "";
  if (!oldpass || !newpass) { showSyncErr2("tüm alanları doldurun"); return; }
  const b = f.querySelector('button[type=submit]');
  b.disabled = true;
  try {
    await api.ChangePassword(oldpass, newpass);
    toast("şifre değiştirildi (diğer oturumlar kapatıldı)", "ok");
    $("#sync-pass-form").classList.add("hidden");
  } catch (err) {
    showSyncErr2(msg(err));
  } finally {
    b.disabled = false;
  }
}

function syncDeleteAccount() {
  askConfirm("Hesabı sil",
    "Bu işlem hesabını <b>ve buluttaki tüm senkron verisini</b> kalıcı olarak siler."
    + " Bu cihazdaki yerel liste etkilenmez. Devam edilsin mi?",
    "Kalıcı olarak sil",
    async () => {
      try {
        await api.DeleteAccount();
        toast("hesap ve bulut verisi silindi", "ok");
        const acc = await api.AccountInfo();
        renderSyncView(acc);
      } catch (e) {
        toast("hesap silinemedi: " + msg(e), "err");
      }
    });
}

/* ---------------- otomatik senkron ---------------- */

let autoSyncTimer = null;

// Yerel bir değişiklikten sonra (kaydet/sil/içe aktar) gecikmeli push.
function scheduleAutoSync() {
  if (autoSyncTimer) return; // zaten kuyrukta
  autoSyncTimer = setTimeout(async () => {
    autoSyncTimer = null;
    try {
      const acc = await api.AccountInfo();
      if (!acc.loggedIn) return;
      const r = await api.SyncNow();
      if (r.ok) { await load(false); toast("☁ eşitleme: " + r.message, "ok"); }
      else jlog("auto-sync: " + r.message);
    } catch (e) { jlog("auto-sync hatası (sessiz): " + msg(e)); }
  }, 1500);
}

// GUI açılışında sessiz pull.
async function autoSyncStartup() {
  try {
    const acc = await api.AccountInfo();
    if (!acc.loggedIn) return;
    const r = await api.SyncNow();
    if (r.ok && (r.localOnly || r.remoteOnly || r.changedBoth)) {
      await load(false);
      toast("☁ eşitleme: " + r.message, "ok");
    } else if (!r.ok) {
      jlog("auto-sync açılış: " + r.message);
    }
  } catch (e) { jlog("auto-sync açılış hatası (sessiz): " + msg(e)); }
}

/* ---------------- başlatma ---------------- */

document.addEventListener("DOMContentLoaded", async () => {
  jlog("DOMContentLoaded — bindings hazır: " + !!(window.go && window.go.gui && window.go.gui.App));
  $("#btn-add").onclick = () => openForm(null);
  $("#btn-ping").onclick = doPing;
  $("#btn-import").onclick = importConfig;
  $("#btn-restore").onclick = () => $("#restore-file").click();
  $("#restore-file").addEventListener("change", (e) => {
    const f = e.target.files && e.target.files[0];
    if (f) restoreFromFile(f);
    e.target.value = "";
  });
  $("#btn-export").onclick = exportJSON;
  $("#btn-sync").onclick = openSyncModal;
  $("#btn-sync-urltoggle").onclick = async () => {
    const field = $("#url-field");
    const willShow = field.classList.contains("hidden");
    setUrlFieldVisible(willShow);
    const f = $("#sync-login-form");
    if (willShow) {
      if (!f.elements.url.value) {
        try { const acc = await api.AccountInfo(); f.elements.url.value = acc.serverUrl || acc.defaultServerUrl || ""; } catch (_) {}
      }
      f.elements.url.focus();
    } else {
      f.elements.url.value = ""; // gizle → sonraki açılışta varsayılana dön
    }
  };
  // Sağ tık menüsü: dışarı tıklama / Escape ile kapanır.
  document.addEventListener("mousedown", (e) => {
    const m = $("#ctx-menu");
    if (!m.classList.contains("hidden") && !m.contains(e.target)) m.classList.add("hidden");
  });
  $("#sync-login-form").addEventListener("submit", (e) => { e.preventDefault(); syncLogin(false); });
  $("#btn-sync-register").onclick = () => syncLogin(true);
  $("#btn-sync-logout").onclick = syncLogout;
  $("#btn-sync-now").onclick = syncNow;
  $("#btn-sync-changepass").onclick = togglePassForm;
  $("#sync-pass-form").addEventListener("submit", syncChangePass);
  $("#btn-sync-deleteacc").onclick = syncDeleteAccount;
  $("#btn-confirm-yes").onclick = doConfirmYes;
  $("#term-back").onclick = hideTerminalView; // oturumlar canlı kalır
  $("#term-close").onclick = closeEmbeddedTerminal;
  $("#btn-termview").onclick = () => {
    showTerminalView();
    const at = activeTermTab();
    if (at) activateTermTab(at);
  };

  $("#server-form").addEventListener("submit", submitForm);
  document.querySelectorAll("[data-close]").forEach((b) => (b.onclick = closeOverlays));
  document.querySelectorAll(".overlay").forEach((o) =>
    o.addEventListener("mousedown", (e) => { if (e.target === o) closeOverlays(); }));

  let searchTimer = null;
  $("#search").addEventListener("input", () => {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(renderList, 120);
  });

  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") { closeOverlays(); $("#ctx-menu").classList.add("hidden"); }
  });

  await load(false);
  doPing();
  autoSyncStartup();

  // MOOR_GUI_AUTOTEST=1 ile başlatıldıysa JS→Go bağıntı zincirini otomatik test et.
  try {
    if ((await api.Env("MOOR_GUI_AUTOTEST")) === "1") runSelfTest();
  } catch (e) { jlog("selftest tetiklenemedi: " + msg(e)); }
});

async function runSelfTest() {
  jlog("=== SELFTEST BAŞLIYOR ===");
  let list = [];
  try {
    list = await api.Servers();
    jlog("selftest Servers() ok, count=" + list.length);
  } catch (e) {
    jlog("selftest Servers() HATA: " + msg(e));
  }
  const probeName = "selftest-probe-" + Date.now();
  try {
    await api.SaveServer({
      id: "", name: probeName, host: "192.0.2.99", port: 22,
      user: "", authType: "", keyPath: "", password: "",
      extraSSHArgs: "", tags: ["selftest"], notes: "otomatik test — silinmeli",
    });
    jlog("selftest SaveServer ok (name=" + probeName + ")");
  } catch (e) {
    jlog("selftest SaveServer HATA: " + msg(e));
  }
  try {
    list = await api.Servers();
    const probe = list.find((s) => s.name === probeName);
    if (!probe) { jlog("selftest probe listede bulunamadı (kayıt işlenmedi mi?)"); return; }
    jlog("selftest probe bulundu id=" + probe.id);
    await api.DeleteServer(probe.id);
    jlog("selftest DeleteServer ok");
  } catch (e) {
    jlog("selftest delete HATA: " + msg(e));
  }

  // --- Gerçek UI akışı: formu aç, doldur, requestSubmit (buton tıklaması ile aynı yol) ---
  const uiName = "selftest-ui-" + Date.now();
  try {
    openForm(null);
    const form = $("#server-form");
    form.elements.name.value = uiName;
    form.elements.host.value = "192.0.2.98";
    jlog("selftest UI: requestSubmit çağrılıyor (name=" + uiName + ")");
    form.requestSubmit();
    // submit asenkron işler; tamamlanması için bekle
    for (let i = 0; i < 50; i++) {
      await new Promise((r) => setTimeout(r, 100));
      list = await api.Servers();
      if (list.some((s) => s.name === uiName)) break;
    }
    list = await api.Servers();
    const uiprobe = list.find((s) => s.name === uiName);
    if (!uiprobe) { jlog("selftest UI: kayıt listede görünmedi!"); return; }
    jlog("selftest UI: kayıt doğrulandı id=" + uiprobe.id);
    $("#form-overlay").classList.add("hidden"); // form kapalı kalmasın
    await api.DeleteServer(uiprobe.id);
    jlog("selftest UI: temizlik tamam");
  } catch (e) {
    jlog("selftest UI HATA: " + msg(e));
  }

  // --- Gömülü terminal altyapısı: yerel kabuk + PTY olay akışı (ağ gerektirmez) ---
  try {
    const marker = "SKPTY_" + Date.now();
    const isWin = (await api.Platform().catch(() => "")) === "windows";
    // Windows'ta kabuk cmd.exe olduğu için ortam değişkeni sözdizimi %VAR%
    // ve komut ayırıcı & kullanılır; POSIX'te $VAR ve ; kullanılır.
    const probeCmd = isWin
      ? `echo ${marker} & echo TERM=%TERM%`
      : `echo ${marker}; echo "TERM=[$TERM]"`;
    const termExpect = isWin ? "TERM=xterm-256color" : "TERM=[xterm-256color]";
    let gotData = "";
    let exited = false;
    const sess = await api.StartLocalShell(80, 24, probeCmd);
    jlog("selftest term: StartLocalShell ok sess=" + sess);
    window.runtime.EventsOn("term:data:" + sess, (d) => { gotData += d; });
    const exitWait = new Promise((resolve) => {
      window.runtime.EventsOn("term:exit:" + sess, () => { exited = true; resolve(); });
      setTimeout(resolve, 6000);
    });
    await exitWait;
    jlog("selftest term: veri=" + JSON.stringify(gotData.slice(0, 300)) + " exit=" + exited);
    if (!gotData.includes(marker)) jlog("selftest term UYARI: marker alınamadı");
    if (!gotData.includes(termExpect)) jlog("selftest term UYARI: TERM=xterm-256color olamadı (raw: " + JSON.stringify(gotData.slice(0, 300)) + ")");
    try { await api.CloseTerminal(sess); } catch (_) {}
    jlog("selftest term: tamam");
  } catch (e) {
    jlog("selftest term HATA: " + msg(e));
  }
  // --- Pano (clipboard) gidiş-dönüş: kopyala/yapıştır altyapısı ---
  try {
    const before = await window.runtime.ClipboardGetText().catch(() => null);
    const probe = "MOORCLIP_" + Date.now();
    await window.runtime.ClipboardSetText(probe);
    const back = await window.runtime.ClipboardGetText();
    jlog("selftest clipboard: roundtrip=" + (back === probe));
    if (before !== null) await window.runtime.ClipboardSetText(before).catch(() => {});
  } catch (e) {
    jlog("selftest clipboard HATA: " + msg(e));
  }

  // --- Senkron/hesap bağıntısı (ağ gerektirmez) ---
  try {
    const acc = await api.AccountInfo();
    if (typeof acc.loggedIn !== "boolean") throw new Error("AccountInfo şekli yanlış");
    jlog("selftest account: loggedIn=" + acc.loggedIn + " user=" + acc.username);
  } catch (e) {
    jlog("selftest account HATA: " + msg(e));
  }
  jlog("=== SELFTEST BİTTİ ===");
}
