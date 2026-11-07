"use strict";
const preferences = window.kianPreferences || {};
const storage = {
  getItem: (key) => preferences[key] ?? null,
  setItem: (key, value) => {
    preferences[key] = String(value);
    window.webkit?.messageHandlers?.kian?.postMessage({
      id: -1,
      action: "preference",
      data: { key, value: String(value) },
    });
  },
  removeItem: (key) => {
    delete preferences[key];
    window.webkit?.messageHandlers?.kian?.postMessage({
      id: -1,
      action: "preference",
      data: { key, value: "" },
    });
  },
};

const $ = (s) => document.querySelector(s),
  content = $("#content");
const state = {
  sessions: [],
  jobs: [],
  live: { active: false, mode: 0 },
  model: { state: "checking" },
  settings: { language: "auto" },
  dependencies: {},
};
let view = "new",
  selected = null,
  seq = 0,
  pending = new Map(),
  files = [],
  urlMeta = null,
  mode = storage.getItem("displayMode") || "clean",
  font = Number(storage.getItem("fontSize")) || 17,
  autoScroll = true,
  pinned = false,
  historyQuery = "",
  sessionGeneration = 0;
const terminal = new Set(["completed", "failed", "cancelled"]);
const escape = (s) =>
  String(s ?? "").replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        c
      ],
  );
const date = (s) =>
  new Date(s).toLocaleDateString(undefined, { month: "short", day: "numeric" });
const clock = (n, full = false) => {
  n = Math.max(0, Math.floor(n || 0));
  const h = Math.floor(n / 3600),
    m = Math.floor(n / 60) % 60,
    s = n % 60;
  return (
    (h || full ? String(h).padStart(2, "0") + ":" : "") +
    String(m).padStart(2, "0") +
    ":" +
    String(s).padStart(2, "0")
  );
};
const duration = (n) => (n > 0 ? clock(n) : "Duration unavailable");
const label = (s) =>
  ({
    live: "Live recording",
    file: "Media file",
    url: "Media URL",
    legacy: "Legacy transcript",
  })[s] || s;
function rpc(action, data) {
  return new Promise((resolve, reject) => {
    if (!window.webkit?.messageHandlers?.kian) {
      reject(Error("Open this interface in the Kian Studio application."));
      return;
    }
    const id = ++seq;
    pending.set(id, { resolve, reject });
    window.webkit.messageHandlers.kian.postMessage({
      id,
      action,
      data: data ?? null,
    });
  });
}
function error(e) {
  $("#error span").textContent = e.message || String(e);
  $("#error").hidden = false;
}
function toast(s) {
  $("#toast").textContent = s;
  $("#toast").style.display = "block";
  clearTimeout(toast.timer);
  toast.timer = setTimeout(() => ($("#toast").style.display = "none"), 2500);
}
async function action(name, data) {
  try {
    return await rpc(name, data);
  } catch (e) {
    error(e);
    return undefined;
  }
}
window.receive = function (event) {
  const d = event.data;
  if (event.type === "reply") {
    if (d.id === 0) {
      applySnapshot(d.result);
      return;
    }
    const p = pending.get(d.id);
    if (p) {
      pending.delete(d.id);
      d.error ? p.reject(Error(d.error)) : p.resolve(d.result);
    }
    return;
  }
  if (event.type === "model") {
    state.model = d;
    sidebar();
    if (view === "settings") renderSettings();
    if (view === "live" && !state.live.active) renderLive();
    return;
  }
  if (event.type === "live") {
    const wasActive = state.live.active;
    state.live = d;
    sidebar();
    if (d.active && (!wasActive || view === "live")) {
      openSession(d.sessionId);
    } else if (view === "live") renderLive();
    if (view === "session") updateRecordingBanner();
    return;
  }
  if (event.type === "jobs") {
    state.jobs = d || [];
    sidebar();
    if (view === "queue") renderQueue();
    return;
  }
  if (event.type === "segment") {
    if (selected?.id === d.sessionId) {
      if (!selected.segments.some((s) => s.id === d.id)) {
        selected.segments.push(d);
        selected.segments.sort((a, b) => a.start - b.start);
        insertSegment(d);
        if (
          autoScroll &&
          !document.querySelector(".segment textarea") &&
          !String(window.getSelection())
        )
          content.scrollTop = content.scrollHeight;
      }
    }
    return;
  }
  if (event.type === "correction") {
    if (selected?.id === d.ID) {
      if (d.Text === null) delete selected.corrections[d.Segment];
      else selected.corrections[d.Segment] = d.Text;
      replaceSegment(d.Segment);
    }
    return;
  }
  if (event.type === "sessions") {
    refreshSessions();
    return;
  }
  if (event.type === "files") {
    setFiles(d);
    return;
  }
  if (event.type === "modelFile") {
    if ($("#model-path")) $("#model-path").value = d[0] || "";
    return;
  }
  if (event.type === "error") {
    error(d);
    return;
  }
  if (event.type === "toast") {
    toast(d);
    return;
  }
  if (event.type === "activity") {
    $("#activity").textContent = d;
    return;
  }
  if (event.type === "resync") {
    rpc("snapshot")
      .then(applySnapshot)
      .then(() => {
        if (selected) openSession(selected.id);
      })
      .catch(error);
  }
};
function applySnapshot(s) {
  if (!s) return;
  Object.assign(state, s);
  sidebar();
  if (!selected && view === "new") {
    const last = storage.getItem("selectedSession");
    if (last && state.sessions.some((s) => s.id === last)) {
      openSession(last);
      return;
    }
  }
  render();
}
async function refreshSessions() {
  try {
    const s = await rpc("snapshot");
    state.sessions = s.sessions || [];
    sidebar();
    if (view === "history") renderHistory();
    if (view === "new") renderNew();
    if (view === "session") {
      const meta = state.sessions.find((s) => s.id === selected?.id);
      if (meta) {
        selected.duration = meta.duration;
        selected.status = meta.status;
        const node = $("#session-meta");
        if (node)
          node.innerText =
            label(selected.type) +
            " · " +
            date(selected.created) +
            " · " +
            duration(selected.duration) +
            "\n" +
            selected.source;
      }
      updateRecordingBanner();
    }
  } catch (e) {
    error(e);
  }
}
function sidebar() {
  document.querySelectorAll("[data-nav]").forEach((b) => {
    b.classList.toggle("active", b.dataset.nav === view);
  });
  $("#live-dot").classList.toggle("on", !!state.live.active);
  $("#queue-count").textContent =
    state.jobs.filter((j) => !terminal.has(j.state)).length || "";
  $("#model-dot").classList.toggle("ready", !!state.model.path);
  $("#recent").innerHTML =
    state.sessions
      .slice(0, 20)
      .map(
        (s) =>
          `<button class="${selected?.id === s.id && view === "session" ? "active" : ""}" data-session="${s.id}" title="${escape(s.title)}">${escape(s.title)}<small>${date(s.created)} · ${escape(label(s.type))}</small></button>`,
      )
      .join("") ||
    '<div class="empty">Your transcripts<br>will appear here.</div>';
  $("#recent")
    .querySelectorAll("[data-session]")
    .forEach((b) => (b.onclick = () => openSession(b.dataset.session)));
}
function navigate(next) {
  sessionGeneration++;
  view = next;
  selected = null;
  sidebar();
  render();
  content.scrollTop = 0;
  content.focus({ preventScroll: true });
}
function render() {
  const titles = {
    new: "Workspace",
    live: "Live transcription",
    import: "Import media",
    queue: "Processing queue",
    history: "Session history",
    settings: "Settings",
    session: "Session",
  };
  $("#breadcrumb").textContent = titles[view];
  (
    ({
      new: renderNew,
      live: renderLive,
      import: renderImport,
      queue: renderQueue,
      history: renderHistory,
      settings: renderSettings,
      session: renderSession,
    })[view] || renderNew
  )();
}
function page(html) {
  content.innerHTML = `<div class="page">${html}</div>`;
}
function rows(sessions) {
  return sessions
    .map(
      (s) =>
        `<button class="session-row" data-open="${s.id}"><span class="row-icon" aria-hidden="true">${s.type === "live" ? "◉" : "≡"}</span><span class="row-main"><strong>${escape(s.title)}</strong><small>${escape(label(s.type))} · ${duration(s.duration)}${s.status === "interrupted" ? " · Interrupted" : ""}</small></span><span class="date">${date(s.created)}</span></button>`,
    )
    .join("");
}
function bindRows() {
  content
    .querySelectorAll("[data-open]")
    .forEach((b) => (b.onclick = () => openSession(b.dataset.open)));
}
function renderNew() {
  page(
    `<div class="welcome-heading"><div class="date">${new Date().toLocaleDateString(undefined, { weekday: "long", month: "long", day: "numeric" })}</div><h1>New transcription</h1><p>A conversation, a lecture, or a recording.<br>Keep the words, in their original language.</p></div>${state.live.active ? '<div class="live-notice"><span><span class="dot"></span>Recording is in progress</span><button class="text-button" id="resume-live">View transcript</button></div>' : ""}<div class="choices"><button class="choice" id="new-live"><span class="choice-symbol" aria-hidden="true">◉</span><div><h2>Record a conversation</h2><p>Your microphone, system audio, or both.</p></div><span aria-hidden="true">›</span></button><button class="choice" id="new-import"><span class="choice-symbol" aria-hidden="true">↥</span><div><h2>Transcribe a recording</h2><p>Import audio, video, or a media link.</p></div><span aria-hidden="true">›</span></button></div><div class="section-head"><h2>Recent sessions</h2><button class="text-button" id="view-history">View all</button></div>${rows(state.sessions.slice(0, 5)) || '<div class="empty">No sessions yet. Start a recording or import your first file.</div>'}`,
  );
  $("#new-live").onclick = () => navigate("live");
  $("#new-import").onclick = () => navigate("import");
  $("#view-history").onclick = () => navigate("history");
  if ($("#resume-live"))
    $("#resume-live").onclick = () => openSession(state.live.sessionId);
  bindRows();
}
function modelNotice() {
  return state.model.path
    ? ""
    : `<div class="waiting-model">${escape(state.model.state === "checking" ? "Checking existing Kian installations and verifying the model…" : "A verified model is needed. Choose an existing model or download it in Settings.")} <button class="text-button" id="open-model">Open settings</button></div>`;
}
function renderLive() {
  if (state.live.active) {
    openSession(state.live.sessionId);
    return;
  }
  page(
    `<div class="page-intro"><h1>Live transcription</h1><p>Stay with the conversation. Kian captures the words locally, while you listen.</p></div>${modelNotice()}<div class="record-panel"><div class="source-controls"><h3>Audio source</h3><select id="audio-source" aria-label="Audio source"><option value="0">Microphone + system audio</option><option value="1">Microphone only</option><option value="2">System audio only</option></select></div><div class="record-body"><div class="record-clock">00:00</div><p>Ready to record</p><button id="start-recording" class="primary" ${state.model.path ? "" : "disabled"}>● &nbsp; Start recording</button></div></div><p class="muted">Transcripts are saved to your Mac. Closing the dashboard keeps recording in the menu bar.</p><div class="section-head"><h2>English, Filipino, and Taglish</h2></div><p class="muted">Automatic language detection preserves the language you speak. Kian does not translate or rewrite your words.</p>`,
  );
  $("#audio-source").value = state.live.mode;
  $("#audio-source").onchange = (e) => action("mode", Number(e.target.value));
  $("#start-recording").onclick = async () => {
    const button = $("#start-recording");
    button.disabled = true;
    try {
      await rpc("start");
    } catch (e) {
      error(e);
      button.disabled = false;
    }
  };
  if ($("#open-model")) $("#open-model").onclick = () => navigate("settings");
}
// Recheck after users install tools outside the app; no restart is needed.
function renderDependencyChecks() {
  const container=$("#dependency-checks");
  if(!container)return;
  const report=state.dependencies;
  const rows=["ffmpeg","ffprobe","yt-dlp"].map(name=>{
    const tool=report.tools?.[name];
    const status=tool ? (tool.available ? "Ready" : "Missing") : "Checking…";
    const detail=view==="settings" && tool?.available ? tool.path : status;
    return `<div class="dependency"><span>${escape(name)}</span><small>${escape(detail)}</small></div>`;
  }).join("");
  const workflows=[["Local files",report.file],["Media URLs",report.url]].map(([name,flow])=>{
    if(!flow)return "";
    if(flow.ready)return `<p>${name}: ready</p>`;
    return `<p>${name}: missing ${escape(flow.missing.join(", "))}.<br>Install with <code>${escape(flow.installCommand)}</code></p>`;
  }).join("");
  container.innerHTML=`${rows}<div class="model-status">${workflows || "Checking media tools…"}<small>Live recording works without these tools.</small></div><button id="recheck-dependencies">Check again</button>`;
  $("#recheck-dependencies").onclick=()=>refreshDependencies(true);
  if($("#choose-files"))$("#choose-files").disabled=!report.file?.ready;
  if($("#fetch-url"))$("#fetch-url").disabled=!report.url?.ready || !!fetchURL.pending;
  if($("#enqueue-url"))$("#enqueue-url").disabled=!report.url?.ready;
}
async function refreshDependencies(retryFiles=false) {
  const button=$("#recheck-dependencies");if(button)button.disabled=true;
  try {
    state.dependencies=await rpc("dependencies");
    if(view==="import"||view==="settings")renderDependencyChecks();
    if(view==="import") {
      if(retryFiles&&state.dependencies.file.ready) {
        for(const file of files.filter(file=>file.error)) {
          file.pending=true;file.error="";renderFiles();
          try{file.meta=await rpc("probe",file.path)}catch(e){file.error=e.message}
          file.pending=false;
        }
      }
      renderFiles();
    }
  } catch(e) {error(e)}
  finally {if($("#recheck-dependencies"))$("#recheck-dependencies").disabled=false}
}
async function setFiles(paths) {
  view = "import";
  selected = null;
  files = paths.map((path) => ({ path, pending: true }));
  urlMeta = null;
  render();
  sidebar();
  for (const file of files) {
    try {
      file.meta = await rpc("probe", file.path);
    } catch (e) {
      file.error = e.message;
    }
    file.pending = false;
    if (view === "import") renderFiles();
  }
}
function renderImport() {
  page(
    `<div class="page-intro"><h1>Import media</h1><p>Turn recordings into editable transcripts, without playing them in real time.</p></div><div id="dependency-checks" aria-live="polite"></div><button class="dropzone" id="choose-files"><span aria-hidden="true">↥</span><strong>Drop audio or video here</strong><small>or choose files from your Mac</small></button><div class="format-hint">MP3, WAV, M4A, AAC, FLAC, OGG · MP4, MOV, MKV, WEBM, AVI</div><div id="files"></div><div id="file-actions"></div><div class="or-divider">or import a link</div><label for="media-url">Media URL</label><div class="url-row"><input id="media-url" type="url" placeholder="Paste a YouTube or direct media link" autocomplete="off"><button id="fetch-url">Fetch details</button></div><small>YouTube, Shorts, and accessible sites supported by yt-dlp. One video per link.</small><div id="url-details"></div>`,
  );
  $("#choose-files").onclick = () => action("pickFiles");
  $("#fetch-url").onclick = fetchURL;
  $("#media-url").onkeydown = (e) => {
    if (e.key === "Enter") fetchURL();
  };
  $("#media-url").oninput = () => {
    urlMeta = null;
    $("#url-details").innerHTML = "";
  };
  renderDependencyChecks();
  refreshDependencies();
  renderFiles();
  if (urlMeta) {
    $("#media-url").value = urlMeta.url;
    renderURL();
  }
}
function renderFiles() {
  if (!$("#files")) return;
  $("#files").innerHTML = files
    .map(
      (f, i) =>
        `<div class="file-item"><div><strong>${escape(f.path.split("/").pop())}</strong><small ${f.error ? 'class="job-error"' : ""}>${escape(f.pending ? "Inspecting audio…" : f.error || `${duration(f.meta.duration)} · ${f.meta.format}`)}</small></div><button class="icon-button" data-remove="${i}" aria-label="Remove file">×</button></div>`,
    )
    .join("");
  $("#files")
    .querySelectorAll("[data-remove]")
    .forEach(
      (b) =>
        (b.onclick = () => {
          files.splice(Number(b.dataset.remove), 1);
          renderFiles();
        }),
    );
  $("#file-actions").innerHTML = files.length
    ? `<div class="import-actions"><span class="muted">${files.length} file${files.length === 1 ? "" : "s"} selected</span><button id="enqueue-files" class="primary" ${files.some((f) => f.pending || f.error) || !state.dependencies.file?.ready ? "disabled" : ""}>Add to queue</button></div>`
    : "";
  if ($("#enqueue-files"))
    $("#enqueue-files").onclick = async () => {
      const b = $("#enqueue-files");
      b.disabled = true;
      try {
        await rpc("enqueue", {
          sources: files.map((f) => f.path),
          kind: "file",
        });
        files = [];
        navigate("queue");
      } catch (e) {
        error(e);
        b.disabled = false;
      }
    };
}
async function fetchURL() {
 if(!state.dependencies.url?.ready || fetchURL.pending)return;
 fetchURL.pending=true;
  const raw = $("#media-url").value.trim(),
    button = $("#fetch-url");
  button.disabled = true;
  button.textContent = "Fetching…";
  try {
    urlMeta = await rpc("metadata", raw);
    if (view === "import") renderURL();
  } catch (e) {
    error(e);
  } finally {
    fetchURL.pending=false;
    if ($("#fetch-url")) {
      $("#fetch-url").disabled = !state.dependencies.url?.ready;
      $("#fetch-url").textContent = "Fetch details";
    }
  }
}
function renderURL() {
  if (!urlMeta || !$("#url-details")) return;
  $("#url-details").innerHTML =
    `<div class="metadata"><h3>${escape(urlMeta.title)}</h3><small>${duration(urlMeta.duration)} · Audio-only download when available</small></div><div class="import-actions"><label><input id="retain-media" type="checkbox">Keep downloaded media for playback</label><button class="primary" id="enqueue-url" ${state.dependencies.url?.ready ? "" : "disabled"}>Add to queue</button></div>`;
  $("#enqueue-url").onclick = async () => {
    const b = $("#enqueue-url");
    b.disabled = true;
    try {
      await rpc("enqueue", {
        sources: [urlMeta.url],
        kind: "url",
        retain: $("#retain-media").checked,
      });
      urlMeta = null;
      navigate("queue");
    } catch (e) {
      error(e);
      b.disabled = false;
    }
  };
}
function renderQueue() {
  const states = {
    waiting: "Waiting",
    downloading: "Downloading audio",
    preparing: "Preparing audio",
    transcribing: "Transcribing",
    paused: "Paused for live recording",
    completed: "Completed",
    failed: "Failed",
    cancelled: "Cancelled",
  };
  page(
    `<div class="page-intro"><h1>Processing queue</h1><p>One recording at a time. Live transcription takes priority; imports pause at the next audio checkpoint.</p></div>${state.jobs.length ? state.jobs.map((j) => `<div class="job"><div class="job-head"><div><h3>${escape(j.title)}</h3><small>${escape(states[j.state] || j.state)} · ${duration(j.duration)}</small></div><div class="controls">${j.sessionId ? `<button data-open="${j.sessionId}">Open transcript</button>` : ""}${!terminal.has(j.state) ? `<button data-cancel="${j.id}">Cancel</button>` : j.state !== "completed" ? `<button data-retry="${j.id}">Retry</button>` : ""}</div></div>${j.state === "transcribing" || j.state === "paused" || j.state === "completed" ? `<div class="job-progress"><progress max="1" value="${j.progress}" aria-label="Transcription progress"></progress><span>${Math.floor(j.progress * 100)}%</span></div><small>${clock(j.processed)} of audio processed · ${clock(j.elapsed)} elapsed</small>` : ""}${j.error ? `<p class="job-error">${escape(j.error)}</p>` : ""}</div>`).join("") : '<div class="empty">Your queue is clear.<br><button class="text-button" id="queue-import">Import audio, video, or a link</button></div>'}`,
  );
  bindRows();
  content
    .querySelectorAll("[data-cancel]")
    .forEach((b) => (b.onclick = () => action("cancel", b.dataset.cancel)));
  content
    .querySelectorAll("[data-retry]")
    .forEach((b) => (b.onclick = () => action("retry", b.dataset.retry)));
  if ($("#queue-import")) $("#queue-import").onclick = () => navigate("import");
}
function renderHistory() {
  page(
    `<div class="page-intro"><h1>Session history</h1><p>Your conversations and recordings, kept on your Mac.</p></div><input id="history-query" type="search" class="history-search" placeholder="Search titles and transcripts" aria-label="Search session history" value="${escape(historyQuery)}"><div id="history-results"></div>`,
  );
  filterHistory();
  $("#history-query").oninput = (e) => {
    historyQuery = e.target.value;
    clearTimeout(renderHistory.timer);
    renderHistory.timer = setTimeout(filterHistory, 180);
  };
}
async function filterHistory() {
  let items = state.sessions;
  const q = historyQuery.toLowerCase().trim();
  if (q) {
    try {
      const ids = await rpc("search", q);
      if (!$("#history-results")) return;
      items = items.filter((s) => ids.includes(s.id));
    } catch (e) {
      error(e);
    }
  }
  if (!$("#history-results")) return;
  $("#history-results").innerHTML =
    rows(items) || '<div class="empty">No matching sessions.</div>';
  bindRows();
}
async function openSession(id) {
  const generation = ++sessionGeneration;
  try {
    const session = await rpc("get", id);
    if (generation !== sessionGeneration) return;
    selected = session;
    selected.segments ??= [];
    selected.corrections ??= {};
    view = "session";
    storage.setItem("selectedSession", id);
    sidebar();
    renderSession();
    $("#breadcrumb").textContent = label(session.type);
    content.scrollTop = 0;
  } catch (e) {
    error(e);
  }
}
function updateRecordingBanner() {
  const target = $("#recording-banner");
  if (!target) return;
  const active = state.live.active && selected?.id === state.live.sessionId;
  target.innerHTML = active
    ? `<div class="live-notice"><span><span class="dot"></span><strong id="live-clock">${clock((Date.now() - new Date(state.live.started)) / 1000)}</strong> &nbsp; ${escape({ loading: "Loading model", recording: "Recording", stopping: "Finishing transcript" }[state.live.state] || state.live.state)}</span><button id="stop-recording" ${state.live.state === "stopping" ? "disabled" : ""}>Stop recording</button></div>`
    : "";
  if ($("#stop-recording")) $("#stop-recording").onclick = () => action("stop");
  if (!active && state.live.error && selected?.id === state.live.sessionId)
    error(state.live.error);
}
function renderSession() {
  if (!selected) return;
  const s = selected;
  page(
    `<div id="recording-banner"></div><div class="session-heading"><h1 id="session-title">${escape(s.title)}</h1><button id="rename-session" class="icon-button" aria-label="Rename session" title="Rename session">✎</button></div><div class="session-meta" id="session-meta">${escape(label(s.type))} · ${date(s.created)} · ${duration(s.duration)}<br>${escape(s.source)}</div><div class="viewer-toolbar"><select id="display-mode" aria-label="Transcript display"><option value="clean">Clean text</option><option value="timestamps">Timestamped</option><option value="sources">Source-aware</option></select><button class="icon-button" id="font-less" aria-label="Decrease font size">A−</button><button class="icon-button" id="font-more" aria-label="Increase font size">A+</button><label><input id="auto-scroll" type="checkbox" ${autoScroll ? "checked" : ""}>Auto-scroll</label><input id="transcript-search" type="search" placeholder="Find in transcript" aria-label="Find in transcript"></div><div id="transcript" class="transcript ${mode}" style="--transcript-size:${font}px"><div class="transcript-empty" id="transcript-empty">${s.status === "processing" || (state.live.active && s.id === state.live.sessionId) ? "Finalized speech will appear here as it is transcribed." : "This session has no finalized segments."}</div></div><div class="viewer-footer"><button id="copy-transcript">Copy transcript</button><select id="export-format" aria-label="Export format"><option value="txt">TXT</option><option value="md">Markdown</option><option value="srt">SRT</option><option value="vtt">VTT</option></select><button id="export-transcript">Export…</button>${s.mediaPath ? '<button id="review-media">Review media</button>' : ""}<button id="open-source">Open source</button><button id="delete-session" class="danger">Delete</button></div>`,
  );
  s.segments.forEach(insertSegment);
  updateRecordingBanner();
  $("#display-mode").value = mode;
  $("#display-mode").onchange = (e) => {
    mode = e.target.value;
    storage.setItem("displayMode", mode);
    $("#transcript").className = "transcript " + mode;
  };
  $("#font-less").onclick = () => setFont(-1);
  $("#font-more").onclick = () => setFont(1);
  $("#auto-scroll").onchange = (e) => (autoScroll = e.target.checked);
  $("#transcript-search").oninput = filterSegments;
  $("#copy-transcript").onclick = () =>
    action("copy", { id: s.id, format: "txt", mode });
  $("#export-transcript").onclick = () =>
    action("export", { id: s.id, format: $("#export-format").value, mode });
  if ($("#review-media"))
    $("#review-media").onclick = () => action("play", { id: s.id, seconds: 0 });
  $("#open-source").onclick = () => action("reveal", s.id);
  $("#delete-session").onclick = async () => {
    if (
      !confirm(
        "Delete this transcript and its saved corrections? Original media, legacy text files, and model files are preserved.",
      )
    )
      return;
    try {
      await rpc("delete", s.id);
      storage.removeItem("selectedSession");
      await refreshSessions();
      navigate("new");
    } catch (e) {
      error(e);
    }
  };
  $("#rename-session").onclick = renameSession;
}
function setFont(delta) {
  font = Math.max(12, Math.min(28, font + delta));
  $("#transcript").style.setProperty("--transcript-size", font + "px");
  storage.setItem("fontSize", font);
}
function segmentNode(seg) {
  const node = document.createElement("article");
  node.className = "segment";
  node.dataset.id = seg.id;
  node.dataset.start = seg.start;
  const corrected = Object.hasOwn(selected.corrections, seg.id);
  node.classList.toggle("corrected", corrected);
  node.innerHTML = `<div class="segment-meta"><span class="source"></span>${seg.untimed ? "" : `<button class="timestamp" title="Seek media to this timestamp">${clock(seg.start)}</button>`}</div><p class="segment-text"></p><div class="segment-tools"><button class="edit-segment">Edit</button>${corrected ? '<button class="restore-segment">Restore original</button>' : ""}<span class="edited">Edited</span></div>`;
  node.querySelector(".source").textContent = seg.source;
  node.querySelector(".segment-text").textContent = corrected
    ? selected.corrections[seg.id]
    : seg.text;
  const seek = node.querySelector(".timestamp");
  if (seek)
    seek.onclick = () =>
      action("play", { id: selected.id, seconds: seg.start });
  node.querySelector(".edit-segment").onclick = () => editSegment(seg, node);
  const restore = node.querySelector(".restore-segment");
  if (restore)
    restore.onclick = () =>
      action("correct", { id: selected.id, segment: seg.id, text: null });
  return node;
}
function insertSegment(seg) {
  const transcript = $("#transcript");
  if (!transcript || transcript.querySelector(`[data-id="${seg.id}"]`)) return;
  $("#transcript-empty")?.remove();
  const node = segmentNode(seg);
  const next = [...transcript.children].find(
    (n) => Number(n.dataset.start) > seg.start,
  );
  transcript.insertBefore(node, next || null);
  filterSegments();
}
function replaceSegment(id) {
  const old = $(`[data-id="${id}"]`),
    seg = selected.segments.find((s) => s.id === id);
  if (old && seg) old.replaceWith(segmentNode(seg));
  filterSegments();
}
function editSegment(seg, node) {
  if (node.querySelector("textarea")) return;
  const text = Object.hasOwn(selected.corrections, seg.id)
    ? selected.corrections[seg.id]
    : seg.text;
  node.innerHTML =
    '<textarea aria-label="Edit transcript segment"></textarea><div class="edit-actions"><button class="primary save-edit">Save correction</button><button class="cancel-edit">Cancel</button></div>';
  const input = node.querySelector("textarea");
  input.value = text;
  input.focus();
  const id = selected.id;
  node.querySelector(".save-edit").onclick = async () => {
    const button = node.querySelector(".save-edit");
    button.disabled = true;
    try {
      await rpc("correct", { id, segment: seg.id, text: input.value });
      if (selected?.id === id) {
        selected.corrections[seg.id] = input.value;
        replaceSegment(seg.id);
      }
    } catch (e) {
      error(e);
      button.disabled = false;
    }
  };
  node.querySelector(".cancel-edit").onclick = () => replaceSegment(seg.id);
  input.onkeydown = (e) => {
    if (e.key === "Escape") replaceSegment(seg.id);
    if (e.metaKey && e.key === "Enter")
      node.querySelector(".save-edit").click();
  };
}
function filterSegments() {
  const q = $("#transcript-search")?.value.toLowerCase() || "";
  document.querySelectorAll(".segment").forEach((n) => {
    if (!n.querySelector("textarea"))
      n.hidden =
        !!q &&
        !n
          .querySelector(".segment-text")
          ?.textContent.toLowerCase()
          .includes(q);
  });
}
function renameSession() {
  const title = $("#session-title");
  const original = selected.title;
  title.innerHTML =
    '<span class="inline-title"><input aria-label="Session title"><button>Save</button></span>';
  const input = title.querySelector("input");
  input.value = original;
  input.focus();
  const save = async () => {
    try {
      await rpc("rename", { id: selected.id, title: input.value });
      selected.title = input.value;
      title.textContent = input.value;
      refreshSessions();
    } catch (e) {
      error(e);
    }
  };
  title.querySelector("button").onclick = save;
  input.onkeydown = (e) => {
    if (e.key === "Enter") save();
    if (e.key === "Escape") title.textContent = original;
  };
}
function renderSettings() {
  const m = state.model,
    s = state.settings;
  page(
    `<div class="page-intro"><h1>Settings</h1><p>Local by default. One model for every transcription.</p></div><div class="settings-group"><h2>Whisper Large v3 Turbo</h2><div class="model-status"><strong>${escape({ reused: "✓ Existing model reused", downloaded: "✓ Model downloaded", checking: "Checking existing models…", missing: "Model required" }[m.state] || m.state)}</strong><code>${escape(m.path || m.detail || "Choose an existing model below.")}</code></div><p>Existing Kian installations and the configured model directory are checked before any download. Every model must pass format and SHA-256 verification.</p><label class="field"><span>Existing model path</span><input id="model-path" value="${escape(s.modelPath || m.path || "")}" placeholder="/path/to/ggml-large-v3-turbo.bin"></label><label class="field"><span>Additional model directory</span><input id="model-dir" value="${escape(s.modelDir || "")}" placeholder="Optional directory to search"></label><div class="controls"><button id="choose-model">Choose model…</button><button id="verify-model">Check existing models</button>${m.path ? "" : '<button id="download-model">Download model (1.6 GB)</button>'}</div></div><div class="settings-group"><h2>Recognition</h2><label class="field"><span>Language</span><select id="language"><option value="auto">Automatic (recommended for Taglish)</option><option value="en">English</option><option value="tl">Filipino / Tagalog</option></select></label><label class="field"><span>Optional vocabulary or context</span><textarea id="context-prompt" placeholder="Names, technical terms, or a short context hint"></textarea></label><p>Context may affect recognition. Automatic detection is the default for mixed English and Filipino. Translation is always off.</p><button id="save-settings" class="primary">Save settings</button></div><div class="settings-group"><h2>Media tools</h2><p>FFmpeg decodes local files. yt-dlp imports supported URLs. Tools are used only when importing; no Python environment setup is needed.</p><div id="dependency-checks" aria-live="polite"></div></div><div class="settings-group"><h2>Storage</h2><p>${escape(state.output || "~/Documents/Kian/")}<br>Transcripts are independent of the original media. Temporary import audio is removed after completion, cancellation, or errors.</p><button id="open-storage">Open transcript folder</button></div>`,
  );
  renderDependencyChecks();
  $("#language").value = s.language || "auto";
  $("#context-prompt").value = s.prompt || "";
  $("#choose-model").onclick = () => action("pickModel");
  $("#verify-model").onclick = () => action("verifyModel");
  if ($("#download-model"))
    $("#download-model").onclick = async () => {
      const b = $("#download-model");
      b.disabled = true;
      try {
        await rpc("downloadModel");
        toast("Model ready");
        state.settings = (await rpc("snapshot")).settings;
        renderSettings();
      } catch (e) {
        error(e);
        b.disabled = false;
      }
    };
  $("#save-settings").onclick = async () => {
    const b = $("#save-settings");
    b.disabled = true;
    try {
      const settings = {
        modelPath: $("#model-path").value.trim(),
        modelDir: $("#model-dir").value.trim(),
        language: $("#language").value,
        prompt: $("#context-prompt").value,
      };
      await rpc("settings", settings);
      state.settings = settings;
      renderSettings();
      toast("Settings saved");
    } catch (e) {
      error(e);
    } finally {
      if ($("#save-settings")) $("#save-settings").disabled = false;
    }
  };
  $("#open-storage").onclick = () => action("reveal", "");
}
$("#error button").onclick = () => ($("#error").hidden = true);
document
  .querySelectorAll("[data-nav]")
  .forEach((b) => (b.onclick = () => navigate(b.dataset.nav)));
$("#all-history").onclick = () => navigate("history");
$("#pin").onclick = async () => {
  pinned = !pinned;
  await action("top", pinned);
  $("#pin").setAttribute("aria-pressed", String(pinned));
  $("#pin").style.color = pinned ? "var(--accent)" : "";
};
document.addEventListener("keydown", (e) => {
  if (e.metaKey && e.key.toLowerCase() === "n") {
    e.preventDefault();
    navigate("new");
  }
  if (e.metaKey && e.key.toLowerCase() === "f") {
    e.preventDefault();
    if (view === "session") $("#transcript-search").focus();
    else {
      navigate("history");
      $("#history-query").focus();
    }
  }
  if (e.key === "Escape") $("#error").hidden = true;
});
setInterval(() => {
  if ($("#live-clock") && state.live.active)
    $("#live-clock").textContent = clock(
      (Date.now() - new Date(state.live.started)) / 1000,
    );
}, 1000);
render();
if (window.webkit?.messageHandlers?.kian)
  rpc("snapshot").then(applySnapshot).catch(error);
