const { test } = require("node:test");
const assert = require("node:assert/strict");
const { readFileSync } = require("node:fs");
const { join } = require("node:path");
const vm = require("node:vm");

// Exercise the production event handlers without a browser-specific File.path.
// Native pasteboard resolution is mocked at the bridge boundary.
function harness() {
  const nodes = new Map(), listeners = new Map(), requests = [];
  function element(selector) {
    if (!nodes.has(selector)) nodes.set(selector, {
      innerHTML: "", textContent: "", hidden: false, value: "",
      style: {}, classList: { add() {}, remove() {}, toggle() {} },
      setAttribute() {}, focus() {}, querySelectorAll() { return []; },
    });
    return nodes.get(selector);
  }
  const ready = { ready: true, missing: [] };
  const snapshot = { sessions: [], jobs: [], live: { active: false, mode: 0 },
    model: { state: "reused", path: "/model.bin" }, settings: { language: "auto" },
    dependencies: { tools: {}, file: ready, url: ready } };
  const window = { addEventListener() {}, getSelection() { return ""; } };
  const document = { querySelector: element, querySelectorAll() { return []; },
    body: element("body"), addEventListener(name, callback) { listeners.set(name, callback); } };
  window.webkit = { messageHandlers: { tala: { postMessage(request) {
    requests.push(request);
    const result = request.action === "snapshot" ? snapshot
      : request.action === "dependencies" ? snapshot.dependencies
      : request.action === "probe" ? { duration: 11, format: "mp3" } : null;
    queueMicrotask(() => window.receive({ type: "reply", data: { id: request.id, result } }));
  } } } };
  const context = vm.createContext({ window, document, setInterval() {}, setTimeout() {}, clearTimeout() {} });
  vm.runInContext(readFileSync(join(__dirname, "../assets/app.js"), "utf8"), context);
  return { context, window, listeners, requests, element };
}
const flush = () => new Promise(setImmediate);

test("file drags are accepted and resolve paths through the native bridge", async () => {
  const h = harness();
  await flush();
  const event = { dataTransfer: { types: ["Files"], files: [{ name: "lecture.mp3" }] },
    prevented: 0, preventDefault() { this.prevented++; } };
  h.listeners.get("dragover")(event);
  assert.equal(event.dataTransfer.dropEffect, "copy");
  h.listeners.get("drop")(event);
  await flush();
  assert.equal(event.prevented, 2);
  assert.equal(h.requests.at(-1).action, "droppedFiles");
  assert.equal(vm.runInContext("pending.size", h.context), 0);
  // The native reply carries absolute paths; probing and review use those paths.
  h.window.receive({ type: "files", data: ["/recordings/lecture.mp3"] });
  await flush();
  assert.ok(h.requests.some(r => r.action === "probe" && r.data === "/recordings/lecture.mp3"));
  assert.match(h.element("#content").innerHTML, /Import media/);
  assert.match(h.element("#files").innerHTML, /lecture.mp3/);
  assert.match(h.element("#files").innerHTML, /00:11/);
});

test("repeated drops deduplicate paths and preserve earlier selected files", async () => {
  const h = harness();
  await flush();
  h.window.receive({ type: "files", data: ["/first.wav", "/first.wav"] });
  await flush();
  h.window.receive({ type: "files", data: ["/first.wav", "/second.wav"] });
  await flush();
  assert.equal(vm.runInContext("files.length", h.context), 2);
  assert.equal(h.requests.filter(r => r.action === "probe").length, 2);
  h.window.receive({ type: "files", data: [] });
  assert.equal(vm.runInContext("files.length", h.context), 2);
});

test("text drags do not read a stale file pasteboard", async () => {
  const h = harness();
  await flush();
  const count = h.requests.length;
  h.listeners.get("drop")({ dataTransfer: { types: ["text/plain"] },
    preventDefault() { assert.fail("ordinary text drag was intercepted"); } });
  assert.equal(h.requests.length, count);
});
