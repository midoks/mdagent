"use strict";

const $ = (sel, el = document) => el.querySelector(sel);

async function api(path, opts) {
  const resp = await fetch(path, opts);
  if (!resp.ok) {
    let msg = "HTTP " + resp.status;
    try {
      const data = await resp.json();
      if (data.error) msg = data.error;
    } catch (_) {}
    throw new Error(msg);
  }
  return resp.json();
}

function esc(s) {
  return String(s == null ? "" : s)
    .replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}

function fmtTime(s) {
  if (!s) return "-";
  const d = new Date(s);
  const p = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

function fmtAgo(s) {
  if (!s) return "-";
  const sec = Math.max(0, (Date.now() - new Date(s).getTime()) / 1000);
  if (sec < 60) return Math.floor(sec) + " 秒前";
  if (sec < 3600) return Math.floor(sec / 60) + " 分钟前";
  if (sec < 86400) return Math.floor(sec / 3600) + " 小时前";
  return Math.floor(sec / 86400) + " 天前";
}

function fmtDuration(task) {
  if (!task.startedAt) return "-";
  const end = task.finishedAt ? new Date(task.finishedAt) : new Date();
  const ms = end - new Date(task.startedAt);
  if (ms < 1000) return ms + "ms";
  return (ms / 1000).toFixed(1) + "s";
}

const STATUS_TEXT = {
  pending: "等待中", running: "执行中", succeeded: "成功", failed: "失败",
  online: "在线", offline: "离线",
};

function badge(status) {
  return `<span class="badge ${esc(status)}">${esc(STATUS_TEXT[status] || status)}</span>`;
}

function shortID(id) {
  return id && id.length > 13 ? id.slice(0, 13) : id;
}

function touchUpdated() {
  const el = $("#updated-at");
  if (el) el.textContent = "更新于 " + fmtTime(new Date().toISOString());
}

// 页面不可见时暂停轮询，可见时立即刷新。
function poll(fn, interval) {
  fn();
  setInterval(() => {
    if (!document.hidden) fn();
  }, interval || 3000);
}

/* ================= 概览页 ================= */

function initOverview() {
  async function refresh() {
    const [stats, tasks] = await Promise.all([api("/api/stats"), api("/api/tasks")]);
    $("#stat-agents").textContent = `${stats.agentsOnline} / ${stats.agentsTotal}`;
    $("#stat-tasks").textContent = stats.tasksTotal;
    $("#stat-rate").textContent = stats.tasksTotal ? stats.successRate.toFixed(1) + "%" : "-";
    $("#stat-today").textContent = stats.tasksToday;

    renderDist(stats);
    renderRecent(tasks.slice(0, 8));
    touchUpdated();
  }
  poll(refresh);
}

function renderDist(stats) {
  const items = [
    ["pending", "等待中", stats.tasksPending],
    ["running", "执行中", stats.tasksRunning],
    ["succeeded", "成功", stats.tasksSucceeded],
    ["failed", "失败", stats.tasksFailed],
  ];
  const total = Math.max(1, stats.tasksTotal);
  $("#dist").innerHTML = items
    .filter(([, , n]) => n > 0)
    .map(([cls, , n]) => `<div class="seg ${cls}" style="width:${(n / total) * 100}%"></div>`)
    .join("");
  $("#dist-legend").innerHTML = items
    .map(([cls, label, n]) => `<span><span class="dot" style="background:var(--${dotColor(cls)})"></span>${label} ${n}</span>`)
    .join("");
}

function dotColor(cls) {
  return { pending: "yellow", running: "primary", succeeded: "green", failed: "red" }[cls] || "gray";
}

function renderRecent(tasks) {
  const tbody = $("#recent-tasks");
  if (!tasks.length) {
    tbody.innerHTML = `<tr><td colspan="5" class="empty">暂无任务，去「任务」页创建一个</td></tr>`;
    return;
  }
  tbody.innerHTML = tasks.map((t) => `
    <tr>
      <td class="mono">${esc(shortID(t.id))}</td>
      <td class="prompt-cell" title="${esc(t.prompt)}">${esc(t.prompt)}</td>
      <td class="mono">${esc(t.agentId ? shortID(t.agentId) : "-")}</td>
      <td>${badge(t.status)}</td>
      <td>${fmtTime(t.createdAt)}</td>
    </tr>`).join("");
}

/* ================= Agent 页 ================= */

function initAgents() {
  async function refresh() {
    const agents = await api("/api/agents");
    const tbody = $("#agent-list");
    if (!agents.length) {
      tbody.innerHTML = `<tr><td colspan="7" class="empty">暂无 Agent，启动一个 agent 进程即可注册</td></tr>`;
    } else {
      tbody.innerHTML = agents.map((a) => `
        <tr>
          <td>${esc(a.name)}</td>
          <td class="mono">${esc(a.id)}</td>
          <td>${badge(a.status)}</td>
          <td>${esc((a.meta && a.meta.provider) || "-")}</td>
          <td>${(a.tools || []).map((t) => `<span class="badge running">${esc(t)}</span>`).join(" ")}</td>
          <td>${a.tasksDone}</td>
          <td title="${fmtTime(a.lastSeen)}">${fmtAgo(a.lastSeen)}</td>
        </tr>`).join("");
    }
    touchUpdated();
  }
  poll(refresh);
}

/* ================= 任务页 ================= */

function initTasks() {
  const expanded = new Set();

  async function refreshAgents() {
    const agents = await api("/api/agents");
    const sel = $("#task-agent");
    const cur = sel.value;
    sel.innerHTML = `<option value="">任意在线 Agent</option>` + agents
      .filter((a) => a.status === "online")
      .map((a) => `<option value="${esc(a.id)}">${esc(a.name)} (${esc(a.id)})</option>`)
      .join("");
    sel.value = cur;
  }

  async function refreshTasks() {
    const tasks = await api("/api/tasks");
    const tbody = $("#task-list");
    if (!tasks.length) {
      tbody.innerHTML = `<tr><td colspan="6" class="empty">暂无任务</td></tr>`;
      return;
    }
    tbody.innerHTML = tasks.map((t) => {
      const open = expanded.has(t.id);
      let html = `
      <tr class="clickable" data-id="${esc(t.id)}">
        <td class="mono">${esc(shortID(t.id))}</td>
        <td class="prompt-cell" title="${esc(t.prompt)}">${esc(t.prompt)}</td>
        <td class="mono">${esc(t.agentId ? shortID(t.agentId) : "-")}</td>
        <td>${badge(t.status)}</td>
        <td>${fmtDuration(t)}</td>
        <td>${fmtTime(t.createdAt)}</td>
      </tr>`;
      if (open) html += renderDetail(t);
      return html;
    }).join("");

    tbody.querySelectorAll("tr.clickable").forEach((tr) => {
      tr.addEventListener("click", () => {
        const id = tr.dataset.id;
        if (expanded.has(id)) expanded.delete(id);
        else expanded.add(id);
        refreshTasks();
      });
    });
    touchUpdated();
  }

  function renderDetail(t) {
    const blocks = [];
    blocks.push(`<div class="detail-block"><div class="label">Prompt</div><pre>${esc(t.prompt)}</pre></div>`);
    if (t.result) blocks.push(`<div class="detail-block"><div class="label">结果</div><pre>${esc(t.result)}</pre></div>`);
    if (t.error) blocks.push(`<div class="detail-block"><div class="label">错误</div><pre>${esc(t.error)}</pre></div>`);
    if (t.steps && t.steps.length) {
      blocks.push(`<div class="detail-block"><div class="label">执行步骤</div><ul class="detail-steps">${
        t.steps.map((s) => `<li>${esc(s)}</li>`).join("")}</ul></div>`);
    }
    return `<tr class="detail-row"><td colspan="6">${blocks.join("")}</td></tr>`;
  }

  $("#task-form").addEventListener("submit", async (e) => {
    e.preventDefault();
    const btn = $("#task-submit");
    const errEl = $("#form-error");
    errEl.textContent = "";
    btn.disabled = true;
    try {
      await api("/api/tasks", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ prompt: $("#task-prompt").value, agentId: $("#task-agent").value }),
      });
      $("#task-prompt").value = "";
      await refreshTasks();
    } catch (err) {
      errEl.textContent = err.message;
    } finally {
      btn.disabled = false;
    }
  });

  poll(refreshTasks);
  poll(refreshAgents, 5000);
}

/* ================= 日志页 ================= */

function initLogs() {
  $("#log-level").addEventListener("change", refresh);
  async function refresh() {
    const level = $("#log-level").value;
    const logs = await api("/api/logs?limit=300&level=" + level);
    const tbody = $("#log-list");
    if (!logs.length) {
      tbody.innerHTML = `<tr><td colspan="4" class="empty">暂无日志</td></tr>`;
      return;
    }
    tbody.innerHTML = logs.map((l) => `
      <tr>
        <td class="mono">${fmtTime(l.time)}</td>
        <td>${badge(l.level)}</td>
        <td class="mono">${esc(l.source)}</td>
        <td>${esc(l.message)}</td>
      </tr>`).join("");
    touchUpdated();
  }
  poll(refresh);
}

/* ================= 入口 ================= */

const inits = { overview: initOverview, agents: initAgents, tasks: initTasks, logs: initLogs };
const page = document.body.dataset.page;
if (inits[page]) inits[page]();
