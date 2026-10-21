(function () {
  "use strict";

  const CONFIG_KEYS = [
    "http_bind",
    "data_dir",
    "ring_timeout_ms",
    "line_recover_wait_ms",
    "feature_code",
    "idle_dictation_code",
    "dictation_hotkey",
    "line_backend",
    "synth_backend",
    "focus_backend",
    "virtual_control_bind",
    "sip_bind",
    "sip_user",
    "sip_password",
    "sip_realm",
    "sip_extension",
    "sip_public_host",
    "phone_mic_enabled",
    "phone_mic_device",
    "tray_enabled",
  ];

  const BOOL_KEYS = new Set(["phone_mic_enabled", "tray_enabled"]);
  const NUMBER_KEYS = new Set(["ring_timeout_ms", "line_recover_wait_ms"]);

  const els = {
    navBtns: document.querySelectorAll(".nav-btn"),
    views: document.querySelectorAll(".view"),
    lineChip: document.getElementById("line-chip"),
    prompt: document.getElementById("prompt"),
    callStatus: document.getElementById("call-status"),
    dictationBanner: document.getElementById("dictation-banner"),
    dictationWarning: document.getElementById("dictation-warning"),
    metaQueue: document.getElementById("meta-queue"),
    metaFocus: document.getElementById("meta-focus"),
    metaDigits: document.getElementById("meta-digits"),
    metaDelivery: document.getElementById("meta-delivery"),
    metaError: document.getElementById("meta-error"),
    metaVm: document.getElementById("meta-vm"),
    recent: document.getElementById("recent"),
    vmList: document.getElementById("vm-list"),
    vmClear: document.getElementById("vm-clear"),
    configForm: document.getElementById("config-form"),
    configMsg: document.getElementById("config-msg"),
    reconnect: document.getElementById("reconnect"),
  };

  let pollTimer = null;
  let sse = null;
  let lastVmCount = -1;

  function showView(name) {
    els.navBtns.forEach(function (btn) {
      btn.classList.toggle("active", btn.dataset.view === name);
    });
    els.views.forEach(function (view) {
      const on = view.dataset.view === name;
      view.classList.toggle("active", on);
      view.hidden = !on;
    });
    if (name === "voicemail") {
      loadVoicemail();
    }
    if (name === "config") {
      loadConfig();
    }
  }

  els.navBtns.forEach(function (btn) {
    btn.addEventListener("click", function () {
      showView(btn.dataset.view);
    });
  });

  async function fetchJSON(path, opts) {
    const res = await fetch(path, opts);
    if (!res.ok) {
      let msg = res.statusText || String(res.status);
      try {
        const body = await res.json();
        if (body && body.error) {
          msg = body.error;
        }
      } catch (_) {
        /* ignore */
      }
      throw new Error(msg);
    }
    if (res.status === 204) {
      return null;
    }
    const ct = res.headers.get("content-type") || "";
    if (ct.indexOf("application/json") === -1) {
      return null;
    }
    return res.json();
  }

  function formatFocus(ft) {
    if (!ft) {
      return "—";
    }
    const parts = [];
    if (ft.app) {
      parts.push(ft.app);
    }
    if (ft.title_contains) {
      parts.push('title~"' + ft.title_contains + '"');
    }
    if (ft.chat_id) {
      parts.push("chat:" + ft.chat_id);
    }
    return parts.length ? parts.join(" · ") : "—";
  }

  function formatTime(iso) {
    if (!iso) {
      return "";
    }
    try {
      return new Date(iso).toLocaleString();
    } catch (_) {
      return String(iso);
    }
  }

  function renderStatus(st) {
    if (!st) {
      return;
    }

    const line = st.line || "down";
    els.lineChip.textContent = "line " + line;
    els.lineChip.classList.toggle("line-up", line === "up");
    els.lineChip.classList.toggle("line-down", line !== "up");

    const call = st.active_call;
    if (call) {
      els.prompt.textContent = call.prompt || "(empty prompt)";
      els.callStatus.textContent = call.status || "active";
      els.metaFocus.textContent = formatFocus(call.focus_target);
      els.metaDigits.textContent = call.digits || "—";
      els.metaDelivery.textContent = call.delivery || "—";
      els.metaError.textContent = call.error || "—";
    } else {
      els.prompt.textContent = "No active call";
      els.callStatus.textContent = st.idle_dictating ? "idle (dictating)" : "idle";
      els.metaFocus.textContent = "—";
      els.metaDigits.textContent = "—";
      els.metaDelivery.textContent = "—";
      els.metaError.textContent = "—";
    }

    els.metaQueue.textContent = String(st.queue_depth != null ? st.queue_depth : 0);
    els.metaVm.textContent = String(st.voicemail_count != null ? st.voicemail_count : 0);

    els.dictationBanner.classList.toggle("hidden", !st.idle_dictating);
    els.dictationWarning.classList.toggle("hidden", !st.dictation_warning);

    const recent = Array.isArray(st.recent) ? st.recent : [];
    els.recent.innerHTML = "";
    if (!recent.length) {
      const empty = document.createElement("li");
      empty.className = "empty";
      empty.textContent = "No recent calls";
      els.recent.appendChild(empty);
    } else {
      recent.forEach(function (c) {
        const li = document.createElement("li");
        const top = document.createElement("div");
        top.className = "item-top";
        const snip = document.createElement("div");
        snip.className = "snippet";
        snip.textContent = c.prompt || "";
        const status = document.createElement("span");
        status.className = "status-chip";
        status.textContent = c.status || "";
        top.appendChild(snip);
        top.appendChild(status);
        const sub = document.createElement("div");
        sub.className = "sub";
        sub.textContent = formatTime(c.created_at) + (c.digits ? " · digits " + c.digits : "");
        li.appendChild(top);
        li.appendChild(sub);
        els.recent.appendChild(li);
      });
    }

    if (st.voicemail_count !== lastVmCount) {
      lastVmCount = st.voicemail_count;
      const vmView = document.getElementById("view-voicemail");
      if (vmView && !vmView.hidden) {
        loadVoicemail();
      }
    }
  }

  async function pollStatus() {
    try {
      const st = await fetchJSON("/v1/status");
      renderStatus(st);
    } catch (_) {
      /* keep last UI while reconnecting / restarting */
    }
  }

  function startPolling() {
    if (pollTimer) {
      return;
    }
    pollTimer = setInterval(pollStatus, 1000);
  }

  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  }

  function connectSSE() {
    if (sse) {
      return;
    }
    try {
      sse = new EventSource("/v1/events");
    } catch (_) {
      sse = null;
      return;
    }
    sse.onopen = function () {
      /* Prefer live events; keep a slower safety poll. */
      stopPolling();
      pollTimer = setInterval(pollStatus, 5000);
    };
    sse.onerror = function () {
      /* Browser EventSource reconnects; fall back to 1s poll while down. */
      stopPolling();
      startPolling();
    };
    function onAny() {
      pollStatus();
    }
    [
      "call.updated",
      "voicemail.changed",
      "dictation.started",
      "dictation.stopped",
      "restart.soon",
      "message",
    ].forEach(function (typ) {
      sse.addEventListener(typ, onAny);
    });
  }

  async function loadVoicemail() {
    try {
      const data = await fetchJSON("/v1/voicemail");
      const entries = (data && data.entries) || [];
      els.vmList.innerHTML = "";
      if (!entries.length) {
        const empty = document.createElement("li");
        empty.className = "empty";
        empty.textContent = "No unheard voicemail";
        els.vmList.appendChild(empty);
        return;
      }
      entries.forEach(function (e) {
        const li = document.createElement("li");
        const top = document.createElement("div");
        top.className = "item-top";
        const left = document.createElement("div");
        const snip = document.createElement("div");
        snip.className = "snippet";
        snip.textContent = e.prompt || "";
        const sub = document.createElement("div");
        sub.className = "sub";
        sub.textContent =
          formatTime(e.created_at) +
          " · " +
          formatFocus(e.focus_target) +
          " · " +
          (e.call_id || "");
        left.appendChild(snip);
        left.appendChild(sub);
        const details = document.createElement("details");
        const summary = document.createElement("summary");
        summary.textContent = "Full prompt";
        const full = document.createElement("pre");
        full.className = "snippet";
        full.textContent = e.prompt || "";
        details.appendChild(summary);
        details.appendChild(full);
        left.appendChild(details);
        const del = document.createElement("button");
        del.type = "button";
        del.className = "btn danger small";
        del.textContent = "Delete";
        del.addEventListener("click", function () {
          deleteVoicemail(e.call_id);
        });
        top.appendChild(left);
        top.appendChild(del);
        li.appendChild(top);
        els.vmList.appendChild(li);
      });
    } catch (err) {
      els.vmList.innerHTML = "";
      const li = document.createElement("li");
      li.className = "empty";
      li.textContent = "Failed to load: " + err.message;
      els.vmList.appendChild(li);
    }
  }

  async function deleteVoicemail(id) {
    if (!id) {
      return;
    }
    try {
      await fetchJSON("/v1/voicemail/" + encodeURIComponent(id), { method: "DELETE" });
      await loadVoicemail();
      await pollStatus();
    } catch (err) {
      alert("Delete failed: " + err.message);
    }
  }

  els.vmClear.addEventListener("click", async function () {
    if (!confirm("Clear all voicemail?")) {
      return;
    }
    try {
      await fetchJSON("/v1/voicemail", { method: "DELETE" });
      await loadVoicemail();
      await pollStatus();
    } catch (err) {
      alert("Clear failed: " + err.message);
    }
  });

  function clearEnvBadges() {
    document.querySelectorAll(".env-badge").forEach(function (b) {
      b.remove();
    });
  }

  function applyEnvBadges(overrides) {
    clearEnvBadges();
    if (!overrides) {
      return;
    }
    Object.keys(overrides).forEach(function (key) {
      if (!overrides[key]) {
        return;
      }
      const label = document.querySelector('label[data-field="' + key + '"]');
      if (!label) {
        return;
      }
      const badge = document.createElement("span");
      badge.className = "env-badge";
      badge.title = "Overridden by ACC_* environment variable";
      badge.textContent = "env";
      label.insertBefore(badge, label.firstChild.nextSibling);
    });
  }

  function fillConfigForm(cfg) {
    CONFIG_KEYS.forEach(function (key) {
      const input = els.configForm.elements.namedItem(key);
      if (!input) {
        return;
      }
      let val = cfg[key];
      if (key === "tray_enabled" && (val === undefined || val === null)) {
        val = true;
      }
      if (BOOL_KEYS.has(key)) {
        input.checked = !!val;
      } else if (NUMBER_KEYS.has(key)) {
        input.value = val != null ? String(val) : "";
      } else {
        input.value = val != null ? String(val) : "";
      }
    });
  }

  /** Always serialize the full DTO so sparse PUT cannot reset bools. */
  function readConfigForm() {
    const out = {};
    CONFIG_KEYS.forEach(function (key) {
      const input = els.configForm.elements.namedItem(key);
      if (!input) {
        return;
      }
      if (BOOL_KEYS.has(key)) {
        out[key] = !!input.checked;
      } else if (NUMBER_KEYS.has(key)) {
        const n = parseInt(input.value, 10);
        out[key] = Number.isFinite(n) ? n : 0;
      } else {
        out[key] = input.value;
      }
    });
    return out;
  }

  async function loadConfig() {
    els.configMsg.textContent = "";
    els.configMsg.classList.remove("error");
    try {
      const data = await fetchJSON("/v1/config");
      fillConfigForm((data && data.config) || {});
      applyEnvBadges((data && data.env_overrides) || {});
    } catch (err) {
      els.configMsg.textContent = "Load failed: " + err.message;
      els.configMsg.classList.add("error");
    }
  }

  function sleep(ms) {
    return new Promise(function (resolve) {
      setTimeout(resolve, ms);
    });
  }

  async function waitHealthy(timeoutMs) {
    const deadline = Date.now() + (timeoutMs || 30000);
    while (Date.now() < deadline) {
      try {
        const h = await fetchJSON("/v1/health");
        if (h && h.ok) {
          return true;
        }
      } catch (_) {
        /* still down */
      }
      await sleep(400);
    }
    return false;
  }

  els.configForm.addEventListener("submit", async function (ev) {
    ev.preventDefault();
    els.configMsg.textContent = "";
    els.configMsg.classList.remove("error");
    const body = readConfigForm();
    els.reconnect.classList.remove("hidden");
    try {
      await fetchJSON("/v1/config", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const ok = await waitHealthy(45000);
      if (!ok) {
        throw new Error("timed out waiting for health after restart");
      }
      await loadConfig();
      await pollStatus();
      els.configMsg.textContent = "Saved.";
    } catch (err) {
      els.configMsg.textContent = "Save failed: " + err.message;
      els.configMsg.classList.add("error");
    } finally {
      els.reconnect.classList.add("hidden");
    }
  });

  pollStatus();
  startPolling();
  connectSSE();
})();
