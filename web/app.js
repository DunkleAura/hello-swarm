const instances = new Map();
const cards = document.querySelector("#instances");
const empty = document.querySelector("#empty");
const status = document.querySelector("#status");
const statusDot = document.querySelector("#status-dot");
const intervalInput = document.querySelector("#interval");
const toggle = document.querySelector("#toggle");

let timer;
let running = true;
let successfulPolls = 0;

const stateLabels = {
  online: "Online",
  missing: "Vermisst",
  offline: "Offline"
};

const stateOrder = { online: 0, missing: 1, offline: 2 };

function text(value, fallback = "-") {
  return value || fallback;
}

function row(label, value, className = "") {
  const dt = document.createElement("dt");
  dt.textContent = label;
  const dd = document.createElement("dd");
  dd.textContent = text(value);
  if (className) dd.className = className;
  return [dt, dd];
}

function updateInstanceStates() {
  const activeCount = Math.max(
    1,
    [...instances.values()].filter((instance) => instance.state !== "offline").length
  );
  const missingAfter = Math.max(3, activeCount * 2);
  const offlineAfter = Math.max(6, activeCount * 5);

  for (const instance of instances.values()) {
    const missedPolls = successfulPolls - instance.lastSeenPoll;
    if (missedPolls >= offlineAfter) {
      instance.state = "offline";
    } else if (missedPolls >= missingAfter) {
      instance.state = "missing";
    } else {
      instance.state = "online";
    }
  }
}

function lastSeenText(timestamp) {
  const seconds = Math.max(0, Math.floor((Date.now() - timestamp) / 1000));
  if (seconds < 5) return "gerade gesehen";
  if (seconds < 60) return `vor ${seconds} Sek.`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `vor ${minutes} Min.`;
  const hours = Math.floor(minutes / 60);
  return `vor ${hours} Std.`;
}

function render() {
  cards.replaceChildren();
  const sorted = [...instances.values()].sort((a, b) => {
    const byState = stateOrder[a.state] - stateOrder[b.state];
    return byState || a.instance_id.localeCompare(b.instance_id);
  });

  for (const instance of sorted) {
    const card = document.createElement("article");
    card.className = `card ${instance.state}`;

    const head = document.createElement("div");
    head.className = "card-head";
    const title = document.createElement("h2");
    title.textContent = instance.hostname;
    const live = document.createElement("span");
    live.className = `live ${instance.state}`;
    live.textContent = `${stateLabels[instance.state]} · ${lastSeenText(instance.lastSeen)}`;
    head.append(title, live);

    const meta = document.createElement("dl");
    meta.className = "meta";
    const fields = [
      ["IP", instance.ip_addresses.join(", "), "ip"],
      ["Instanz", instance.instance_id],
      ["Node", instance.node_name],
      ["Service", instance.service_name],
      ["Task", instance.task_name],
      ["Slot", instance.task_slot],
      ["Runtime", `${instance.os}/${instance.architecture} · ${instance.go_version}`],
      ["Version", instance.version],
      ["Gestartet", new Date(instance.started_at).toLocaleString()]
    ];
    for (const field of fields) meta.append(...row(...field));

    card.append(head, meta);
    cards.append(card);
  }

  const counts = { online: 0, missing: 0, offline: 0 };
  for (const instance of instances.values()) counts[instance.state] += 1;
  document.querySelector("#online-count").textContent = String(counts.online);
  document.querySelector("#missing-count").textContent = String(counts.missing);
  document.querySelector("#offline-count").textContent = String(counts.offline);
  empty.hidden = instances.size > 0;
}

function schedule() {
  clearTimeout(timer);
  if (!running) return;
  const seconds = Math.min(3600, Math.max(1, Number(intervalInput.value) || 5));
  intervalInput.value = String(seconds);
  timer = setTimeout(poll, seconds * 1000);
}

async function poll() {
  if (!running) return;
  try {
    const response = await fetch("/api/info", { cache: "no-store" });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const instance = await response.json();
    successfulPolls += 1;
    instance.lastSeen = Date.now();
    instance.lastSeenPoll = successfulPolls;
    instance.state = "online";
    instances.set(instance.instance_id, instance);
    updateInstanceStates();
    status.textContent = "Polling aktiv";
    statusDot.className = "status-dot online";
    render();
  } catch (error) {
    status.textContent = `Nicht erreichbar: ${error.message}`;
    statusDot.className = "status-dot error";
  } finally {
    schedule();
  }
}

intervalInput.addEventListener("change", () => {
  schedule();
});

toggle.addEventListener("click", () => {
  running = !running;
  toggle.textContent = running ? "Pausieren" : "Fortsetzen";
  status.textContent = running ? "Polling aktiv" : "Polling pausiert";
  statusDot.className = running ? "status-dot online" : "status-dot";
  if (running) poll(); else clearTimeout(timer);
});

setInterval(render, 1000);
poll();
