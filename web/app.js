const state = {
  session: loadSession(),
  demoMode: false,
  authView: "register",
  selectedRegion: "americas",
  me: null,
  profile: null,
  skills: [],
  room: [],
  templates: [],
  rankings: [],
  currentChallenge: null,
  latestRun: null,
  latestSubmission: null,
  latestExplanation: null,
  hints: [],
  chatMessages: [],
  candidates: [],
  chatPollTimer: null,
  flashTimer: null,
};

const elements = {};

window.addEventListener("unhandledrejection", (event) => {
  const message = event.reason && event.reason.message ? event.reason.message : "Request failed";
  showFlash(message, "error");
});

document.addEventListener("DOMContentLoaded", boot);

async function boot() {
  cacheElements();
  bindEvents();
  updateSessionBadge();

  if (state.session.accessToken) {
    try {
      await hydrateWorkspace();
      showWorkspace();
      showFlash("Session restored.", "success");
      return;
    } catch (error) {
      clearSession();
      showFlash(error.message || "Session expired. Please sign in again.", "error");
    }
  }

  showAuth();
}

function cacheElements() {
  [
    "auth-panel",
    "auth-scene",
    "auth-scene-badge",
    "auth-title",
    "auth-copy",
    "auth-region-copy",
    "workspace",
    "flash",
    "logout-button",
    "session-label",
    "user-name",
    "user-meta",
    "profile-score",
    "profile-percentile",
    "profile-confidence",
    "profile-completed",
    "profile-streak",
    "profile-track",
    "overview-global",
    "overview-country",
    "overview-country-code",
    "overview-role",
    "room-status",
    "room-stage",
    "skills-list",
    "rankings-body",
    "template-list",
    "challenge-title",
    "challenge-meta",
    "challenge-description",
    "file-tabs",
    "editor",
    "hint-focus",
    "hint-question",
    "hint-stream",
    "evaluation-grid",
    "explanation-box",
    "chat-peer-id",
    "chat-log",
    "chat-message",
    "candidates-body",
    "mutation-preview",
    "hr-nav-link",
  ].forEach((id) => {
    elements[toCamel(id)] = document.getElementById(id);
  });
}

function bindEvents() {
  document.getElementById("register-form").addEventListener("submit", onRegister);
  document.getElementById("login-form").addEventListener("submit", onLogin);
  elements.logoutButton.addEventListener("click", onLogout);
  document.getElementById("explore-demo").addEventListener("click", enterDemoMode);
  document.getElementById("explore-demo-top").addEventListener("click", enterDemoMode);

  document.querySelectorAll("[data-auth-view]").forEach((button) => {
    button.addEventListener("click", () => setAuthView(button.dataset.authView));
  });
  document.querySelectorAll(".auth-region-button").forEach((button) => {
    button.addEventListener("click", () => setRegion(button.dataset.region));
  });

  document.querySelectorAll(".rail-link").forEach((button) => {
    button.addEventListener("click", () => setScreen(button.dataset.screen));
  });

  document.getElementById("refresh-overview").addEventListener("click", refreshOverview);
  document.getElementById("refresh-rankings").addEventListener("click", loadRankings);
  document.getElementById("refresh-templates").addEventListener("click", loadTemplates);
  document.getElementById("run-checks").addEventListener("click", runChecks);
  document.getElementById("request-hint").addEventListener("click", requestHint);
  document.getElementById("request-explanation").addEventListener("click", requestExplanation);
  document.getElementById("submit-solution").addEventListener("click", submitSolution);
  document.getElementById("refresh-chat").addEventListener("click", loadChatMessages);
  document.getElementById("refresh-candidates").addEventListener("click", loadCandidates);

  document.getElementById("friend-request-form").addEventListener("submit", onFriendRequest);
  document.getElementById("friend-accept-form").addEventListener("submit", onFriendAccept);
  document.getElementById("chat-form").addEventListener("submit", onChatSend);
  document.getElementById("candidate-search-form").addEventListener("submit", onCandidateSearch);
  document.getElementById("company-form").addEventListener("submit", onCreateCompany);
  document.getElementById("job-form").addEventListener("submit", onCreateJob);
  document.getElementById("shortlist-form").addEventListener("submit", onShortlist);
  document.getElementById("mutation-preview-form").addEventListener("submit", onMutationPreview);
  document.getElementById("register-country").addEventListener("input", (event) => {
    event.target.dataset.autofill = "false";
  });

  elements.templateList.addEventListener("click", async (event) => {
    const button = event.target.closest("[data-template-id]");
    if (!button) {
      return;
    }
    await startChallenge(button.dataset.templateId);
  });

  elements.fileTabs.addEventListener("click", (event) => {
    const button = event.target.closest("[data-file-name]");
    if (!button || !state.currentChallenge) {
      return;
    }
    state.currentChallenge.activeFile = button.dataset.fileName;
    renderChallenge();
  });

  elements.editor.addEventListener("input", onEditorInput);
  elements.editor.addEventListener("paste", onEditorPaste);
  elements.chatPeerId.addEventListener("change", startChatPolling);

  document.addEventListener("visibilitychange", onVisibilityChange);
}

async function onRegister(event) {
  event.preventDefault();
  const payload = {
    email: document.getElementById("register-email").value.trim(),
    username: document.getElementById("register-username").value.trim(),
    password: document.getElementById("register-password").value,
    country: document.getElementById("register-country").value.trim().toUpperCase(),
    role: document.getElementById("register-role").value,
  };
  const auth = await api("/v1/auth/register", { method: "POST", body: payload, auth: false });
  setSession(auth);
  setAuthView("register");
  await hydrateWorkspace();
  showWorkspace();
  showFlash("Account created and session opened.", "success");
}

async function onLogin(event) {
  event.preventDefault();
  const payload = {
    email: document.getElementById("login-email").value.trim(),
    password: document.getElementById("login-password").value,
  };
  const auth = await api("/v1/auth/login", { method: "POST", body: payload, auth: false });
  setSession(auth);
  setAuthView("login");
  await hydrateWorkspace();
  showWorkspace();
  showFlash("Welcome back.", "success");
}

function onLogout() {
  const wasDemo = state.demoMode;
  clearSession();
  showAuth();
  showFlash(wasDemo ? "Preview closed." : "Signed out.", "success");
}

async function hydrateWorkspace() {
  const [me, profile, skills, room] = await Promise.all([
    api("/v1/me"),
    api("/v1/profile"),
    api("/v1/skills"),
    api("/v1/room"),
  ]);

  state.me = me;
  state.profile = profile;
  state.skills = skills.skills || [];
  state.room = room.items || [];

  await Promise.all([loadTemplates(), loadRankings()]);
  if (state.me.role === "hr" || state.me.role === "admin") {
    await loadCandidates();
  }

  renderShell();
}

function enterDemoMode() {
  state.demoMode = true;
  seedDemoWorkspace();
  renderShell();
  showWorkspace();
  showFlash("Preview mode enabled.", "success");
}

function renderShell() {
  updateSessionBadge();
  renderProfileSummary();
  renderRoom();
  renderSkills();
  renderRankings();
  renderTemplates();
  renderChallenge();
  renderCandidates();
  renderChat();
  applyRoleVisibility();
}

function renderProfileSummary() {
  if (!state.me || !state.profile) {
    return;
  }

  elements.userName.textContent = state.me.username;
  elements.userMeta.textContent = state.demoMode
    ? "Interactive preview with unlocked product screens"
    : `${state.me.role === "hr" ? "Recruiter" : "Candidate"} ID ${state.me.id}`;
  elements.profileScore.textContent = formatScore(state.profile.current_skill_score);
  elements.profilePercentile.textContent = `${formatScore(state.profile.percentile_global)} global`;
  elements.profileConfidence.textContent = formatScore(state.profile.confidence_score);
  elements.profileCompleted.textContent = String(state.profile.completed_challenges);
  elements.profileStreak.textContent = String(state.profile.streak_days);
  elements.profileTrack.textContent = (state.profile.selected_track || "react").toUpperCase();
  elements.overviewGlobal.textContent = formatScore(state.profile.percentile_global);
  elements.overviewCountry.textContent = formatScore(state.profile.percentile_country);
  elements.overviewCountryCode.textContent = state.me.country || "US";
  elements.overviewRole.textContent = state.demoMode ? "Preview" : state.me.role === "hr" ? "Recruiter" : "Candidate";
}

function renderRoom() {
  const iconMap = {
    monitor: "MN",
    desk: "DK",
    shelf: "SH",
    chair: "CH",
    plant: "PL",
    trophy_case: "TC",
  };

  if (!state.room.length) {
    elements.roomStage.innerHTML = `<div class="empty-state">Start or submit a challenge to build the room.</div>`;
    elements.roomStatus.textContent = "Waiting for challenge activity";
    return;
  }

  elements.roomStatus.textContent = `Updated ${new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`;
  elements.roomStage.innerHTML = state.room
    .map((item) => {
      const glow = item.state_json && item.state_json.glow ? "glow" : "";
      return `
        <article class="room-item ${glow}">
          <div class="room-icon">${iconMap[item.room_item_code] || item.room_item_code.slice(0, 2).toUpperCase()}</div>
          <div>
            <strong>${titleize(item.room_item_code)}</strong>
            <p class="subtle">${titleize(item.current_variant.replaceAll("_", " "))}</p>
          </div>
          <span class="level-badge">${item.current_level}</span>
        </article>
      `;
    })
    .join("");
}

function renderSkills() {
  if (!state.skills.length) {
    elements.skillsList.innerHTML = `<div class="empty-state">No skill entries yet.</div>`;
    return;
  }

  elements.skillsList.innerHTML = state.skills
    .map(
      (skill) => `
        <div class="skill-chip">
          <div>
            <strong>${titleize(skill.skill_code)}</strong>
            <div class="subtle">${skill.level}</div>
          </div>
          <div>${formatScore(skill.score)}</div>
        </div>
      `
    )
    .join("");
}

async function refreshOverview() {
  if (state.demoMode) {
    renderShell();
    showFlash("Preview data refreshed.", "success");
    return;
  }
  await hydrateWorkspace();
  showFlash("Overview refreshed.", "success");
}

async function loadRankings() {
  if (state.demoMode) {
    renderRankings();
    return;
  }
  const payload = await api("/v1/rankings/global");
  state.rankings = payload.rankings || [];
  renderRankings();
}

function renderRankings() {
  if (!elements.rankingsBody) {
    return;
  }

  elements.rankingsBody.innerHTML = state.rankings
    .map(
      (entry) => `
        <tr>
          <td>${entry.rank}</td>
          <td>${entry.username}</td>
          <td>${entry.country || "-"}</td>
          <td>${formatScore(entry.current_skill_score)}</td>
          <td>${formatScore(entry.confidence_score)}</td>
          <td><code>${entry.user_id}</code></td>
        </tr>
      `
    )
    .join("");
}

async function loadTemplates() {
  if (state.demoMode) {
    renderTemplates();
    return;
  }
  const payload = await api("/v1/challenges/templates");
  state.templates = payload.templates || [];
  renderTemplates();
}

function renderTemplates() {
  if (!state.templates.length) {
    elements.templateList.innerHTML = `<div class="empty-state">No challenge templates available.</div>`;
    return;
  }

  const currentTemplateId = state.currentChallenge ? state.currentChallenge.template_id : "";
  elements.templateList.innerHTML = state.templates
    .map(
      (template) => `
        <article class="template-card ${currentTemplateId === template.id ? "active" : ""}">
          <div class="panel-header tight">
            <div>
              <h3>${template.title}</h3>
              <p>${titleize(template.category)} · Difficulty ${template.difficulty}</p>
            </div>
            <button class="primary" type="button" data-template-id="${template.id}">Start</button>
          </div>
          <div class="challenge-meta">
            <span class="meta-pill">${template.track.toUpperCase()}</span>
            <span class="meta-pill">${template.evaluation_config_json.max_attempts} attempts</span>
          </div>
        </article>
      `
    )
    .join("");
}

async function startChallenge(templateId) {
  if (state.demoMode) {
    state.currentChallenge = buildDemoChallenge(templateId);
    state.latestRun = null;
    state.latestSubmission = null;
    state.latestExplanation = null;
    state.hints = [];
    setScreen("challenge");
    renderChallenge();
    showFlash(`Loaded demo challenge ${state.currentChallenge.title}.`, "success");
    return;
  }

  const view = await api("/v1/challenges/instances", {
    method: "POST",
    body: { template_id: templateId },
  });

  const files = {
    ...(view.variant.generated_files || {}),
    ...(view.visible_tests || {}),
  };
  const editableFiles = Array.isArray(view.editable_files) ? view.editable_files : Object.keys(view.variant.generated_files || {});
  state.currentChallenge = {
    ...view,
    files,
    editableFiles,
    activeFile: editableFiles[0] || Object.keys(files)[0],
    startedAtMs: new Date(view.instance.started_at).getTime(),
    telemetry: {
      firstInputSent: false,
      lastSnapshotSentAt: 0,
      snapshotTimer: null,
      events: [],
    },
  };
  state.latestRun = null;
  state.latestSubmission = null;
  state.latestExplanation = null;
  state.hints = [];
  setScreen("challenge");
  renderChallenge();
  showFlash(`Started ${view.title}.`, "success");
}

function renderChallenge() {
  if (!state.currentChallenge) {
    elements.challengeTitle.textContent = "No challenge selected";
    elements.challengeMeta.innerHTML = `<span class="meta-pill">Choose a template from the task bank</span>`;
    elements.challengeDescription.textContent = "A started task will load the prompt, README, visible tests, and source file into the room.";
    elements.fileTabs.innerHTML = "";
    elements.editor.value = "";
    elements.editor.readOnly = true;
    elements.hintStream.textContent = "No hints requested yet.";
    elements.evaluationGrid.innerHTML = "";
    elements.explanationBox.textContent = "Submit a solution to generate a structured explanation.";
    return;
  }

  const challenge = state.currentChallenge;
  const editableFiles = Array.isArray(challenge.editableFiles) ? challenge.editableFiles : ["src/App.tsx"];
  elements.challengeTitle.textContent = challenge.title;
  elements.challengeMeta.innerHTML = `
    <span class="meta-pill">${titleize(challenge.category)}</span>
    <span class="meta-pill">Difficulty ${challenge.difficulty}</span>
    <span class="meta-pill">Seed ${challenge.variant.seed}</span>
    <span class="meta-pill">Status ${challenge.instance.status}</span>
  `;
  elements.challengeDescription.textContent = challenge.description_md || challenge.description || "";

  const fileNames = Object.keys(challenge.files || {});
  elements.fileTabs.innerHTML = fileNames
    .map(
      (fileName) => `
        <button class="file-tab ${challenge.activeFile === fileName ? "active" : ""}" type="button" data-file-name="${escapeHtml(fileName)}">
          ${escapeHtml(fileName)}
        </button>
      `
    )
    .join("");

  const activeFile = challenge.activeFile;
  const content = challenge.files[activeFile] || "";
  elements.editor.value = content;
  elements.editor.readOnly = !editableFiles.includes(activeFile);

  renderHints();
  renderEvaluation();
  renderExplanation();
}

function onEditorInput(event) {
  if (!state.currentChallenge || !editableFilesForChallenge(state.currentChallenge).includes(state.currentChallenge.activeFile)) {
    return;
  }
  state.currentChallenge.files[state.currentChallenge.activeFile] = event.target.value;

  if (!state.currentChallenge.telemetry.firstInputSent) {
    state.currentChallenge.telemetry.firstInputSent = true;
    sendTelemetry("input", {});
  }
  scheduleSnapshot();
}

function onEditorPaste() {
  if (!state.currentChallenge || !editableFilesForChallenge(state.currentChallenge).includes(state.currentChallenge.activeFile)) {
    return;
  }
  sendTelemetry("paste", {
    active_file: state.currentChallenge.activeFile,
    char_count: (state.currentChallenge.files[state.currentChallenge.activeFile] || "").length,
  });
}

function scheduleSnapshot() {
  const telemetry = state.currentChallenge.telemetry;
  clearTimeout(telemetry.snapshotTimer);
  telemetry.snapshotTimer = window.setTimeout(() => {
    if (!state.currentChallenge) {
      return;
    }
    const now = Date.now();
    if (now - telemetry.lastSnapshotSentAt < 8000) {
      return;
    }
    telemetry.lastSnapshotSentAt = now;
    const source = editableSourceText(state.currentChallenge);
    sendTelemetry("snapshot", {
      active_file: state.currentChallenge.activeFile,
      line_count: source.split("\n").length,
      char_count: source.length,
    });
  }, 1200);
}

async function sendTelemetry(eventType, payload) {
  if (!state.currentChallenge) {
    return;
  }
  const offsetSeconds = Math.max(0, Math.round((Date.now() - state.currentChallenge.startedAtMs) / 1000));
  if (state.demoMode) {
    state.currentChallenge.telemetry.events.push({
      event_type: eventType,
      offset_seconds: offsetSeconds,
      payload,
    });
    return;
  }
  try {
    await api(`/v1/challenges/instances/${state.currentChallenge.instance.id}/telemetry`, {
      method: "POST",
      body: {
        event_type: eventType,
        offset_seconds: offsetSeconds,
        payload,
      },
    });
  } catch (error) {
    console.warn("telemetry failed", error);
  }
}

async function submitSolution() {
  if (!state.currentChallenge) {
    showFlash("Start a challenge first.", "error");
    return;
  }
  if (state.demoMode) {
    submitDemoSolution();
    return;
  }
  const payload = await api(`/v1/challenges/instances/${state.currentChallenge.instance.id}/submissions`, {
    method: "POST",
    body: {
      language: "jsx",
      source_files: editableSourceFiles(state.currentChallenge),
    },
  });
  state.latestRun = null;
  state.latestSubmission = payload;
  state.latestExplanation = null;
  state.currentChallenge.instance.status = payload.submission.execution_status || "done";
  await refreshOverview();
  renderChallenge();
  showFlash(`Submitted. Suspicion level: ${payload.anti_cheat.level}.`, "success");
}

async function runChecks() {
  if (!state.currentChallenge) {
    showFlash("Start a challenge first.", "error");
    return;
  }
  if (state.demoMode) {
    runDemoChecks();
    return;
  }
  const payload = await api(`/v1/challenges/instances/${state.currentChallenge.instance.id}/runs`, {
    method: "POST",
    body: {
      language: "jsx",
      source_files: editableSourceFiles(state.currentChallenge),
    },
  });
  state.latestRun = payload;
  state.latestSubmission = null;
  state.latestExplanation = null;
  renderChallenge();
  showFlash(`Run complete. ${payload.execution.tests_passed}/${payload.execution.tests_total} checks passed.`, "success");
}

function renderEvaluation() {
  if (state.latestSubmission) {
    const evaluation = state.latestSubmission.evaluation;
    const execution = state.latestSubmission.execution;
    const antiCheat = state.latestSubmission.anti_cheat;
    const passedChecks = (execution.checks || []).filter((check) => check.passed).length;
    elements.evaluationGrid.innerHTML = `
      <div><span>Mode</span><strong>Final submission</strong></div>
      <div><span>Final score</span><strong>${formatScore(evaluation.final_score)}</strong></div>
      <div><span>Correctness</span><strong>${formatScore(evaluation.test_score)}</strong></div>
      <div><span>Quality</span><strong>${formatScore(evaluation.quality_score)}</strong></div>
      <div><span>Speed</span><strong>${formatScore(evaluation.speed_score)}</strong></div>
      <div><span>Consistency</span><strong>${formatScore(evaluation.consistency_score)}</strong></div>
      <div><span>Checks</span><strong>${execution.tests_passed}/${execution.tests_total}</strong></div>
      <div><span>Passed checks</span><strong>${passedChecks}</strong></div>
      <div><span>Lint issues</span><strong>${execution.lint_errors}</strong></div>
      <div><span>Render count</span><strong>${execution.render_count}</strong></div>
      <div><span>Exec ms</span><strong>${execution.exec_ms}</strong></div>
      <div><span>Suspicion</span><strong>${antiCheat.level.toUpperCase()}</strong></div>
      <div><span>Similarity</span><strong>${formatScore(antiCheat.similarity_score)}</strong></div>
    `;
    return;
  }

  if (!state.latestRun) {
    elements.evaluationGrid.innerHTML = `<div class="empty-state">No evaluation yet.</div>`;
    return;
  }

  const execution = state.latestRun.execution;
  const telemetry = state.latestRun.telemetry || {};
  const checks = execution.checks || [];
  elements.evaluationGrid.innerHTML = `
    <div><span>Mode</span><strong>Run preview</strong></div>
    <div><span>Checks</span><strong>${execution.tests_passed}/${execution.tests_total}</strong></div>
    <div><span>Lint issues</span><strong>${execution.lint_errors}</strong></div>
    <div><span>Render count</span><strong>${execution.render_count}</strong></div>
    <div><span>Exec ms</span><strong>${execution.exec_ms}</strong></div>
    <div><span>Solve time</span><strong>${execution.solve_time_seconds}s</strong></div>
    <div><span>Edits</span><strong>${execution.edit_count}</strong></div>
    <div><span>Pastes</span><strong>${execution.paste_events}</strong></div>
    <div><span>Focus loss</span><strong>${execution.focus_loss_events}</strong></div>
    <div><span>Snapshots</span><strong>${execution.snapshot_events}</strong></div>
    <div><span>First input</span><strong>${telemetry.time_to_first_input_seconds || execution.first_input_seconds}s</strong></div>
    <div><span>Check detail</span><strong>${checks.filter((check) => check.passed).length}/${checks.length}</strong></div>
  `;
}

function editableSourceFiles(challenge) {
  const files = {};
  editableFilesForChallenge(challenge).forEach((fileName) => {
    files[fileName] = challenge.files[fileName] || "";
  });
  return files;
}

function editableSourceText(challenge) {
  return editableFilesForChallenge(challenge)
    .map((fileName) => challenge.files[fileName] || "")
    .join("\n");
}

function editableFilesForChallenge(challenge) {
  return Array.isArray(challenge.editableFiles) && challenge.editableFiles.length > 0
    ? challenge.editableFiles
    : ["src/App.tsx"];
}

async function requestHint() {
  if (!state.currentChallenge) {
    showFlash("Start a challenge before asking for a hint.", "error");
    return;
  }
  if (state.demoMode) {
    requestDemoHint();
    return;
  }
  const response = await api(`/v1/ai/challenges/${state.currentChallenge.instance.id}/hint`, {
    method: "POST",
    body: {
      focus_area: elements.hintFocus.value.trim(),
      question: elements.hintQuestion.value.trim(),
    },
  });
  state.hints.push(response);
  renderHints();
  showFlash(`Hint delivered. ${response.remaining_hints} remaining.`, "success");
}

function renderHints() {
  if (!state.hints.length) {
    elements.hintStream.textContent = "No hints requested yet.";
    return;
  }
  elements.hintStream.innerHTML = state.hints
    .map(
      (hint) => `
        <div class="hint-item">
          <strong>${escapeHtml(hint.focus_area || "General guidance")}</strong>
          <div>${escapeHtml(hint.hint)}</div>
          <small>${hint.provider} · ${hint.remaining_hints} hints remaining</small>
        </div>
      `
    )
    .join("");
}

async function requestExplanation() {
  if (!state.currentChallenge || !state.latestSubmission) {
    showFlash("Start and submit a challenge first.", "error");
    return;
  }
  if (state.demoMode) {
    requestDemoExplanation();
    return;
  }
  const response = await api(`/v1/ai/challenges/${state.currentChallenge.instance.id}/explain`, {
    method: "POST",
    body: {},
  });
  state.latestExplanation = response;
  renderExplanation();
  showFlash("Explanation generated.", "success");
}

function renderExplanation() {
  if (!state.latestExplanation) {
    elements.explanationBox.textContent = "Submit a solution to generate a structured explanation.";
    return;
  }

  const explanation = state.latestExplanation;
  elements.explanationBox.innerHTML = `
    <strong>${escapeHtml(explanation.summary)}</strong>
    <br /><br />
    <strong>Strengths</strong>
    <ul>${explanation.strengths.map((item) => `<li>${escapeHtml(item)}</li>`).join("")}</ul>
    <strong>Improvements</strong>
    <ul>${explanation.improvements.map((item) => `<li>${escapeHtml(item)}</li>`).join("")}</ul>
    <strong>Suspicion notes</strong>
    <ul>${explanation.suspicion_notes.map((item) => `<li>${escapeHtml(item)}</li>`).join("")}</ul>
    <strong>Next step</strong>
    <div>${escapeHtml(explanation.recommended_next)}</div>
  `;
}

async function onFriendRequest(event) {
  event.preventDefault();
  const userId = document.getElementById("friend-request-user-id").value.trim();
  if (!userId) {
    return;
  }
  if (state.demoMode) {
    showFlash(`Demo friend request queued for ${userId}.`, "success");
    return;
  }
  await api(`/v1/friends/${encodeURIComponent(userId)}/request`, { method: "POST" });
  showFlash("Friend request sent.", "success");
}

async function onFriendAccept(event) {
  event.preventDefault();
  const userId = document.getElementById("friend-accept-user-id").value.trim();
  if (!userId) {
    return;
  }
  if (state.demoMode) {
    showFlash(`Demo friend request accepted for ${userId}.`, "success");
    return;
  }
  await api(`/v1/friends/${encodeURIComponent(userId)}/accept`, { method: "POST" });
  showFlash("Friend request accepted.", "success");
}

function startChatPolling() {
  if (state.chatPollTimer) {
    clearInterval(state.chatPollTimer);
  }
  if (state.demoMode) {
    renderChat();
    return;
  }
  const peerId = elements.chatPeerId.value.trim();
  if (!peerId) {
    return;
  }
  loadChatMessages();
  state.chatPollTimer = window.setInterval(loadChatMessages, 12000);
}

async function loadChatMessages() {
  if (state.demoMode) {
    renderChat();
    return;
  }
  const peerId = elements.chatPeerId.value.trim();
  if (!peerId) {
    elements.chatLog.textContent = "Choose a peer user ID to load direct chat.";
    return;
  }
  try {
    const payload = await api(`/v1/chat/direct/${encodeURIComponent(peerId)}/messages`);
    state.chatMessages = payload.messages || [];
    renderChat();
  } catch (error) {
    elements.chatLog.textContent = error.message;
  }
}

function renderChat() {
  if (!state.chatMessages.length) {
    elements.chatLog.textContent = "Chat is empty.";
    return;
  }
  elements.chatLog.innerHTML = state.chatMessages
    .map(
      (message) => `
        <div class="chat-message">
          <strong>${escapeHtml(message.sender_user_id)}</strong>
          <div>${escapeHtml(message.body)}</div>
          <small>${new Date(message.created_at).toLocaleString()}</small>
        </div>
      `
    )
    .join("");
}

async function onChatSend(event) {
  event.preventDefault();
  const peerId = elements.chatPeerId.value.trim();
  const body = elements.chatMessage.value.trim();
  if (!peerId || !body) {
    return;
  }
  if (state.demoMode) {
    state.chatMessages.push({
      sender_user_id: state.me.username,
      body,
      created_at: new Date().toISOString(),
    });
    elements.chatMessage.value = "";
    renderChat();
    showFlash("Demo message sent.", "success");
    return;
  }
  await api(`/v1/chat/direct/${encodeURIComponent(peerId)}/messages`, {
    method: "POST",
    body: { body },
  });
  elements.chatMessage.value = "";
  await loadChatMessages();
  showFlash("Message sent.", "success");
}

async function onCandidateSearch(event) {
  event.preventDefault();
  await loadCandidates();
}

async function loadCandidates() {
  if (!state.me || (!state.demoMode && state.me.role !== "hr" && state.me.role !== "admin")) {
    return;
  }
  if (state.demoMode) {
    const minScore = Number(document.getElementById("candidate-min-score").value || "20");
    const topPercent = Number(document.getElementById("candidate-top-percent").value || "100");
    state.candidates = demoCandidates()
      .filter((candidate) => candidate.current_skill_score >= minScore)
      .filter((candidate) => candidate.percentile_global <= topPercent)
      .sort((left, right) => right.current_skill_score - left.current_skill_score);
    renderCandidates();
    return;
  }
  const params = new URLSearchParams({
    min_score: document.getElementById("candidate-min-score").value || "20",
    top_percent: document.getElementById("candidate-top-percent").value || "100",
    active_days: document.getElementById("candidate-active-days").value || "14",
  });
  const payload = await api(`/v1/hr/candidates?${params.toString()}`);
  state.candidates = payload.candidates || [];
  renderCandidates();
}

function renderCandidates() {
  if (!elements.candidatesBody) {
    return;
  }
  if (!state.candidates.length) {
    elements.candidatesBody.innerHTML = `<tr><td colspan="5">No candidate results yet.</td></tr>`;
    return;
  }
  elements.candidatesBody.innerHTML = state.candidates
    .map(
      (candidate) => `
        <tr>
          <td>${candidate.username}</td>
          <td>${candidate.country}</td>
          <td>${formatScore(candidate.current_skill_score)}</td>
          <td>${formatScore(candidate.confidence_score)}</td>
          <td><code>${candidate.user_id}</code></td>
        </tr>
      `
    )
    .join("");
}

async function onCreateCompany(event) {
  event.preventDefault();
  if (state.demoMode) {
    const demoCompanyID = "cmp_demo_preview";
    document.getElementById("job-company-id").value = demoCompanyID;
    document.getElementById("shortlist-company-id").value = demoCompanyID;
    showFlash(`Demo company created: ${demoCompanyID}`, "success");
    return;
  }
  const payload = await api("/v1/hr/companies", {
    method: "POST",
    body: {
      name: document.getElementById("company-name").value.trim(),
      description: document.getElementById("company-description").value.trim(),
    },
  });
  document.getElementById("job-company-id").value = payload.id;
  document.getElementById("shortlist-company-id").value = payload.id;
  showFlash(`Company created: ${payload.id}`, "success");
}

async function onCreateJob(event) {
  event.preventDefault();
  const companyId = document.getElementById("job-company-id").value.trim();
  if (state.demoMode) {
    showFlash(`Demo job created for ${companyId || "cmp_demo_preview"}.`, "success");
    return;
  }
  const payload = {
    title: document.getElementById("job-title").value.trim(),
    description: document.getElementById("job-description").value.trim(),
    required_score: Number(document.getElementById("job-required-score").value || "0"),
    required_skills_json: { react: 60 },
  };
  const job = await api(`/v1/hr/companies/${encodeURIComponent(companyId)}/jobs`, {
    method: "POST",
    body: payload,
  });
  showFlash(`Job created: ${job.id}`, "success");
}

async function onShortlist(event) {
  event.preventDefault();
  if (state.demoMode) {
    showFlash("Demo shortlist saved.", "success");
    return;
  }
  const payload = await api("/v1/hr/shortlists", {
    method: "POST",
    body: {
      company_id: document.getElementById("shortlist-company-id").value.trim(),
      user_id: document.getElementById("shortlist-user-id").value.trim(),
      status: document.getElementById("shortlist-status").value.trim(),
      notes: document.getElementById("shortlist-notes").value.trim(),
    },
  });
  showFlash(`Shortlist saved: ${payload.id}`, "success");
}

async function onMutationPreview(event) {
  event.preventDefault();
  const templateId = document.getElementById("mutation-template-id").value.trim();
  const seed = Number(document.getElementById("mutation-seed").value || "0");
  if (state.demoMode) {
    const preview = demoMutationPreview(templateId, seed);
    renderMutationPreview(preview);
    showFlash(`Preview generated from seed ${preview.seed}.`, "success");
    return;
  }
  const preview = await api(`/v1/hr/ai/templates/${encodeURIComponent(templateId)}/mutation-preview`, {
    method: "POST",
    body: { seed },
  });
  renderMutationPreview(preview);
  showFlash(`Preview generated from seed ${preview.seed}.`, "success");
}

function seedDemoWorkspace() {
  const config = regionConfig(state.selectedRegion) || regionConfig("americas");
  state.me = {
    id: "usr_preview",
    username: "Preview Operator",
    country: config.country,
    role: "user",
  };
  state.profile = {
    user_id: state.me.id,
    selected_track: "react",
    current_skill_score: 81,
    percentile_global: 92,
    percentile_country: config.percentileCountry,
    streak_days: 6,
    confidence_score: 88,
    completed_challenges: 12,
  };
  state.skills = cloneData(demoSkills());
  state.room = cloneData(demoRoomItems());
  state.rankings = cloneData(demoRankings());
  state.templates = cloneData(demoTemplates());
  state.candidates = cloneData(demoCandidates());
  state.chatMessages = cloneData(demoChatMessages());
  state.currentChallenge = buildDemoChallenge("react_feature_search");
  state.latestRun = null;
  state.latestSubmission = null;
  state.latestExplanation = null;
  state.hints = [];
  syncDemoRanking();
  elements.chatPeerId.value = "usr_recruiter_lena";
  document.getElementById("job-company-id").value = "cmp_demo_preview";
  document.getElementById("shortlist-company-id").value = "cmp_demo_preview";
  document.getElementById("shortlist-user-id").value = "usr_candidate_ava";
  document.getElementById("mutation-template-id").value = "react_feature_search";
  document.getElementById("mutation-seed").value = "1729";
  elements.mutationPreview.textContent = "Run a mutation preview to inspect how the task surface can be changed for each candidate.";
}

function regionConfig(region) {
  const regions = {
    americas: {
      key: "americas",
      country: "US",
      percentileCountry: 95,
      sceneLabel: "Americas room",
      description: "Browse the challenge bank, recruiter desk, rankings, and chat flows from the Americas launch room before creating an account.",
    },
    europe: {
      key: "europe",
      country: "DE",
      percentileCountry: 92,
      sceneLabel: "Europe room",
      description: "Switch the first scene to the Europe room and preview the product before creating an account.",
    },
    mena: {
      key: "mena",
      country: "AE",
      percentileCountry: 89,
      sceneLabel: "MENA room",
      description: "Explore the MENA room setup, then inspect rankings, recruiter workflows, and the challenge bank without signing in.",
    },
    apac: {
      key: "apac",
      country: "SG",
      percentileCountry: 93,
      sceneLabel: "APAC room",
      description: "Preview the APAC launch room and move through the room, tasks, and recruiter desk before authentication.",
    },
  };
  return regions[region] || null;
}

function demoSkills() {
  return [
    { skill_code: "react", level: "gold", score: 86 },
    { skill_code: "state_management", level: "gold", score: 82 },
    { skill_code: "performance", level: "silver", score: 76 },
    { skill_code: "debugging", level: "silver", score: 74 },
    { skill_code: "testing", level: "silver", score: 69 },
  ];
}

function demoRoomItems() {
  return [
    { room_item_code: "monitor", current_level: "gold", current_variant: "signal_monitor", state_json: { glow: true } },
    { room_item_code: "desk", current_level: "gold", current_variant: "adaptive_desk", state_json: { glow: false } },
    { room_item_code: "shelf", current_level: "silver", current_variant: "task_bank", state_json: { glow: false } },
    { room_item_code: "chair", current_level: "gold", current_variant: "architecture_throne", state_json: { glow: true } },
    { room_item_code: "plant", current_level: "silver", current_variant: "streak_growth", state_json: { glow: false } },
    { room_item_code: "trophy_case", current_level: "silver", current_variant: "top_percentile", state_json: { glow: true } },
  ];
}

function demoRankings() {
  return rankDemoEntries([
    {
      user_id: "usr_archer",
      username: "Archer",
      country: "DE",
      current_skill_score: 90,
      confidence_score: 93,
      percentile_global: 4,
    },
    {
      user_id: "usr_preview",
      username: "Preview Operator",
      country: "US",
      current_skill_score: 81,
      confidence_score: 88,
      percentile_global: 8,
    },
    {
      user_id: "usr_candidate_ava",
      username: "Ava",
      country: "US",
      current_skill_score: 78,
      confidence_score: 84,
      percentile_global: 12,
    },
    {
      user_id: "usr_candidate_mina",
      username: "Mina",
      country: "BR",
      current_skill_score: 74,
      confidence_score: 80,
      percentile_global: 19,
    },
    {
      user_id: "usr_candidate_omar",
      username: "Omar",
      country: "AE",
      current_skill_score: 69,
      confidence_score: 73,
      percentile_global: 24,
    },
  ]);
}

function demoCandidates() {
  return [
    {
      user_id: "usr_candidate_ava",
      username: "Ava",
      country: "US",
      current_skill_score: 78,
      confidence_score: 84,
      percentile_global: 12,
    },
    {
      user_id: "usr_candidate_mina",
      username: "Mina",
      country: "BR",
      current_skill_score: 74,
      confidence_score: 80,
      percentile_global: 19,
    },
    {
      user_id: "usr_candidate_omar",
      username: "Omar",
      country: "AE",
      current_skill_score: 69,
      confidence_score: 73,
      percentile_global: 24,
    },
    {
      user_id: "usr_candidate_jiho",
      username: "Jiho",
      country: "KR",
      current_skill_score: 65,
      confidence_score: 70,
      percentile_global: 31,
    },
  ];
}

function demoChatMessages() {
  return [
    {
      sender_user_id: "usr_recruiter_lena",
      body: "I opened your room preview. The performance task and room evolution stand out.",
      created_at: new Date(Date.now() - 18 * 60 * 1000).toISOString(),
    },
    {
      sender_user_id: "Preview Operator",
      body: "The task bank mutation preview and run feedback are the parts I wanted to inspect first.",
      created_at: new Date(Date.now() - 11 * 60 * 1000).toISOString(),
    },
  ];
}

function demoTemplates() {
  return [
    {
      id: "react_feature_search",
      title: "Build a search board",
      category: "feature",
      difficulty: 2,
      track: "react",
      evaluation_config_json: { max_attempts: 2 },
      description_md:
        "Create a responsive candidate search interface with local filtering, empty states, and a stable render path.",
      demo_files: {
        "README.md": "# Search board\nCreate a candidate search board with local filtering, a clear empty state, and responsive query handling.",
        "src/App.tsx": `import { useMemo, useState } from "react";

const candidates = [
  "Ada Lovelace",
  "Grace Hopper",
  "Margaret Hamilton",
  "Radia Perlman",
  "Evelyn Boyd",
];

export default function SearchBoard() {
  const [query, setQuery] = useState("");

  const filtered = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return candidates.filter((candidate) => candidate.toLowerCase().includes(normalized));
  }, [query]);

  return (
    <section className="board">
      <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search candidates" />
      {filtered.length === 0 ? <p>No results</p> : <ul>{filtered.map((candidate) => <li key={candidate}>{candidate}</li>)}</ul>}
    </section>
  );
}`,
        "tests/visible.spec.ts": `it("shows an empty state when nothing matches", () => {
  expect(screen.getByText("No results")).toBeInTheDocument();
});`,
      },
    },
    {
      id: "react_performance_rerenders",
      title: "Stabilize row rerenders",
      category: "performance",
      difficulty: 3,
      track: "react",
      evaluation_config_json: { max_attempts: 2 },
      description_md:
        "Reduce rerender fan-out in a candidate list by stabilizing callbacks and memoizing derived row data.",
      demo_files: {
        "README.md": "# Rerender control\nUse memoization and stable callbacks to keep a large list responsive.",
        "src/App.tsx": `import { memo, useCallback, useMemo, useState } from "react";

const rows = Array.from({ length: 50 }, (_, index) => ({ id: index + 1, name: "Candidate " + (index + 1) }));

const CandidateRow = memo(function CandidateRow({ row, onSelect }) {
  return <button onClick={() => onSelect(row.id)}>{row.name}</button>;
});

export default function CandidateBoard() {
  const [selected, setSelected] = useState(null);
  const visibleRows = useMemo(() => rows.slice(0, 20), []);
  const handleSelect = useCallback((id) => setSelected(id), []);

  return (
    <section>
      <p>Selected: {selected ?? "none"}</p>
      {visibleRows.map((row) => <CandidateRow key={row.id} row={row} onSelect={handleSelect} />)}
    </section>
  );
}`,
        "tests/visible.spec.ts": `it("renders only the visible rows", () => {
  expect(screen.getByText("Selected: none")).toBeInTheDocument();
});`,
      },
    },
    {
      id: "react_debug_effect_loop",
      title: "Stop an effect loop",
      category: "debug",
      difficulty: 3,
      track: "react",
      evaluation_config_json: { max_attempts: 2 },
      description_md:
        "Fix an unstable effect dependency without removing the live filtering behavior.",
      demo_files: {
        "README.md": "# Effect loop\nStabilize derived filter state and preserve the filtering behavior.",
        "src/App.tsx": `import { useEffect, useMemo, useState } from "react";

const source = ["frontend", "backend", "platform", "design"];

export default function LoopFixer() {
  const [query, setQuery] = useState("front");
  const filters = useMemo(() => ({ query: query.trim().toLowerCase() }), [query]);
  const [matches, setMatches] = useState([]);

  useEffect(() => {
    setMatches(source.filter((item) => item.includes(filters.query)));
  }, [filters]);

  return (
    <section>
      <input value={query} onChange={(event) => setQuery(event.target.value)} />
      <pre>{JSON.stringify(matches, null, 2)}</pre>
    </section>
  );
}`,
        "tests/visible.spec.ts": `it("keeps filtering active", () => {
  expect(screen.getByDisplayValue("front")).toBeInTheDocument();
});`,
      },
    },
    {
      id: "react_logic_custom_hook",
      title: "Design a reusable custom hook",
      category: "logic",
      difficulty: 2,
      track: "react",
      evaluation_config_json: { max_attempts: 2 },
      description_md:
        "Create a custom hook that manages async state and exposes a stable API for the consuming component.",
      demo_files: {
        "README.md": "# Custom hook\nExport a hook with loading, error, and retry support.",
        "src/App.tsx": `import { useEffect, useState } from "react";

export function useCandidateFeed() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [items, setItems] = useState([]);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setItems(["Ava", "Mina", "Omar"]);
      setLoading(false);
    }, 400);
    return () => window.clearTimeout(timer);
  }, []);

  return { loading, error, items, retry: () => setError("") };
}

export default function CandidateFeed() {
  const { loading, items } = useCandidateFeed();
  return <section>{loading ? <p>Loading</p> : items.map((item) => <div key={item}>{item}</div>)}</section>;
}`,
        "tests/visible.spec.ts": `it("shows loading first", () => {
  expect(screen.getByText("Loading")).toBeInTheDocument();
});`,
      },
    },
    {
      id: "react_refactor_split_component",
      title: "Refactor a dense dashboard",
      category: "refactor",
      difficulty: 2,
      track: "react",
      evaluation_config_json: { max_attempts: 2 },
      description_md:
        "Split a large dashboard into smaller parts without losing the render path or interaction flow.",
      demo_files: {
        "README.md": "# Refactor dashboard\nExtract helpers or presentational units so the component is easier to maintain.",
        "src/App.tsx": `const metrics = [
  { label: "Score", value: 82 },
  { label: "Confidence", value: 88 },
];

export default function Dashboard() {
  return (
    <section>
      <header>
        <h1>Candidate Dashboard</h1>
      </header>
      <div>{metrics.map((metric) => <article key={metric.label}><strong>{metric.value}</strong><span>{metric.label}</span></article>)}</div>
    </section>
  );
}`,
        "tests/visible.spec.ts": `it("keeps the dashboard visible", () => {
  expect(screen.getByText("Candidate Dashboard")).toBeInTheDocument();
});`,
      },
    },
  ];
}

function buildDemoChallenge(templateID) {
  const template = cloneData(demoTemplates().find((item) => item.id === templateID) || demoTemplates()[0]);
  const files = cloneData(template.demo_files);
  const now = new Date();
  return {
    instance: {
      id: `demo_${template.id}`,
      status: "assigned",
      started_at: now.toISOString(),
    },
    template_id: template.id,
    title: template.title,
    description_md: template.description_md,
    category: template.category,
    difficulty: template.difficulty,
    variant: {
      seed: 1729,
      generated_files: files,
    },
    files,
    activeFile: "src/App.tsx",
    startedAtMs: now.getTime(),
    telemetry: {
      firstInputSent: false,
      lastSnapshotSentAt: 0,
      snapshotTimer: null,
      events: [],
    },
  };
}

function runDemoChecks() {
  const execution = buildDemoExecution(false);
  state.latestRun = {
    status: "preview",
    instance_id: state.currentChallenge.instance.id,
    template_id: state.currentChallenge.template_id,
    execution,
    telemetry: execution.telemetry,
  };
  state.latestSubmission = null;
  state.latestExplanation = null;
  renderChallenge();
  showFlash(`Preview ready. ${execution.tests_passed}/${execution.tests_total} checks passed.`, "success");
}

function submitDemoSolution() {
  const execution = buildDemoExecution(true);
  const correctness = execution.tests_total ? (execution.tests_passed / execution.tests_total) * 100 : 0;
  const quality = clamp(100 - execution.lint_errors * 12 + (execution.tests_passed >= 4 ? 6 : 0), 42, 96);
  const speed = clamp(100 - Math.max(0, execution.solve_time_seconds - 420) / 6, 58, 97);
  const consistency = clamp(state.profile.confidence_score - execution.paste_events * 5 + 4, 55, 96);
  const finalScore = round((correctness * 0.4) + (quality * 0.2) + (speed * 0.2) + (consistency * 0.2));
  const antiCheat = demoAntiCheat(execution);

  state.profile.current_skill_score = round((state.profile.current_skill_score * 0.78) + (finalScore * 0.22));
  state.profile.percentile_global = clamp(round(state.profile.percentile_global + 1.4), 0, 99);
  state.profile.percentile_country = clamp(round(state.profile.percentile_country + 0.9), 0, 99);
  state.profile.confidence_score = clamp(round((state.profile.confidence_score * 0.82) + (quality * 0.18)), 0, 100);
  state.profile.completed_challenges += 1;
  state.currentChallenge.instance.status = "done";

  updateDemoSkills(finalScore);
  updateDemoRoom();
  syncDemoRanking();

  state.latestRun = null;
  state.latestSubmission = {
    submission: {
      id: `demo_sub_${Date.now()}`,
      execution_status: "done",
      submitted_at: new Date().toISOString(),
    },
    evaluation: {
      final_score: finalScore,
      test_score: round(correctness),
      quality_score: round(quality),
      speed_score: round(speed),
      consistency_score: round(consistency),
    },
    execution,
    anti_cheat: antiCheat,
    telemetry: antiCheat.signals,
  };
  state.latestExplanation = null;
  renderShell();
  setScreen("challenge");
  showFlash(`Demo submission scored at ${formatScore(finalScore)}.`, "success");
}

function buildDemoExecution(isFinal) {
  const code = state.currentChallenge.files["src/App.tsx"] || "";
  const checks = demoCheckResults(state.currentChallenge.template_id, code);
  const telemetry = summarizeDemoTelemetry(state.currentChallenge.telemetry.events || [], state.currentChallenge.startedAtMs);
  const testsPassed = checks.filter((check) => check.passed).length;
  const lintErrors = estimateDemoLint(code);
  const renderCount = estimateDemoRenderCount(code);
  const execMS = estimateDemoExecMS(code);
  const suspicion = demoSuspicionScore(telemetry);

  return {
    tests_passed: testsPassed,
    tests_total: checks.length,
    lint_errors: lintErrors,
    render_count: renderCount,
    exec_ms: execMS,
    solve_time_seconds: Math.max(18, Math.round((Date.now() - state.currentChallenge.startedAtMs) / 1000)),
    edit_count: telemetry.input_events + telemetry.snapshot_events,
    paste_events: telemetry.paste_events,
    focus_loss_events: telemetry.focus_loss_events,
    snapshot_events: telemetry.snapshot_events,
    first_input_seconds: telemetry.time_to_first_input_seconds,
    similarity_score: isFinal ? round(Math.max(0.08, telemetry.paste_events * 0.12)) : 0.04,
    suspicion_score: suspicion,
    checks,
    telemetry,
  };
}

function requestDemoHint() {
  const usedHints = state.hints.length + 1;
  const focusArea = elements.hintFocus.value.trim() || titleize(state.currentChallenge.category);
  const question = elements.hintQuestion.value.trim();
  state.hints.push({
    provider: "preview",
    focus_area: focusArea,
    used_hints: usedHints,
    remaining_hints: Math.max(0, 3 - usedHints),
    hint:
      question || focusArea
        ? `Look at ${focusArea.toLowerCase()} first. Verify that the component keeps a stable render path, then inspect the state transition that controls the visible UI.`
        : "Start with the state model and the render path. The strongest submissions remove noise first, then tighten interactions.",
  });
  renderHints();
  showFlash(`Preview hint delivered. ${Math.max(0, 3 - usedHints)} remaining.`, "success");
}

function requestDemoExplanation() {
  const execution = state.latestSubmission.execution;
  const evaluation = state.latestSubmission.evaluation;
  state.latestExplanation = {
    provider: "preview",
    summary: `This demo solution clears ${execution.tests_passed} of ${execution.tests_total} checks and lands at ${formatScore(evaluation.final_score)} overall.`,
    strengths: [
      "The challenge keeps a readable component structure for recruiter review.",
      "The current version preserves the main user path instead of optimizing too early.",
    ],
    improvements: [
      execution.lint_errors > 0 ? "Clean up debugging residue and tighten type clarity." : "Push memoization and extraction slightly further for stronger polish.",
      "Treat edge states as first-class UI output rather than implicit behavior.",
    ],
    suspicion_notes:
      state.latestSubmission.anti_cheat.level === "low"
        ? ["Telemetry looks organic in preview mode."]
        : ["Paste-heavy behavior increased the preview suspicion score."],
    recommended_next: "Run another preview after refining the interaction path, then submit once the render and state story are both clean.",
  };
  renderExplanation();
  showFlash("Preview explanation generated.", "success");
}

function demoMutationPreview(templateID, seed) {
  const template = demoTemplates().find((item) => item.id === templateID) || demoTemplates()[0];
  const normalizedSeed = seed || 1729;
  return {
    provider: "preview",
    seed: normalizedSeed,
    title: `${template.title} · Variant ${normalizedSeed}`,
    description_md: `${template.description_md} In this variant, dataset labels and surface wording are shifted while the core skill signal stays the same.`,
    variable_renames: {
      query: normalizedSeed % 2 === 0 ? "needle" : "searchTerm",
      candidates: normalizedSeed % 3 === 0 ? "profiles" : "results",
      filtered: normalizedSeed % 5 === 0 ? "visibleRows" : "matches",
    },
    reviewer_notes: [
      "Surface language changes while preserving the same underlying reasoning path.",
      "Visible tests still inspect correctness, quality, speed, and consistency.",
    ],
  };
}

function renderMutationPreview(preview) {
  elements.mutationPreview.innerHTML = `
    <strong>${escapeHtml(preview.title)}</strong>
    <br /><br />
    <div>${escapeHtml(preview.description_md)}</div>
    <br />
    <strong>Variable renames</strong>
    <div>${Object.entries(preview.variable_renames || {})
      .map(([key, value]) => `${escapeHtml(key)} -> ${escapeHtml(String(value))}`)
      .join("<br />")}</div>
    <br />
    <strong>Reviewer notes</strong>
    <ul>${(preview.reviewer_notes || []).map((item) => `<li>${escapeHtml(item)}</li>`).join("")}</ul>
  `;
}

function demoCheckResults(templateID, code) {
  const lower = code.toLowerCase();
  switch (templateID) {
    case "react_feature_search":
      return [
        { name: "Keeps local query state", passed: lower.includes("usestate(") || lower.includes("usereducer(") },
        { name: "Filters the candidates list", passed: lower.includes(".filter(") },
        { name: "Renders an empty state", passed: lower.includes("no results") || lower.includes("empty") },
        { name: "Handles input changes", passed: lower.includes("onchange") && lower.includes("setquery") },
        { name: "Stabilizes derived filtering", passed: lower.includes("usememo(") || lower.includes("usedeferredvalue(") },
      ];
    case "react_performance_rerenders":
      return [
        { name: "Memoizes row rendering", passed: lower.includes("memo(") || lower.includes("react.memo") },
        { name: "Stabilizes callbacks", passed: lower.includes("usecallback(") },
        { name: "Memoizes derived rows", passed: lower.includes("usememo(") || lower.includes("usedeferredvalue(") },
        { name: "Avoids noisy inline handlers", passed: !lower.includes("onclick={()") && !lower.includes("onselect={()") },
      ];
    case "react_debug_effect_loop":
      return [
        { name: "Stabilizes the effect dependency", passed: lower.includes("usememo(") || lower.includes("useref(") },
        { name: "Keeps trimming logic", passed: lower.includes(".trim(") || lower.includes(".trim()") },
        { name: "Preserves the effect path", passed: lower.includes("useeffect(") },
        { name: "Avoids inline object recreation", passed: !lower.includes("const filters = { query:") },
      ];
    case "react_logic_custom_hook":
      return [
        { name: "Exports a custom hook", passed: lower.includes("export function use") || lower.includes("export const use") },
        { name: "Tracks async state", passed: lower.includes("loading") || lower.includes("error") },
        { name: "Returns a stable API", passed: lower.includes("return {") || lower.includes("return [") },
        { name: "Updates internal state", passed: lower.includes("usestate(") || lower.includes("usereducer(") },
      ];
    default:
      return [
        { name: "Preserves the render path", passed: lower.includes("return") },
        { name: "Removes placeholder TODOs", passed: !lower.includes("todo") },
        { name: "Introduces structure", passed: lower.includes("function") || lower.includes("const ") },
      ];
  }
}

function summarizeDemoTelemetry(events, startedAtMs) {
  let firstInputSeconds = -1;
  const summary = {
    time_to_first_input_seconds: 0,
    input_events: 0,
    paste_events: 0,
    focus_loss_events: 0,
    snapshot_events: 0,
  };

  events.forEach((event) => {
    if (event.event_type === "input") {
      summary.input_events += 1;
      if (firstInputSeconds < 0) {
        firstInputSeconds = Math.max(0, event.offset_seconds || 0);
      }
    }
    if (event.event_type === "paste") {
      summary.paste_events += 1;
      if (firstInputSeconds < 0) {
        firstInputSeconds = Math.max(0, event.offset_seconds || 0);
      }
    }
    if (event.event_type === "focus_lost") {
      summary.focus_loss_events += 1;
    }
    if (event.event_type === "snapshot") {
      summary.snapshot_events += 1;
      if (firstInputSeconds < 0) {
        firstInputSeconds = Math.max(0, event.offset_seconds || 0);
      }
    }
  });

  if (firstInputSeconds < 0) {
    firstInputSeconds = Math.max(0, Math.round((Date.now() - startedAtMs) / 1000));
  }
  summary.time_to_first_input_seconds = firstInputSeconds;
  return summary;
}

function estimateDemoLint(code) {
  const lower = code.toLowerCase();
  let count = 0;
  ["todo", "console.log", "debugger", "@ts-ignore"].forEach((marker) => {
    count += lower.includes(marker) ? 1 : 0;
  });
  return count;
}

function estimateDemoRenderCount(code) {
  const lower = code.toLowerCase();
  let count = 5;
  if (lower.includes("usememo(") || lower.includes("memo(")) {
    count -= 1;
  }
  if (lower.includes("usecallback(")) {
    count -= 1;
  }
  return Math.max(2, count);
}

function estimateDemoExecMS(code) {
  const lower = code.toLowerCase();
  let value = 4;
  if (lower.includes("usememo(") || lower.includes("usedeferredvalue(")) {
    value -= 1;
  }
  if (lower.includes("setinterval(") && !lower.includes("clearinterval")) {
    value += 2;
  }
  return Math.max(1, value);
}

function demoSuspicionScore(telemetry) {
  let score = 0;
  if (telemetry.paste_events >= 1) {
    score += 18;
  }
  if (telemetry.focus_loss_events >= 2) {
    score += 10;
  }
  if (telemetry.time_to_first_input_seconds <= 10) {
    score += 12;
  }
  return score;
}

function demoAntiCheat(execution) {
  const score = demoSuspicionScore(execution.telemetry);
  const level = score >= 45 ? "high" : score >= 20 ? "medium" : "low";
  return {
    level,
    score,
    similarity_score: execution.similarity_score,
    signals: execution.telemetry,
  };
}

function updateDemoSkills(finalScore) {
  state.skills = state.skills.map((skill) => ({
    ...skill,
    score: round(skill.score + (skill.skill_code === "react" ? finalScore * 0.04 : finalScore * 0.02)),
  }));
}

function updateDemoRoom() {
  state.room = state.room.map((item) => ({
    ...item,
    state_json: {
      ...(item.state_json || {}),
      glow: item.room_item_code === "monitor" || item.room_item_code === "trophy_case",
    },
  }));
}

function syncDemoRanking() {
  const updated = demoRankings().filter((entry) => entry.user_id !== state.me.id);
  updated.push({
    user_id: state.me.id,
    username: state.me.username,
    country: state.me.country,
    current_skill_score: state.profile.current_skill_score,
    confidence_score: state.profile.confidence_score,
    percentile_global: state.profile.percentile_global,
  });
  state.rankings = rankDemoEntries(updated);
}

function rankDemoEntries(entries) {
  return entries
    .slice()
    .sort((left, right) => right.current_skill_score - left.current_skill_score)
    .map((entry, index) => ({
      ...entry,
      rank: index + 1,
    }));
}

function cloneData(value) {
  return JSON.parse(JSON.stringify(value));
}

function onVisibilityChange() {
  if (!state.currentChallenge) {
    return;
  }
  if (document.hidden) {
    sendTelemetry("focus_lost", {});
  } else {
    sendTelemetry("focus_gained", {});
  }
}

function setScreen(screen) {
  document.querySelectorAll(".rail-link").forEach((button) => {
    button.classList.toggle("active", button.dataset.screen === screen);
  });
  document.querySelectorAll(".screen").forEach((section) => {
    section.classList.toggle("active", section.id === `screen-${screen}`);
  });
}

function showWorkspace() {
  document.body.classList.remove("auth-mode");
  elements.authPanel.classList.add("hidden");
  elements.workspace.classList.remove("hidden");
}

function showAuth() {
  document.body.classList.add("auth-mode");
  elements.workspace.classList.add("hidden");
  elements.authPanel.classList.remove("hidden");
  setScreen("overview");
  setAuthView(state.authView || "register");
  setRegion(state.selectedRegion || "americas");
}

function applyRoleVisibility() {
  const isHR = state.demoMode || (state.me && (state.me.role === "hr" || state.me.role === "admin"));
  elements.hrNavLink.classList.toggle("hidden", !isHR);
  document.getElementById("screen-hr").classList.toggle("hidden", !isHR);
}

function updateSessionBadge() {
  if (state.demoMode) {
    elements.sessionLabel.textContent = "Preview mode";
    elements.logoutButton.textContent = "Exit preview";
    elements.logoutButton.hidden = false;
    return;
  }
  if (!state.session.accessToken || !state.me) {
    elements.sessionLabel.textContent = "Signed out";
    elements.logoutButton.textContent = "Log out";
    elements.logoutButton.hidden = true;
    return;
  }
  elements.sessionLabel.textContent = `${state.me.username} · ${state.me.role}`;
  elements.logoutButton.textContent = "Log out";
  elements.logoutButton.hidden = false;
}

function setAuthView(view) {
  state.authView = view === "login" ? "login" : "register";
  document.querySelectorAll(".auth-tab").forEach((button) => {
    button.classList.toggle("active", button.dataset.authView === state.authView);
  });
  document.getElementById("register-form").classList.toggle("hidden", state.authView !== "register");
  document.getElementById("login-form").classList.toggle("hidden", state.authView !== "login");

  const copy =
    state.authView === "login"
      ? {
          title: "Welcome back",
          body: "Return to your room, rankings, challenge history, and recruiter workflows.",
        }
      : {
          title: "Create an account",
          body: "Build your room, prove your React skills, and unlock recruiter review.",
        };
  elements.authTitle.textContent = copy.title;
  elements.authCopy.textContent = copy.body;
}

function setRegion(region) {
  const config = regionConfig(region) || regionConfig("americas");
  state.selectedRegion = config.key;

  document.querySelectorAll(".auth-region-button").forEach((button) => {
    button.classList.toggle("active", button.dataset.region === config.key);
  });
  elements.authScene.dataset.region = config.key;
  elements.authSceneBadge.textContent = config.sceneLabel;
  elements.authRegionCopy.textContent = config.description;

  const countryInput = document.getElementById("register-country");
  if (countryInput && (!countryInput.value || countryInput.dataset.autofill === "true")) {
    countryInput.value = config.country;
    countryInput.dataset.autofill = "true";
  }
}

async function api(path, options = {}) {
  const method = options.method || "GET";
  const headers = new Headers(options.headers || {});
  if (options.body !== undefined) {
    headers.set("Content-Type", "application/json");
  }
  if (options.auth !== false && state.session.accessToken) {
    headers.set("Authorization", `Bearer ${state.session.accessToken}`);
  }

  const response = await fetch(path, {
    method,
    headers,
    body: options.body !== undefined ? JSON.stringify(options.body) : undefined,
  });

  if (response.status === 401 && options.auth !== false && state.session.refreshToken && options.retry !== false) {
    await refreshAccessToken();
    return api(path, { ...options, retry: false });
  }

  const contentType = response.headers.get("Content-Type") || "";
  const payload = contentType.includes("application/json") ? await response.json() : await response.text();
  if (!response.ok) {
    const message = typeof payload === "string" ? payload : payload.error || "Request failed";
    throw new Error(message);
  }
  return payload;
}

async function refreshAccessToken() {
  const auth = await api("/v1/auth/refresh", {
    method: "POST",
    body: { refresh_token: state.session.refreshToken },
    auth: false,
    retry: false,
  });
  setSession(auth);
}

function setSession(auth) {
  state.demoMode = false;
  state.session = {
    accessToken: auth.access_token,
    refreshToken: auth.refresh_token,
    expiresAt: auth.expires_at,
  };
  state.me = auth.user;
  persistSession();
  updateSessionBadge();
}

function clearSession() {
  if (state.chatPollTimer) {
    clearInterval(state.chatPollTimer);
  }
  state.demoMode = false;
  state.session = { accessToken: "", refreshToken: "", expiresAt: "" };
  state.me = null;
  state.profile = null;
  state.templates = [];
  state.rankings = [];
  state.skills = [];
  state.room = [];
  state.latestRun = null;
  state.currentChallenge = null;
  state.latestSubmission = null;
  state.latestExplanation = null;
  state.hints = [];
  state.chatMessages = [];
  state.candidates = [];
  localStorage.removeItem("signal-room-session");
  updateSessionBadge();
}

function persistSession() {
  localStorage.setItem("signal-room-session", JSON.stringify(state.session));
}

function loadSession() {
  try {
    return JSON.parse(localStorage.getItem("signal-room-session")) || { accessToken: "", refreshToken: "", expiresAt: "" };
  } catch (_) {
    return { accessToken: "", refreshToken: "", expiresAt: "" };
  }
}

function showFlash(message, type) {
  clearTimeout(state.flashTimer);
  elements.flash.className = `notice visible ${type}`;
  elements.flash.textContent = message;
  state.flashTimer = window.setTimeout(() => {
    elements.flash.className = "notice";
    elements.flash.textContent = "";
  }, 4200);
}

function formatScore(value) {
  const number = Number(value || 0);
  return number.toFixed(2).replace(/\.00$/, "");
}

function round(value) {
  return Math.round(Number(value || 0) * 100) / 100;
}

function clamp(value, min, max) {
  return Math.min(max, Math.max(min, value));
}

function titleize(value) {
  return String(value || "")
    .replaceAll("_", " ")
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

function toCamel(value) {
  return value.replace(/-([a-z])/g, (_, char) => char.toUpperCase());
}

function escapeHtml(value) {
  return String(value || "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}
