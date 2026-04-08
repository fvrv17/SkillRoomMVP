"use client";

import { forwardRef, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { apiFetch, clearAuth, loadAuth, loadRegionId } from "@/lib/client";
import { REGIONS } from "@/lib/preview-data";
import {
  ROOM_DEFAULT_THEME_ID,
  ROOM_DEFAULT_WINDOW_SCENE_ID,
  ROOM_SCENE_ORDER,
  ROOM_SCENE_SLOTS,
  ROOM_THEMES,
  ROOM_WINDOW_SCENES,
  normalizeRoomLevel,
  normalizeRoomTheme,
  normalizeWindowScene,
} from "@/lib/room-scene-config";

const NAV_ITEMS = [
  { id: "overview", label: "Overview" },
  { id: "challenges", label: "Challenges" },
  { id: "room", label: "Room" },
  { id: "ranking", label: "Ranking" },
  { id: "hr", label: "HR view" },
];

export default function WorkspaceClient() {
  const router = useRouter();
  const telemetryRef = useRef({ startedAtMs: 0, inputCount: 0, firstInputSent: false });
  const editorPreviewRef = useRef(null);
  const [booting, setBooting] = useState(true);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [session, setSession] = useState(null);
  const [activeView, setActiveView] = useState("overview");
  const [filters, setFilters] = useState({ minScore: "600", minConfidence: "70", activeDays: "14" });
  const [data, setData] = useState({
    user: null,
    profile: null,
    skills: [],
    room: [],
    rankings: [],
    templates: [],
    candidates: [],
  });
  const [currentChallenge, setCurrentChallenge] = useState(null);
  const [activeFile, setActiveFile] = useState("");
  const [runResult, setRunResult] = useState(null);
  const [submissionResult, setSubmissionResult] = useState(null);
  const [hintResult, setHintResult] = useState(null);
  const [explanationResult, setExplanationResult] = useState(null);
  const [busyAction, setBusyAction] = useState("");
  const [selectedRoomCode, setSelectedRoomCode] = useState("");
  const roomThemeID = ROOM_DEFAULT_THEME_ID;
  const windowSceneID = ROOM_DEFAULT_WINDOW_SCENE_ID;

  useEffect(() => {
    const auth = loadAuth();
    const region = REGIONS.find((item) => item.id === loadRegionId()) || REGIONS[0];
    const nextSession = { auth, region };
    setSession(nextSession);
    setActiveView(defaultView(auth?.user?.role));
    setBooting(false);
  }, []);

  useEffect(() => {
    if (booting || !session) {
      return;
    }
    loadWorkspace(session);
  }, [booting, session]);

  const roomItems = data.room || [];
  const skills = data.skills || [];
  const profile = data.profile || {};
  const user = data.user || {};
  const templates = data.templates || [];
  const rankings = data.rankings || [];
  const candidates = data.candidates || [];
  const token = session?.auth?.access_token || "";
  const allowedNav = NAV_ITEMS.filter((item) => user.role === "hr" || item.id !== "hr");
  const editableFiles = currentChallenge?.editableFiles || [];
  const editorFiles = currentChallenge?.files || {};
  const visibleTests = currentChallenge?.visibleTests || {};
  const editorContent = activeFile ? editorFiles[activeFile] || "" : "";
  const editorLanguage = activeFile.endsWith(".js") ? "js" : "jsx";
  const orderedRoomItems = useMemo(
    () =>
      ROOM_SCENE_ORDER.map((code) => roomItems.find((item) => item.room_item_code === code)).filter(Boolean),
    [roomItems],
  );
  const selectedRoomItem = orderedRoomItems.find((item) => item.room_item_code === selectedRoomCode) || orderedRoomItems[0] || null;
  const selectedRoomSlot = selectedRoomItem ? ROOM_SCENE_SLOTS[selectedRoomItem.room_item_code] : null;

  useEffect(() => {
    if (orderedRoomItems.length === 0) {
      if (selectedRoomCode) {
        setSelectedRoomCode("");
      }
      return;
    }
    if (!orderedRoomItems.some((item) => item.room_item_code === selectedRoomCode)) {
      setSelectedRoomCode(orderedRoomItems[0].room_item_code);
    }
  }, [orderedRoomItems, selectedRoomCode]);

  async function loadWorkspace(currentSession) {
    setLoading(true);
    setError("");

    try {
      if (!currentSession.auth?.access_token) {
        return;
      }

      const [me, profilePayload, skillsPayload, roomPayload, templatesPayload, rankingPayload] = await Promise.all([
        apiFetch("/v1/me", { token: currentSession.auth.access_token }),
        apiFetch("/v1/profile", { token: currentSession.auth.access_token }),
        apiFetch("/v1/skills", { token: currentSession.auth.access_token }),
        apiFetch("/v1/room", { token: currentSession.auth.access_token }),
        apiFetch("/v1/challenges/templates", { token: currentSession.auth.access_token }),
        apiFetch("/v1/rankings/global", { token: currentSession.auth.access_token }),
      ]);

      let candidatePayload = { candidates: [] };
      if (me.role === "hr") {
        candidatePayload = await fetchCandidates(currentSession.auth.access_token, filters);
      }

      setData({
        user: me,
        profile: profilePayload,
        skills: skillsPayload.skills || [],
        room: roomPayload.items || [],
        rankings: rankingPayload.rankings || [],
        templates: templatesPayload.templates || [],
        candidates: candidatePayload.candidates || [],
      });
      setActiveView(defaultView(me.role));
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "Unable to load workspace");
      clearAuth();
    } finally {
      setLoading(false);
    }
  }

  async function refreshAfterSubmission(profilePayload, skillsPayload, roomPayload) {
    if (!token) {
      return;
    }
    const rankingPayload = await apiFetch("/v1/rankings/global", { token });
    let nextCandidates = data.candidates;
    if (user.role === "hr") {
      const candidatePayload = await fetchCandidates(token, filters);
      nextCandidates = candidatePayload.candidates || [];
    }
    setData((current) => ({
      ...current,
      profile: profilePayload,
      skills: skillsPayload,
      room: roomPayload,
      rankings: rankingPayload.rankings || [],
      candidates: nextCandidates,
    }));
  }

  async function handleApplyFilters() {
    if (!token || user.role !== "hr") {
      return;
    }

    setBusyAction("filters");
    setError("");
    try {
      const candidatePayload = await fetchCandidates(token, filters);
      setData((current) => ({ ...current, candidates: candidatePayload.candidates || [] }));
    } catch (candidateError) {
      setError(candidateError instanceof Error ? candidateError.message : "Unable to load candidates");
    } finally {
      setBusyAction("");
    }
  }

  async function handleStartChallenge(templateID) {
    setError("");
    setRunResult(null);
    setSubmissionResult(null);
    setHintResult(null);
    setExplanationResult(null);

    try {
      const view = await apiFetch("/v1/challenges/instances", {
        method: "POST",
        token,
        body: { template_id: templateID },
      });
      const nextChallenge = normalizeChallenge(view);
      setCurrentChallenge(nextChallenge);
      setActiveFile(nextChallenge.editableFiles[0] || "");
      setActiveView("challenges");
      telemetryRef.current = { startedAtMs: Date.now(), inputCount: 0, firstInputSent: false };
    } catch (challengeError) {
      setError(challengeError instanceof Error ? challengeError.message : "Unable to start challenge");
    }
  }

  async function recordTelemetry(eventType, payload = {}) {
    if (!token || !currentChallenge?.instanceId) {
      return;
    }
    const offsetSeconds = Math.max(0, Math.round((Date.now() - telemetryRef.current.startedAtMs) / 1000));
    try {
      await apiFetch(`/v1/challenges/instances/${currentChallenge.instanceId}/telemetry`, {
        method: "POST",
        token,
        body: {
          event_type: eventType,
          offset_seconds: offsetSeconds,
          payload,
        },
      });
    } catch {
      // Telemetry must not block the solving flow.
    }
  }

  function updateEditorContent(nextValue) {
    if (!currentChallenge || !activeFile) {
      return;
    }
    setCurrentChallenge((current) => ({
      ...current,
      files: {
        ...current.files,
        [activeFile]: nextValue,
      },
    }));

    telemetryRef.current.inputCount += 1;
    if (!telemetryRef.current.firstInputSent) {
      telemetryRef.current.firstInputSent = true;
      void recordTelemetry("input", { file: activeFile, chars: nextValue.length });
      return;
    }
    if (telemetryRef.current.inputCount <= 5 || telemetryRef.current.inputCount % 12 === 0) {
      void recordTelemetry("input", { file: activeFile, chars: nextValue.length });
    }
  }

  function handleEditorScroll(event) {
    if (!editorPreviewRef.current) {
      return;
    }
    editorPreviewRef.current.scrollTop = event.target.scrollTop;
    editorPreviewRef.current.scrollLeft = event.target.scrollLeft;
  }

  async function handleRunChecks() {
    if (!currentChallenge) {
      return;
    }

    setBusyAction("run");
    setError("");
    setRunResult(null);

    try {
      await recordTelemetry("snapshot", { files: editableFiles.length });
      const payload = await apiFetch(`/v1/challenges/instances/${currentChallenge.instanceId}/runs`, {
        method: "POST",
        token,
        body: {
          language: "jsx",
          source_files: editableSourceFiles(currentChallenge),
        },
      });
      setRunResult(payload);
    } catch (runError) {
      setError(runError instanceof Error ? runError.message : "Runner execution failed");
    } finally {
      setBusyAction("");
    }
  }

  async function handleSubmit() {
    if (!currentChallenge) {
      return;
    }

    setBusyAction("submit");
    setError("");

    try {
      await recordTelemetry("snapshot", { files: editableFiles.length, action: "submit" });
      const payload = await apiFetch(`/v1/challenges/instances/${currentChallenge.instanceId}/submissions`, {
        method: "POST",
        token,
        body: {
          language: "jsx",
          source_files: editableSourceFiles(currentChallenge),
        },
      });
      setSubmissionResult(payload);
      setRunResult({
        execution: payload.execution,
        telemetry: payload.telemetry,
      });
      await refreshAfterSubmission(payload.profile, payload.skills, payload.room);
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : "Submission failed");
    } finally {
      setBusyAction("");
    }
  }

  async function handleHint() {
    if (!currentChallenge) {
      return;
    }

    setBusyAction("hint");
    setError("");
    try {
      const payload = await apiFetch(`/v1/ai/challenges/${currentChallenge.instanceId}/hint`, {
        method: "POST",
        token,
        body: {},
      });
      setHintResult(payload);
    } catch (hintError) {
      setError(hintError instanceof Error ? hintError.message : "Unable to fetch hint");
    } finally {
      setBusyAction("");
    }
  }

  async function handleExplain() {
    if (!currentChallenge || !submissionResult) {
      return;
    }

    setBusyAction("explain");
    setError("");
    try {
      const payload = await apiFetch(`/v1/ai/challenges/${currentChallenge.instanceId}/explain`, {
        method: "POST",
        token,
        body: { submission_id: submissionResult.submission?.id },
      });
      setExplanationResult(payload);
    } catch (explainError) {
      setError(explainError instanceof Error ? explainError.message : "Unable to explain evaluation");
    } finally {
      setBusyAction("");
    }
  }

  function handleSignOut() {
    clearAuth();
    router.push("/");
  }

  if (booting) {
    return <main className="workspace-shell"><section className="empty-state"><h1>Loading SkillRoom...</h1></section></main>;
  }

  if (!session?.auth) {
    return (
      <main className="workspace-shell">
        <section className="empty-state">
          <h1>No active workspace</h1>
          <p>Start from the launch scene to create an account or sign in.</p>
          <Link href="/" className="primary-button">
            Back to launch
          </Link>
        </section>
      </main>
    );
  }

  return (
    <main className="workspace-shell">
      <header className="workspace-header">
        <div>
          <div className="brand-chip">SkillRoom</div>
          <p className="workspace-subtitle">
            {`${user.username || "Workspace"} • ${user.role === "hr" ? "Recruiter" : "Candidate"}`}
          </p>
        </div>
        <div className="workspace-meta">
          <span className="meta-pill">{session?.region?.label || "Americas"}</span>
          <span className="meta-pill">{`Confidence ${formatNumber(profile.confidence_score)}`}</span>
          <button type="button" className="secondary-button" onClick={handleSignOut}>
            Sign out
          </button>
        </div>
      </header>

      <nav className="workspace-nav">
        {allowedNav.map((item) => (
          <button
            key={item.id}
            type="button"
            className={activeView === item.id ? "nav-button active" : "nav-button"}
            onClick={() => setActiveView(item.id)}
          >
            {item.label}
          </button>
        ))}
      </nav>

      {error ? <p className="inline-error workspace-error">{error}</p> : null}

      {activeView === "overview" ? (
        <section className="workspace-grid workspace-grid--overview">
          <div className="card stats-card">
            <p className="eyebrow">Skill score</p>
            <h2>{formatNumber(profile.current_skill_score)}</h2>
            <div className="stats-row">
              <Stat label="Confidence" value={formatNumber(profile.confidence_score)} />
              <Stat label="Global %" value={formatNumber(profile.percentile_global)} />
              <Stat label="Solved" value={profile.completed_challenges || 0} />
            </div>
          </div>
          <div className="card">
            <p className="eyebrow">Skill distribution</p>
            <div className="skill-list">
              {skills.map((skill) => (
                <div key={skill.skill_code} className="skill-item">
                  <div className="skill-row">
                    <div className="skill-meta">
                      <span className="skill-name">{labelize(skill.skill_code)}</span>
                      <span className={`skill-level ${levelClass(skill.level)}`}>{skill.level}</span>
                    </div>
                    <strong>{formatNumber(skill.score)}</strong>
                  </div>
                </div>
              ))}
            </div>
          </div>
          <div className="card card--span-2">
            <div className="section-heading">
              <div>
                <p className="eyebrow">Room state</p>
                <h3>Items move with real skill data</h3>
              </div>
              <button type="button" className="secondary-button" onClick={() => setActiveView("room")}>
                Open room
              </button>
            </div>
            <RoomStage items={roomItems} compact themeID={roomThemeID} windowSceneID={windowSceneID} />
          </div>
        </section>
      ) : null}

      {activeView === "challenges" ? (
        <section className="workspace-grid workspace-grid--challenges">
          <div className="card template-panel">
            <div className="section-heading">
              <div>
                <p className="eyebrow">Task bank</p>
                <h3>Runner-compatible React challenges</h3>
              </div>
            </div>
            <div className="template-list">
              {templates.map((template) => (
                <button
                  key={template.id}
                  type="button"
                  className={currentChallenge?.templateId === template.id ? "template-card active" : "template-card"}
                  onClick={() => handleStartChallenge(template.id)}
                >
                  <div className="template-card__header">
                    <span>{labelize(template.category)}</span>
                    <strong>D{template.difficulty}</strong>
                  </div>
                  <h4>{template.title}</h4>
                  <p>{excerpt(template.description_md || template.description)}</p>
                </button>
              ))}
            </div>
          </div>

          <div className="card challenge-panel">
            {currentChallenge ? (
              <>
                <div className="section-heading">
                  <div>
                    <p className="eyebrow">Active attempt</p>
                    <h3>{currentChallenge.title}</h3>
                  </div>
                  <span className="meta-pill">
                    {labelize(currentChallenge.category)} • Attempt {currentChallenge.attemptNumber}
                  </span>
                </div>

                <p className="challenge-description">{currentChallenge.description}</p>

                {editableFiles.length > 1 ? (
                  <div className="file-tabs">
                    {editableFiles.map((file) => (
                      <button
                        key={file}
                        type="button"
                        className={file === activeFile ? "file-tab active" : "file-tab"}
                        onClick={() => setActiveFile(file)}
                      >
                        {fileName(file)}
                      </button>
                    ))}
                  </div>
                ) : (
                  <div className="file-tabs">
                    <div className="file-label">{fileName(activeFile)}</div>
                  </div>
                )}

                <div className="editor">
                  <div className="editor-toolbar">
                    <strong>{fileName(activeFile)}</strong>
                    <span>{editorLanguage.toUpperCase()} editor</span>
                  </div>
                  <div className="editor-canvas">
                    <HighlightedCode
                      ref={editorPreviewRef}
                      code={editorContent}
                      language={editorLanguage}
                      className="editor-preview"
                    />
                    <textarea
                      className="code-editor"
                      value={editorContent}
                      onChange={(event) => updateEditorContent(event.target.value)}
                      onPaste={(event) => {
                        void recordTelemetry("paste", {
                          file: activeFile,
                          chars: event.clipboardData.getData("text").length,
                        });
                      }}
                      onScroll={handleEditorScroll}
                      spellCheck="false"
                      wrap="off"
                    />
                  </div>
                </div>

                <div className="action-row">
                  <button type="button" className="secondary-button" onClick={handleHint} disabled={busyAction !== ""}>
                    {busyAction === "hint" ? "Loading hint..." : "Get hint"}
                  </button>
                  <button type="button" className="secondary-button" onClick={handleRunChecks} disabled={busyAction !== ""}>
                    {busyAction === "run" ? "Running..." : "Run checks"}
                  </button>
                  <button type="button" className="primary-button" onClick={handleSubmit} disabled={busyAction !== ""}>
                    {busyAction === "submit" ? "Submitting..." : "Submit solution"}
                  </button>
                </div>

                <div className="challenge-results">
                  <div className="result-card">
                    <p className="eyebrow">Visible tests</p>
                    {Object.entries(visibleTests).map(([file, content]) => (
                      <div key={file} className="test-block">
                        <strong>{file}</strong>
                        <HighlightedCode code={content} language={file.endsWith(".js") ? "js" : "jsx"} />
                      </div>
                    ))}
                  </div>

                  <div className="result-card">
                    <p className="eyebrow">Runner output</p>
                    {runResult?.execution ? (
                      <ExecutionSummary execution={runResult.execution} />
                    ) : (
                      <p className="muted-copy">Run checks to execute the visible and hidden test suite in the runner.</p>
                    )}
                  </div>

                  <div className="result-card">
                    <p className="eyebrow">Scoring</p>
                    {submissionResult?.evaluation ? (
                      <EvaluationSummary evaluation={submissionResult.evaluation} antiCheat={submissionResult.anti_cheat} />
                    ) : (
                      <p className="muted-copy">Submission scoring appears here after the backend computes correctness, quality, speed, and consistency.</p>
                    )}
                  </div>

                  <div className="result-card">
                    <div className="section-heading">
                      <div>
                        <p className="eyebrow">AI explanation</p>
                        <h4>Why the score moved</h4>
                      </div>
                      <button
                        type="button"
                        className="secondary-button"
                        onClick={handleExplain}
                        disabled={!submissionResult || busyAction !== ""}
                      >
                        {busyAction === "explain" ? "Explaining..." : "Explain score"}
                      </button>
                    </div>
                    {hintResult ? (
                      <div className="ai-block">
                        <strong>Hint</strong>
                        <p>{hintResult.hint}</p>
                      </div>
                    ) : null}
                    {explanationResult ? (
                      <div className="ai-block">
                        <p>{explanationResult.summary}</p>
                        <ul className="plain-list">
                          {(explanationResult.strengths || []).map((item) => (
                            <li key={item}>Strength: {item}</li>
                          ))}
                          {(explanationResult.improvements || []).map((item) => (
                            <li key={item}>Improve: {item}</li>
                          ))}
                        </ul>
                      </div>
                    ) : (
                      <p className="muted-copy">Use hints during the attempt or request an explanation after submission.</p>
                    )}
                  </div>
                </div>
              </>
            ) : (
              <div className="empty-panel">
                <h3>Select a challenge template</h3>
                <p>Start from the task bank on the left. Deterministic variants are generated per user and attempt.</p>
              </div>
            )}
          </div>
        </section>
      ) : null}

      {activeView === "room" ? (
        <section className="workspace-grid workspace-grid--room">
          <div className="card room-scene-card">
            <div className="section-heading">
              <div>
                <p className="eyebrow">Skill room</p>
                <h3>Every item is mapped to a real signal</h3>
              </div>
            </div>
            <RoomStage
              items={roomItems}
              selectedCode={selectedRoomCode}
              onSelect={setSelectedRoomCode}
              themeID={roomThemeID}
              windowSceneID={windowSceneID}
            />
          </div>
          <div className="card room-inspector">
            <p className="eyebrow">Item inspector</p>
            <RoomInspector
              item={selectedRoomItem}
              slot={selectedRoomSlot}
              onSelect={setSelectedRoomCode}
              items={orderedRoomItems}
            />
          </div>
        </section>
      ) : null}

      {activeView === "ranking" ? (
        <section className="workspace-grid workspace-grid--ranking">
          <div className="card card--span-2">
            <div className="section-heading">
              <div>
                <p className="eyebrow">Global ranking</p>
                <h3>Score, confidence, and recency</h3>
              </div>
            </div>
            <div className="ranking-table">
              <div className="ranking-row ranking-row--header">
                <span>Rank</span>
                <span>User</span>
                <span>Score</span>
                <span>Confidence</span>
                <span>Percentile</span>
                <span>Solved</span>
              </div>
              {rankings.map((entry) => (
                <div key={`${entry.rank}-${entry.username}`} className="ranking-row">
                  <span>#{entry.rank}</span>
                  <span>{entry.username}</span>
                  <span>{formatNumber(entry.current_skill_score)}</span>
                  <span>{formatNumber(entry.confidence_score)}</span>
                  <span>{formatNumber(entry.percentile)}</span>
                  <span>{entry.completed_challenges || 0}</span>
                </div>
              ))}
            </div>
          </div>
        </section>
      ) : null}

      {activeView === "hr" ? (
        <section className="workspace-grid workspace-grid--hr">
          {user.role === "hr" ? (
            <>
              <div className="card card--span-2">
                <div className="section-heading">
                  <div>
                    <p className="eyebrow">Candidate scan</p>
                    <h3>Evaluate candidates in under 10 seconds</h3>
                  </div>
                </div>
                <div className="filters-row">
                  <label className="form-row">
                    <span>Score &gt;</span>
                    <input
                      type="number"
                      value={filters.minScore}
                      onChange={(event) => setFilters((current) => ({ ...current, minScore: event.target.value }))}
                    />
                  </label>
                  <label className="form-row">
                    <span>Confidence &gt;</span>
                    <input
                      type="number"
                      value={filters.minConfidence}
                      onChange={(event) => setFilters((current) => ({ ...current, minConfidence: event.target.value }))}
                    />
                  </label>
                  <label className="form-row">
                    <span>Active &lt; days</span>
                    <input
                      type="number"
                      value={filters.activeDays}
                      onChange={(event) => setFilters((current) => ({ ...current, activeDays: event.target.value }))}
                    />
                  </label>
                  <button type="button" className="primary-button" onClick={handleApplyFilters} disabled={busyAction === "filters"}>
                    {busyAction === "filters" ? "Applying..." : "Apply filters"}
                  </button>
                </div>
              </div>
              {candidates.map((candidate) => {
                const summary = candidate.summary || candidate;
                return (
                  <div key={candidate.user_id} className="card candidate-card">
                  <div className="candidate-card__header">
                    <div>
                      <h3>{candidate.username}</h3>
                      <p>{candidate.country}</p>
                    </div>
                    <strong>{formatNumber(summary.score ?? candidate.current_skill_score)}</strong>
                  </div>
                  <div className="stats-row">
                    <Stat label="Percentile" value={formatNumber(summary.percentile ?? candidate.percentile_global)} />
                    <Stat label="Confidence" value={`${formatNumber(summary.confidence_score ?? candidate.confidence_score)} / ${String(summary.confidence_level || candidate.confidence_level || "medium").toUpperCase()}`} />
                    <Stat label="Solved" value={summary.tasks_completed ?? candidate.tasks_solved ?? 0} />
                  </div>
                  <p><strong>Strengths:</strong> {(candidate.strengths || []).map(labelize).join(", ") || "N/A"}</p>
                  <p><strong>Weaknesses:</strong> {(candidate.weaknesses || []).map(labelize).join(", ") || "N/A"}</p>
                  <ul className="plain-list">
                    {(candidate.confidence_reasons || []).map((reason) => (
                      <li key={reason}>{reason}</li>
                    ))}
                  </ul>
                  <ul className="plain-list">
                    {(candidate.recent_activity || []).map((item) => (
                      <li key={item}>{item}</li>
                    ))}
                  </ul>
                  </div>
                );
              })}
            </>
          ) : (
            <div className="card empty-panel">
              <h3>Recruiter view is restricted</h3>
              <p>Register as a recruiter to search candidates, review confidence, and shortlist talent.</p>
            </div>
          )}
        </section>
      ) : null}

      {loading ? <div className="loading-bar" /> : null}
    </main>
  );
}

function defaultView(role) {
  if (role === "hr") {
    return "hr";
  }
  return "overview";
}

function normalizeChallenge(view) {
  const generatedFiles = { ...(view.variant?.generated_files || {}) };
  const editableFiles = [...(view.editable_files || view.variant?.editable_files || [])];
  return {
    instanceId: view.instance?.id || view.instance?.ID || "",
    attemptNumber: view.instance?.attempt_number || 1,
    templateId: view.template_id,
    title: view.title,
    description: view.description_md || "",
    category: view.category,
    difficulty: view.difficulty,
    editableFiles,
    visibleTests: { ...(view.visible_tests || {}) },
    files: generatedFiles,
  };
}

function editableSourceFiles(challenge) {
  const files = {};
  for (const file of challenge.editableFiles || []) {
    files[file] = challenge.files[file] || "";
  }
  return files;
}

async function fetchCandidates(token, filters) {
  const query = new URLSearchParams();
  if (filters.minScore) {
    query.set("min_score", filters.minScore);
  }
  if (filters.minConfidence) {
    query.set("min_confidence", filters.minConfidence);
  }
  if (filters.activeDays) {
    query.set("active_days", filters.activeDays);
  }
  return apiFetch(`/v1/hr/candidates?${query.toString()}`, { token });
}

function readState(item) {
  if (!item || typeof item !== "object") {
    return {};
  }
  return item.state_json || item.state || {};
}

function formatNumber(value) {
  if (value === undefined || value === null || Number.isNaN(Number(value))) {
    return "0";
  }
  return Math.round(Number(value)).toString();
}

function excerpt(value) {
  if (!value) {
    return "";
  }
  return String(value).replace(/\s+/g, " ").trim().slice(0, 120);
}

function fileName(value) {
  const normalized = String(value || "").trim();
  if (!normalized) {
    return "untitled";
  }
  const parts = normalized.split("/");
  return parts[parts.length - 1] || normalized;
}

function labelize(value) {
  return String(value || "")
    .replace(/_/g, " ")
    .replace(/\b\w/g, (match) => match.toUpperCase());
}

function roomLabel(code) {
  switch (code) {
    case "monitor":
      return "Monitor / React";
    case "desk":
      return "Desk / JavaScript";
    case "chair":
      return "Chair / Architecture";
    case "plant":
      return "Plant / Consistency";
    case "trophy_case":
      return "Trophy / Achievements";
    case "shelf":
      return "Shelf / Volume";
    default:
      return labelize(code);
  }
}

function roomPresentationMode(slot, item) {
  const state = readState(item);
  return state.presentation_mode || slot?.presentationMode || "tiered";
}

function roomAchievementEntries(item) {
  const raw = readState(item).achievements;
  if (!Array.isArray(raw)) {
    return [];
  }
  return raw
    .map((entry) => {
      if (!entry || typeof entry !== "object") {
        return null;
      }
      return {
        code: String(entry.code || ""),
        title: String(entry.title || entry.code || "Achievement"),
        description: String(entry.description || ""),
      };
    })
    .filter(Boolean);
}

function roomAchievementCount(item) {
  const state = readState(item);
  const numeric = Number(state.achievement_count);
  if (Number.isFinite(numeric) && numeric >= 0) {
    return numeric;
  }
  return roomAchievementEntries(item).length;
}

function roomMetaLabel(slot, item) {
  if (roomPresentationMode(slot, item) === "achievement_case") {
    const count = roomAchievementCount(item);
    if (count === 0) {
      return "No trophies";
    }
    return count === 1 ? "1 trophy" : `${count} trophies`;
  }
  return item?.current_level || "bronze";
}

function levelClass(level) {
  switch (String(level || "").toLowerCase()) {
    case "platinum":
      return "level-platinum";
    case "gold":
      return "level-gold";
    case "silver":
      return "level-silver";
    case "bronze":
      return "level-bronze";
    default:
      return "";
  }
}

function ExecutionSummary({ execution }) {
  const checks = execution.checks || [];
  const qualityChecks = checks.filter((check) => check.kind === "quality");
  return (
    <div className="summary-stack">
      <div className="stats-row">
        <Stat label="Passed" value={execution.tests_passed || 0} />
        <Stat label="Total" value={execution.tests_total || 0} />
        <Stat label="Hidden fail" value={execution.hidden_failed || 0} />
        <Stat label="Cost ms" value={execution.execution_cost_ms || execution.execution_time_ms || 0} />
      </div>
      {qualityChecks.length > 0 ? (
        <div className="stats-row">
          <Stat label="Quality pass" value={execution.quality_passed || 0} />
          <Stat label="Quality fail" value={execution.quality_failed || 0} />
        </div>
      ) : null}
      <ul className="plain-list">
        {checks.map((check) => (
          <li key={`${check.kind || "correctness"}-${check.name}`}>
            {check.passed ? "PASS" : "FAIL"} · {(check.kind || "correctness").toUpperCase()} · {check.name}
          </li>
        ))}
        {(execution.errors || []).map((error) => (
          <li key={error}>{error}</li>
        ))}
      </ul>
    </div>
  );
}

function EvaluationSummary({ evaluation, antiCheat }) {
  const report = evaluation.report_json || {};
  const executionCostScore = evaluation.execution_cost_score ?? evaluation.speed_score;
  return (
    <div className="summary-stack">
      <div className="stats-row">
        <Stat label="Final" value={formatNumber(evaluation.final_score)} />
        <Stat label="Correctness" value={formatNumber(evaluation.test_score)} />
        <Stat label="Quality" value={formatNumber(evaluation.quality_score)} />
        <Stat label="Runtime efficiency" value={formatNumber(executionCostScore)} />
      </div>
      <div className="stats-row">
        <Stat label="Lint quality" value={formatNumber(report.lint_quality ?? evaluation.lint_score)} />
        <Stat label="Task quality" value={formatNumber(report.task_quality ?? 0)} />
        <Stat label="Consistency" value={formatNumber(evaluation.consistency_score)} />
        <Stat label="Suspicion" value={antiCheat?.level || "low"} />
        <Stat label="Signals" value={antiCheat?.score || 0} />
      </div>
      <ul className="plain-list">
        {(antiCheat?.reasons || []).map((reason) => (
          <li key={reason}>{reason}</li>
        ))}
      </ul>
    </div>
  );
}

const HighlightedCode = forwardRef(function HighlightedCode({ code, language = "js", className = "" }, ref) {
  const tokens = useMemo(() => tokenizeCode(code || ""), [code]);
  const blockClassName = className ? `code-block ${className}` : "code-block";
  return (
    <pre ref={ref} className={blockClassName}>
      <code className={`language-${language}`}>
        {tokens.map((token, index) =>
          token.type === "plain" ? (
            <span key={index}>{token.value}</span>
          ) : (
            <span key={index} className={`token ${token.type}`}>
              {token.value}
            </span>
          ),
        )}
      </code>
    </pre>
  );
});

const KEYWORD_PATTERN = /\b(?:import|from|export|default|function|return|const|let|var|if|else|for|while|switch|case|break|continue|try|catch|throw|async|await|new|class|extends|null|true|false)\b/;
const TOKEN_PATTERN = /\/\*[\s\S]*?\*\/|\/\/[^\n]*|"(?:\\.|[^"])*"|'(?:\\.|[^'])*'|`(?:\\.|[^`])*`|\b(?:import|from|export|default|function|return|const|let|var|if|else|for|while|switch|case|break|continue|try|catch|throw|async|await|new|class|extends|null|true|false)\b|\b[A-Za-z_$][\w$]*(?=\s*\()/g;

function tokenizeCode(code) {
  const tokens = [];
  let cursor = 0;

  for (const match of code.matchAll(TOKEN_PATTERN)) {
    const value = match[0];
    const index = match.index ?? 0;
    if (index > cursor) {
      tokens.push({ type: "plain", value: code.slice(cursor, index) });
    }
    tokens.push({ type: classifyToken(value), value });
    cursor = index + value.length;
  }

  if (cursor < code.length) {
    tokens.push({ type: "plain", value: code.slice(cursor) });
  }

  return tokens;
}

function classifyToken(value) {
  if (value.startsWith("//") || value.startsWith("/*")) {
    return "comment";
  }
  if (value.startsWith("\"") || value.startsWith("'") || value.startsWith("`")) {
    return "string";
  }
  if (KEYWORD_PATTERN.test(value)) {
    return "keyword";
  }
  return "function";
}

function RoomStage({ items, compact = false, selectedCode = "", onSelect, themeID = ROOM_DEFAULT_THEME_ID, windowSceneID = ROOM_DEFAULT_WINDOW_SCENE_ID }) {
  const indexed = {};
  for (const item of items || []) {
    indexed[item.room_item_code] = item;
  }
  const interactive = typeof onSelect === "function" && !compact;
  const theme = ROOM_THEMES[normalizeRoomTheme(themeID)];
  const windowScene = ROOM_WINDOW_SCENES[normalizeWindowScene(windowSceneID)];

  return (
    <div className={compact ? `room-stage room-stage--compact room-stage--${theme.id}` : `room-stage room-stage--${theme.id}`}>
      <div className="room-stage__canvas">
        <img className="room-stage__window-scene" src={windowScene.asset} alt="" aria-hidden="true" />
        <img className="room-stage__shell" src={theme.shellAsset} alt="" aria-hidden="true" />
        {theme.overlayAsset ? <img className="room-stage__theme-overlay" src={theme.overlayAsset} alt="" aria-hidden="true" /> : null}
        {ROOM_SCENE_ORDER.map((code) => (
          <RoomSlot
            key={code}
            slot={ROOM_SCENE_SLOTS[code]}
            item={indexed[code]}
            interactive={interactive}
            selected={selectedCode === code}
            onSelect={onSelect}
          />
        ))}
      </div>
    </div>
  );
}

function RoomSlot({ slot, item, interactive, selected, onSelect }) {
  const level = normalizeRoomLevel(item?.current_level);
  const presentationMode = roomPresentationMode(slot, item);
  const showLevelBadge = slot.showLevelBadge !== false && presentationMode !== "achievement_case";
  const assetPath = slot.availableLevels?.includes(level) ? slot.assets?.[level] : "";
  const itemLabel = presentationMode === "achievement_case" ? `${slot.title} ${roomMetaLabel(slot, item)}` : `${slot.title} ${level}`;
  const itemTitle = presentationMode === "achievement_case" ? `${slot.title} • ${roomMetaLabel(slot, item)}` : `${slot.title} • ${labelize(level)}`;
  return (
    <div
      className={`room-slot room-slot--${slot.code} room-slot--${level}${selected ? " is-selected" : ""}`}
      style={slot.style}
      aria-label={itemLabel}
      title={itemTitle}
    >
      <button
        type="button"
        className="room-slot__button"
        onClick={() => onSelect?.(slot.code)}
        disabled={!interactive}
        aria-pressed={selected}
      >
        <div className="room-slot__shadow" />
        {assetPath ? (
          <img className="room-slot__asset" src={assetPath} alt="" aria-hidden="true" />
        ) : (
          <div className={`room-slot__placeholder room-slot__placeholder--${slot.code}`}>
            <div className="room-slot__silhouette" />
            {showLevelBadge ? <span className={`room-slot__level-badge room-slot__level-badge--${level}`}>{level}</span> : null}
          </div>
        )}
      </button>
    </div>
  );
}

function RoomInspector({
  item,
  slot,
  items,
  onSelect,
}) {
  if (!item || !slot) {
    return (
      <div className="empty-panel">
        <h3>Select an item</h3>
        <p>The room inspector shows the skill mapping, level, and linked evidence for the selected slot.</p>
      </div>
    );
  }

  const state = readState(item);
  const presentationMode = roomPresentationMode(slot, item);
  const achievementEntries = roomAchievementEntries(item);
  return (
    <div className="room-inspector__stack">
      <div className="room-inspector__hero">
        <div>
          <p className="room-inspector__skill">{slot.skill}</p>
          <h3>{roomLabel(item.room_item_code)}</h3>
        </div>
        {presentationMode === "achievement_case" ? (
          <span className="room-inspector__meta">{roomMetaLabel(slot, item)}</span>
        ) : (
          <span className={`skill-level ${levelClass(item.current_level)}`}>{item.current_level}</span>
        )}
      </div>

      <div className="room-inspector__selector">
        {items.map((entry) => (
          <button
            key={entry.room_item_code}
            type="button"
            className={entry.room_item_code === item.room_item_code ? "room-inspector__chip active" : "room-inspector__chip"}
            onClick={() => onSelect(entry.room_item_code)}
          >
            <span>{labelize(entry.room_item_code)}</span>
            <strong>{roomMetaLabel(ROOM_SCENE_SLOTS[entry.room_item_code], entry)}</strong>
          </button>
        ))}
      </div>

      <div className="room-inspector__section">
        <span className="room-inspector__label">Mapped signal</span>
        <p>{state.explanation || "No explanation yet."}</p>
      </div>

      {presentationMode === "achievement_case" ? (
        <div className="room-inspector__section">
          <span className="room-inspector__label">Unlocked trophies</span>
          {achievementEntries.length > 0 ? (
            <ul className="plain-list">
              {achievementEntries.map((achievement) => (
                <li key={achievement.code || achievement.title}>
                  <strong>{achievement.title}</strong>
                  {achievement.description ? ` — ${achievement.description}` : ""}
                </li>
              ))}
            </ul>
          ) : (
            <p className="muted-copy">The case is empty until verified milestones are reached.</p>
          )}
        </div>
      ) : null}

      <div className="room-inspector__section">
        <span className="room-inspector__label">{presentationMode === "achievement_case" ? "Recent evidence" : "Linked tasks"}</span>
        {(state.linked_tasks || []).length > 0 ? (
          <ul className="plain-list">
            {state.linked_tasks.map((task) => (
              <li key={task}>{task}</li>
            ))}
          </ul>
        ) : (
          <p className="muted-copy">
            {presentationMode === "achievement_case"
              ? "Verified activity will appear here as trophies are earned."
              : "Evidence appears here once tasks are completed and scored."}
          </p>
        )}
      </div>
    </div>
  );
}

function Stat({ label, value }) {
  return (
    <div className="stat-block">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}
