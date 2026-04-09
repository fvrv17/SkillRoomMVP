"use client";

import { forwardRef, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { apiFetch, clearAuth, isUnauthorizedError, loadAuth, loadRegionId, subscribeAuth } from "@/lib/client";
import { REGIONS } from "@/lib/preview-data";
import {
  ROOM_DEFAULT_FLOOR_STYLE,
  ROOM_DEFAULT_THEME_ID,
  ROOM_DEFAULT_WALL_STYLE,
  ROOM_DEFAULT_WINDOW_SCENE_ID,
  ROOM_FLOOR_STYLES,
  ROOM_SCENE_ORDER,
  ROOM_SCENE_SLOTS,
  ROOM_THEMES,
  ROOM_WALL_STYLES,
  ROOM_WINDOW_SCENES,
  normalizeRoomLevel,
  normalizeRoomTheme,
  normalizeWindowScene,
  resolveRoomCustomization,
} from "@/lib/room-scene-config";

const DEVELOPER_NAV_ITEMS = [
  { id: "overview", label: "Overview" },
  { id: "challenges", label: "Challenges" },
  { id: "room", label: "Room" },
  { id: "ranking", label: "Ranking" },
];

const HR_NAV_ITEMS = [
  { id: "overview", label: "Overview" },
  { id: "candidates", label: "Candidates" },
  { id: "leaderboard", label: "Leaderboard" },
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
    leaderboard: [],
    monetization: null,
    cosmetics: { catalog: [], owned: [], equipped: [] },
    roomCustomization: { equipped: [] },
  });
  const [currentChallenge, setCurrentChallenge] = useState(null);
  const [activeFile, setActiveFile] = useState("");
  const [runResult, setRunResult] = useState(null);
  const [submissionResult, setSubmissionResult] = useState(null);
  const [hintResult, setHintResult] = useState(null);
  const [explanationResult, setExplanationResult] = useState(null);
  const [busyAction, setBusyAction] = useState("");
  const [selectedRoomCode, setSelectedRoomCode] = useState("");
  const [visibleRoomPopoverCode, setVisibleRoomPopoverCode] = useState("");
  const [selectedCandidate, setSelectedCandidate] = useState(null);
  const [selectedCandidateRoomCode, setSelectedCandidateRoomCode] = useState("");
  const [visibleCandidateRoomPopoverCode, setVisibleCandidateRoomPopoverCode] = useState("");
  const [linkedInDraft, setLinkedInDraft] = useState("");
  const [profileNotice, setProfileNotice] = useState("");
  const [customizationOpen, setCustomizationOpen] = useState(false);
  const roomThemeID = ROOM_DEFAULT_THEME_ID;

  useEffect(() => {
    const auth = loadAuth();
    const region = REGIONS.find((item) => item.id === loadRegionId()) || REGIONS[0];
    const nextSession = { auth, region };
    setSession(nextSession);
    setActiveView(defaultView(auth?.user?.role));
    setBooting(false);

    return subscribeAuth(
      (nextAuth) => {
        setSession((current) => ({ ...(current || { region }), auth: nextAuth || null }));
      },
      () => {
        setSession((current) => ({ ...(current || { region }), auth: null }));
      },
    );
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
  const leaderboard = data.leaderboard || [];
  const monetization = data.monetization || null;
  const cosmeticInventory = data.cosmetics || { catalog: [], owned: [], equipped: [] };
  const roomCustomization = useMemo(
    () => resolveRoomCustomization(data.roomCustomization?.equipped || cosmeticInventory.equipped || []),
    [data.roomCustomization, cosmeticInventory.equipped],
  );
  const windowSceneID = roomCustomization.windowSceneID || ROOM_DEFAULT_WINDOW_SCENE_ID;
  const token = session?.auth?.access_token || "";
  const isHR = user.role === "hr";
  const allowedNav = useMemo(() => (isHR ? HR_NAV_ITEMS : DEVELOPER_NAV_ITEMS), [isHR]);
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
  const candidateRoomItems = selectedCandidate?.room || [];
  const candidateRoomCustomization = useMemo(
    () => resolveRoomCustomization(selectedCandidate?.room_customization?.equipped || []),
    [selectedCandidate],
  );
  const orderedCandidateRoomItems = useMemo(
    () =>
      ROOM_SCENE_ORDER.map((code) => candidateRoomItems.find((item) => item.room_item_code === code)).filter(Boolean),
    [candidateRoomItems],
  );
  const selectedCandidateRoomItem =
    orderedCandidateRoomItems.find((item) => item.room_item_code === selectedCandidateRoomCode) || orderedCandidateRoomItems[0] || null;
  const selectedCandidateRoomSlot = selectedCandidateRoomItem ? ROOM_SCENE_SLOTS[selectedCandidateRoomItem.room_item_code] : null;

  useEffect(() => {
    if (orderedRoomItems.length === 0) {
      if (selectedRoomCode) {
        setSelectedRoomCode("");
      }
      if (visibleRoomPopoverCode) {
        setVisibleRoomPopoverCode("");
      }
      return;
    }
    if (!orderedRoomItems.some((item) => item.room_item_code === selectedRoomCode)) {
      setSelectedRoomCode(orderedRoomItems[0].room_item_code);
    }
  }, [orderedRoomItems, selectedRoomCode, visibleRoomPopoverCode]);

  useEffect(() => {
    if (orderedCandidateRoomItems.length === 0) {
      if (selectedCandidateRoomCode) {
        setSelectedCandidateRoomCode("");
      }
      if (visibleCandidateRoomPopoverCode) {
        setVisibleCandidateRoomPopoverCode("");
      }
      return;
    }
    if (!orderedCandidateRoomItems.some((item) => item.room_item_code === selectedCandidateRoomCode)) {
      setSelectedCandidateRoomCode(orderedCandidateRoomItems[0].room_item_code);
    }
  }, [orderedCandidateRoomItems, selectedCandidateRoomCode, visibleCandidateRoomPopoverCode]);

  useEffect(() => {
    if (!visibleRoomPopoverCode) {
      return undefined;
    }
    const timeoutID = window.setTimeout(() => {
      setVisibleRoomPopoverCode((current) => (current === visibleRoomPopoverCode ? "" : current));
    }, 7000);
    return () => window.clearTimeout(timeoutID);
  }, [visibleRoomPopoverCode]);

  useEffect(() => {
    if (!visibleCandidateRoomPopoverCode) {
      return undefined;
    }
    const timeoutID = window.setTimeout(() => {
      setVisibleCandidateRoomPopoverCode((current) => (current === visibleCandidateRoomPopoverCode ? "" : current));
    }, 7000);
    return () => window.clearTimeout(timeoutID);
  }, [visibleCandidateRoomPopoverCode]);

  useEffect(() => {
    if (isHR) {
      setLinkedInDraft("");
      setProfileNotice("");
      return;
    }
    setLinkedInDraft(profile.linkedin_url || "");
  }, [isHR, profile.linkedin_url]);

  useEffect(() => {
    if (allowedNav.length === 0) {
      return;
    }
    if (isHR && activeView === "candidate-room") {
      return;
    }
    if (!allowedNav.some((item) => item.id === activeView)) {
      setActiveView(allowedNav[0].id);
    }
  }, [activeView, allowedNav, isHR]);

  async function loadWorkspace(currentSession) {
    setLoading(true);
    setError("");
    setProfileNotice("");

    try {
      if (!currentSession.auth?.access_token) {
        return;
      }

      const me = await apiFetch("/v1/me", { token: currentSession.auth.access_token });
      if (me.role === "hr") {
        const [candidatePayload, leaderboardPayload] = await Promise.all([
          fetchCandidates(currentSession.auth.access_token, filters),
          fetchHRLeaderboard(currentSession.auth.access_token),
        ]);
        setData({
          user: me,
          profile: null,
          skills: [],
          room: [],
          rankings: [],
          templates: [],
          candidates: candidatePayload.candidates || [],
          leaderboard: leaderboardPayload.rankings || [],
          monetization: candidatePayload.monetization || leaderboardPayload.monetization || null,
          cosmetics: { catalog: [], owned: [], equipped: [] },
          roomCustomization: { equipped: [] },
        });
      } else {
        const [profilePayload, skillsPayload, roomPayload, templatesPayload, rankingPayload, monetizationPayload, cosmeticsPayload] = await Promise.all([
          apiFetch("/v1/profile", { token: currentSession.auth.access_token }),
          apiFetch("/v1/skills", { token: currentSession.auth.access_token }),
          apiFetch("/v1/room", { token: currentSession.auth.access_token }),
          apiFetch("/v1/challenges/templates", { token: currentSession.auth.access_token }),
          apiFetch("/v1/rankings/global", { token: currentSession.auth.access_token }),
          apiFetch("/v1/monetization/summary", { token: currentSession.auth.access_token }),
          fetchCosmeticInventory(currentSession.auth.access_token),
        ]);
        setData({
          user: me,
          profile: profilePayload,
          skills: skillsPayload.skills || [],
          room: roomPayload.items || [],
          roomCustomization: roomPayload.customization || { equipped: [] },
          rankings: rankingPayload.rankings || [],
          templates: templatesPayload.templates || [],
          candidates: [],
          leaderboard: [],
          monetization: monetizationPayload || null,
          cosmetics: cosmeticsPayload || { catalog: [], owned: [], equipped: [] },
        });
      }
      setSelectedCandidate(null);
      setActiveView(defaultView(me.role));
    } catch (loadError) {
      if (isUnauthorizedError(loadError)) {
        setError("");
        return;
      }
      setError(loadError instanceof Error ? loadError.message : "Unable to load workspace");
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
    let nextLeaderboard = data.leaderboard;
    let nextMonetization = data.monetization;
    if (user.role === "hr") {
      const [candidatePayload, leaderboardPayload] = await Promise.all([
        fetchCandidates(token, filters),
        fetchHRLeaderboard(token),
      ]);
      nextCandidates = candidatePayload.candidates || [];
      nextLeaderboard = leaderboardPayload.rankings || [];
      nextMonetization = candidatePayload.monetization || leaderboardPayload.monetization || null;
    }
    setData((current) => ({
      ...current,
      profile: profilePayload,
      skills: skillsPayload,
      room: roomPayload,
      roomCustomization: current.roomCustomization,
      rankings: rankingPayload.rankings || [],
      candidates: nextCandidates,
      leaderboard: nextLeaderboard,
      monetization: nextMonetization,
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
      setData((current) => ({
        ...current,
        candidates: candidatePayload.candidates || [],
        monetization: candidatePayload.monetization || current.monetization,
      }));
      setSelectedCandidate(null);
    } catch (candidateError) {
      setError(candidateError instanceof Error ? candidateError.message : "Unable to load candidates");
    } finally {
      setBusyAction("");
    }
  }

  async function handleOpenCandidate(userID) {
    if (!token || user.role !== "hr") {
      return;
    }
    setBusyAction(`candidate:${userID}`);
    setError("");
    try {
      const payload = await fetchCandidateDetail(token, userID);
      setSelectedCandidate(payload);
      setSelectedCandidateRoomCode("");
      setVisibleCandidateRoomPopoverCode("");
      setData((current) => ({
        ...current,
        monetization: payload.monetization || current.monetization,
      }));
    } catch (candidateError) {
      setError(candidateError instanceof Error ? candidateError.message : "Unable to load candidate detail");
    } finally {
      setBusyAction("");
    }
  }

  async function handleUnlockCandidate(userID) {
    if (!token || user.role !== "hr") {
      return;
    }
    setBusyAction(`unlock:${userID}`);
    setError("");
    try {
      const payload = await unlockCandidate(token, userID);
      setSelectedCandidate(payload);
      const [candidatePayload, leaderboardPayload] = await Promise.all([
        fetchCandidates(token, filters),
        fetchHRLeaderboard(token),
      ]);
      setData((current) => ({
        ...current,
        candidates: candidatePayload.candidates || current.candidates,
        leaderboard: leaderboardPayload.rankings || current.leaderboard,
        monetization: payload.monetization || candidatePayload.monetization || leaderboardPayload.monetization || current.monetization,
      }));
    } catch (unlockError) {
      setError(unlockError instanceof Error ? unlockError.message : "Unable to unlock candidate");
    } finally {
      setBusyAction("");
    }
  }

  async function handleInviteCandidate(userID) {
    if (!token || user.role !== "hr") {
      return;
    }
    setBusyAction(`invite:${userID}`);
    setError("");
    try {
      const payload = await inviteCandidate(token, userID);
      setSelectedCandidate(payload);
      const [candidatePayload, leaderboardPayload] = await Promise.all([
        fetchCandidates(token, filters),
        fetchHRLeaderboard(token),
      ]);
      setData((current) => ({
        ...current,
        candidates: candidatePayload.candidates || current.candidates,
        leaderboard: leaderboardPayload.rankings || current.leaderboard,
        monetization: payload.monetization || candidatePayload.monetization || leaderboardPayload.monetization || current.monetization,
      }));
    } catch (inviteError) {
      setError(inviteError instanceof Error ? inviteError.message : "Unable to invite candidate");
    } finally {
      setBusyAction("");
    }
  }

  async function handleOpenCandidateRoom(userID) {
    if (!token || user.role !== "hr") {
      return;
    }
    setBusyAction(`room:${userID}`);
    setError("");
    try {
      const payload = await fetchCandidateDetail(token, userID);
      setSelectedCandidate(payload);
      setSelectedCandidateRoomCode("");
      setVisibleCandidateRoomPopoverCode("");
      setData((current) => ({
        ...current,
        monetization: payload.monetization || current.monetization,
      }));
      setActiveView("candidate-room");
    } catch (roomError) {
      setError(roomError instanceof Error ? roomError.message : "Unable to load candidate room");
    } finally {
      setBusyAction("");
    }
  }

  async function handleSaveProfile() {
    if (!token || isHR) {
      return;
    }
    setBusyAction("profile");
    setError("");
    setProfileNotice("");
    try {
      const payload = await apiFetch("/v1/profile", {
        method: "PATCH",
        token,
        body: {
          linkedin_url: linkedInDraft,
        },
      });
      setData((current) => ({
        ...current,
        profile: payload,
        user: {
          ...(current.user || {}),
          profile: payload,
        },
      }));
      setLinkedInDraft(payload.linkedin_url || "");
      setProfileNotice("LinkedIn profile saved.");
    } catch (profileError) {
      setError(profileError instanceof Error ? profileError.message : "Unable to save profile");
    } finally {
      setBusyAction("");
    }
  }

  async function handleEquipCosmetic(cosmeticCode) {
    if (!token || isHR) {
      return;
    }
    setBusyAction(`equip:${cosmeticCode}`);
    setError("");
    try {
      const payload = await equipCosmetic(token, cosmeticCode);
      setData((current) => ({
        ...current,
        cosmetics: payload,
        roomCustomization: {
          equipped: payload.equipped || [],
        },
      }));
    } catch (cosmeticError) {
      setError(cosmeticError instanceof Error ? cosmeticError.message : "Unable to equip cosmetic");
    } finally {
      setBusyAction("");
    }
  }

  function handleSelectRoomSlot(code) {
    setSelectedRoomCode(code);
    setVisibleRoomPopoverCode("");
    window.setTimeout(() => {
      setVisibleRoomPopoverCode(code);
    }, 0);
  }

  function handleSelectCandidateRoomSlot(code) {
    setSelectedCandidateRoomCode(code);
    setVisibleCandidateRoomPopoverCode("");
    window.setTimeout(() => {
      setVisibleCandidateRoomPopoverCode(code);
    }, 0);
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
          {isHR ? (
            <span className="meta-pill">
              {`${Math.max((monetization?.entitlements?.candidate_unlocks_per_month || 0) - (monetization?.usage?.candidate_unlocks_used || 0), 0)} unlocks left`}
            </span>
          ) : (
            <span className="meta-pill">{`Confidence ${formatNumber(profile.confidence_score)}`}</span>
          )}
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
          {isHR ? (
            <>
              <div className="card stats-card">
                <p className="eyebrow">Recruiter plan</p>
                <h2>{labelize(monetization?.plan?.tier || "free")}</h2>
                <div className="stats-row">
                  <Stat label="Unlocks used" value={monetization?.usage?.candidate_unlocks_used || 0} />
                  <Stat
                    label="Unlocks left"
                    value={Math.max((monetization?.entitlements?.candidate_unlocks_per_month || 0) - (monetization?.usage?.candidate_unlocks_used || 0), 0)}
                  />
                  <Stat
                    label="Invites left"
                    value={Math.max((monetization?.entitlements?.candidate_invites_per_month || 0) - (monetization?.usage?.candidate_invites_used || 0), 0)}
                  />
                </div>
              </div>
              <div className="card">
                <p className="eyebrow">Recruiter workspace</p>
                <h3>Review candidates, unlock full profiles, and inspect candidate rooms.</h3>
                <ul className="plain-list">
                  <li>Developer challenges and rankings are hidden from recruiter accounts.</li>
                  <li>Candidate room state appears only inside unlocked candidate detail.</li>
                  <li>Unlocks are enforced by your active HR plan.</li>
                </ul>
                <div className="action-row action-row--compact">
                  <button type="button" className="primary-button" onClick={() => setActiveView("leaderboard")}>
                    Open leaderboard
                  </button>
                  <button type="button" className="secondary-button" onClick={() => setActiveView("candidates")}>
                    Open candidates
                  </button>
                </div>
              </div>
              <div className="card card--span-2">
                <div className="section-heading">
                  <div>
                    <p className="eyebrow">Top candidates</p>
                    <h3>Leaderboard preview</h3>
                  </div>
                </div>
                <div className="ranking-table">
                  <div className="ranking-row ranking-row--header">
                    <span>Rank</span>
                    <span>User</span>
                    <span>Score</span>
                    <span>Confidence</span>
                    <span>Status</span>
                    <span>Action</span>
                  </div>
                  {leaderboard.slice(0, 5).map((candidate, index) => (
                    <div key={`${candidate.user_id}-${index}`} className="ranking-row">
                      <span>#{index + 1}</span>
                      <span>{candidate.username}</span>
                      <span>{formatNumber(candidate.summary?.score ?? candidate.current_skill_score)}</span>
                      <span>{formatNumber(candidate.summary?.confidence_score ?? candidate.confidence_score)}</span>
                      <span>{candidateAccessLabel(candidate.access)}</span>
                      <button type="button" className="secondary-button" onClick={() => handleOpenCandidate(candidate.user_id)}>
                        {candidate.access?.is_unlocked ? "Open profile" : "Preview"}
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            </>
          ) : (
            <>
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
              <div className="card">
                <div className="section-heading">
                  <div>
                    <p className="eyebrow">Developer profile</p>
                    <h3>Attach your LinkedIn</h3>
                  </div>
                </div>
                <label className="form-row profile-form-row">
                  <span>LinkedIn</span>
                  <input
                    type="url"
                    placeholder="https://www.linkedin.com/in/your-profile"
                    value={linkedInDraft}
                    onChange={(event) => setLinkedInDraft(event.target.value)}
                  />
                </label>
                <p className="muted-copy">Recruiters can view this link only after they unlock your full profile.</p>
                {profileNotice ? <p className="inline-success">{profileNotice}</p> : null}
                <button type="button" className="primary-button" onClick={handleSaveProfile} disabled={busyAction === "profile"}>
                  {busyAction === "profile" ? "Saving..." : "Save profile"}
                </button>
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
                <RoomStage
                  items={roomItems}
                  compact
                  themeID={roomThemeID}
                  windowSceneID={windowSceneID}
                  customization={roomCustomization}
                />
              </div>
            </>
          )}
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
              popoverCode={visibleRoomPopoverCode}
              onSelect={handleSelectRoomSlot}
              themeID={roomThemeID}
              windowSceneID={windowSceneID}
              customization={roomCustomization}
            />
          </div>
          <div className="card room-inspector">
            <p className="eyebrow">Item inspector</p>
            <RoomInspector
              item={selectedRoomItem}
              slot={selectedRoomSlot}
              onSelect={handleSelectRoomSlot}
              items={orderedRoomItems}
            />
          </div>
          <div className="card card--span-2">
            <RoomCustomizationPanel
              inventory={cosmeticInventory}
              monetization={monetization}
              onEquip={handleEquipCosmetic}
              busyAction={busyAction}
              open={customizationOpen}
              onToggle={() => setCustomizationOpen((current) => !current)}
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

      {activeView === "leaderboard" ? (
        <section className="workspace-grid workspace-grid--ranking">
          <div className="card card--span-2">
            <div className="section-heading">
              <div>
                <p className="eyebrow">Candidate leaderboard</p>
                <h3>Sorted by score, confidence, and recent activity</h3>
              </div>
            </div>
            <div className="ranking-table">
              <div className="ranking-row ranking-row--header">
                <span>Rank</span>
                <span>User</span>
                <span>Score</span>
                <span>Confidence</span>
                <span>Status</span>
                <span>Action</span>
              </div>
              {leaderboard.map((candidate, index) => (
                <div key={`${candidate.user_id}-${index}`} className="ranking-row">
                  <span>#{index + 1}</span>
                  <span>{candidate.username}</span>
                  <span>{formatNumber(candidate.summary?.score ?? candidate.current_skill_score)}</span>
                  <span>{formatNumber(candidate.summary?.confidence_score ?? candidate.confidence_score)}</span>
                  <span>{candidateAccessLabel(candidate.access)}</span>
                  <div className="candidate-card__actions">
                    <button
                      type="button"
                      className="secondary-button"
                      onClick={() => handleOpenCandidate(candidate.user_id)}
                      disabled={busyAction === `candidate:${candidate.user_id}`}
                    >
                      {busyAction === `candidate:${candidate.user_id}` ? "Loading..." : candidate.access?.is_unlocked ? "Open profile" : "Preview"}
                    </button>
                    <button
                      type="button"
                      className="secondary-button"
                      onClick={() => handleOpenCandidateRoom(candidate.user_id)}
                      disabled={!candidate.access?.is_unlocked || busyAction === `room:${candidate.user_id}`}
                    >
                      {busyAction === `room:${candidate.user_id}` ? "Opening..." : "Open room"}
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </section>
      ) : null}

      {activeView === "candidates" ? (
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
                {monetization ? (
                  <div className="meta-row">
                    <span className="meta-pill">{`${labelize(monetization.plan?.tier || "free")} plan`}</span>
                    <span className="meta-pill">{`${monetization.usage?.candidate_unlocks_used || 0} / ${monetization.entitlements?.candidate_unlocks_per_month || 0} unlocks used`}</span>
                    <span className="meta-pill">{`${Math.max((monetization.entitlements?.candidate_unlocks_per_month || 0) - (monetization.usage?.candidate_unlocks_used || 0), 0)} remaining`}</span>
                  </div>
                ) : null}
              </div>
              {selectedCandidate ? (
                <div className="card card--span-2 candidate-detail-card">
                  <div className="candidate-card__header">
                    <div>
                      <p className="eyebrow">Candidate detail</p>
                      <h3>{selectedCandidate.candidate?.username || "Candidate"}</h3>
                      <p>{selectedCandidate.candidate?.country || ""}</p>
                    </div>
                    <strong>{candidateAccessLabel(selectedCandidate.candidate?.access)}</strong>
                  </div>
                  <div className="stats-row">
                    <Stat label="Score" value={formatNumber(selectedCandidate.candidate?.summary?.score || selectedCandidate.candidate?.current_skill_score)} />
                    <Stat label="Percentile" value={formatNumber(selectedCandidate.candidate?.summary?.percentile || selectedCandidate.candidate?.percentile_global)} />
                    <Stat label="Confidence" value={`${formatNumber(selectedCandidate.candidate?.summary?.confidence_score || selectedCandidate.candidate?.confidence_score)} / ${String(selectedCandidate.candidate?.summary?.confidence_level || selectedCandidate.candidate?.confidence_level || "medium").toUpperCase()}`} />
                  </div>
                  {selectedCandidate.candidate?.access?.is_unlocked ? (
                    <div className="candidate-detail-grid">
                      <div className="candidate-detail-block">
                        <span>Contact</span>
                        <p>{selectedCandidate.contact?.email || "N/A"}</p>
                        <p>
                          {selectedCandidate.contact?.linkedin_url ? (
                            <a href={selectedCandidate.contact.linkedin_url} target="_blank" rel="noreferrer">
                              LinkedIn profile
                            </a>
                          ) : (
                            "No LinkedIn attached"
                          )}
                        </p>
                      </div>
                      <div className="candidate-detail-block">
                        <span>Profile</span>
                        <p>{selectedCandidate.profile?.selected_track || "react"}</p>
                        <p>{selectedCandidate.profile?.bio || "No bio yet."}</p>
                      </div>
                      <div className="candidate-detail-block">
                        <span>Skills</span>
                        <ul className="plain-list">
                          {(selectedCandidate.skills || []).slice(0, 5).map((skill) => (
                            <li key={skill.skill_code}>{`${labelize(skill.skill_code)} — ${formatNumber(skill.score)}`}</li>
                          ))}
                        </ul>
                      </div>
                      <div className="candidate-detail-block">
                        <span>Recent submissions</span>
                        <ul className="plain-list">
                          {(selectedCandidate.recent_submissions || []).slice(0, 5).map((entry) => (
                            <li key={entry.submission_id}>{`${entry.template_title} · ${formatNumber(entry.final_score)} · ${entry.execution_status}`}</li>
                          ))}
                        </ul>
                      </div>
                    </div>
                  ) : (
                    <div className="candidate-detail-lock">
                      <p>Contact, full profile, room breakdown, and submission history are locked until this candidate is unlocked.</p>
                      <p>{`Locked fields: ${(selectedCandidate.locked_fields || []).map(labelize).join(", ")}`}</p>
                      <button
                        type="button"
                        className="primary-button"
                        onClick={() => handleUnlockCandidate(selectedCandidate.candidate?.user_id)}
                        disabled={busyAction === `unlock:${selectedCandidate.candidate?.user_id}` || !selectedCandidate.candidate?.access?.can_unlock}
                      >
                        {busyAction === `unlock:${selectedCandidate.candidate?.user_id}` ? "Unlocking..." : unlockActionLabel(selectedCandidate.candidate?.access)}
                      </button>
                    </div>
                  )}
                  {selectedCandidate.candidate?.access?.is_unlocked ? (
                    <div className="candidate-card__actions">
                      <button
                        type="button"
                        className="secondary-button"
                        onClick={() => handleOpenCandidateRoom(selectedCandidate.candidate?.user_id)}
                        disabled={busyAction === `room:${selectedCandidate.candidate?.user_id}`}
                      >
                        {busyAction === `room:${selectedCandidate.candidate?.user_id}` ? "Opening..." : "Open room"}
                      </button>
                      <button
                        type="button"
                        className="primary-button"
                        onClick={() => handleInviteCandidate(selectedCandidate.candidate?.user_id)}
                        disabled={selectedCandidate.candidate?.access?.is_invited || !selectedCandidate.candidate?.access?.can_invite || busyAction === `invite:${selectedCandidate.candidate?.user_id}`}
                      >
                        {busyAction === `invite:${selectedCandidate.candidate?.user_id}` ? "Inviting..." : inviteActionLabel(selectedCandidate.candidate?.access)}
                      </button>
                    </div>
                  ) : null}
                </div>
              ) : null}
              {candidates.map((candidate) => {
                const summary = candidate.summary || candidate;
                const access = candidate.access || {};
                return (
                  <div key={candidate.user_id} className="card candidate-card">
                    <div className="candidate-card__header">
                      <div>
                        <h3>{candidate.username}</h3>
                        <p>{candidate.country}</p>
                      </div>
                      <div className="candidate-card__status">
                        <span className={access.is_unlocked ? "meta-pill meta-pill--success" : "meta-pill"}>{candidateAccessLabel(access)}</span>
                        <strong>{formatNumber(summary.score ?? candidate.current_skill_score)}</strong>
                      </div>
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
                    <div className="candidate-card__actions">
                      <button
                        type="button"
                        className="secondary-button"
                        onClick={() => handleOpenCandidate(candidate.user_id)}
                        disabled={busyAction === `candidate:${candidate.user_id}`}
                      >
                        {busyAction === `candidate:${candidate.user_id}` ? "Loading..." : access.is_unlocked ? "View full profile" : "View preview"}
                      </button>
                      <button
                        type="button"
                        className="primary-button"
                        onClick={() => handleUnlockCandidate(candidate.user_id)}
                        disabled={access.is_unlocked || !access.can_unlock || busyAction === `unlock:${candidate.user_id}`}
                      >
                        {busyAction === `unlock:${candidate.user_id}` ? "Unlocking..." : unlockActionLabel(access)}
                      </button>
                      <button
                        type="button"
                        className="secondary-button"
                        onClick={() => handleOpenCandidateRoom(candidate.user_id)}
                        disabled={!access.is_unlocked || busyAction === `room:${candidate.user_id}`}
                      >
                        {busyAction === `room:${candidate.user_id}` ? "Opening..." : "Open room"}
                      </button>
                    </div>
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

      {activeView === "candidate-room" ? (
        <section className="workspace-grid workspace-grid--room">
          {selectedCandidate?.candidate?.access?.is_unlocked ? (
            <>
              <div className="card room-scene-card">
                <div className="section-heading">
                  <div>
                    <p className="eyebrow">Candidate room</p>
                    <h3>{`${selectedCandidate.candidate?.username || "Candidate"}'s skill room`}</h3>
                  </div>
                  <div className="candidate-card__actions">
                    <button type="button" className="secondary-button" onClick={() => setActiveView("leaderboard")}>
                      Back to leaderboard
                    </button>
                    <button
                      type="button"
                      className="primary-button"
                      onClick={() => handleInviteCandidate(selectedCandidate.candidate?.user_id)}
                      disabled={selectedCandidate.candidate?.access?.is_invited || !selectedCandidate.candidate?.access?.can_invite || busyAction === `invite:${selectedCandidate.candidate?.user_id}`}
                    >
                      {busyAction === `invite:${selectedCandidate.candidate?.user_id}` ? "Inviting..." : inviteActionLabel(selectedCandidate.candidate?.access)}
                    </button>
                  </div>
                </div>
                <RoomStage
                  items={selectedCandidate.room || []}
                  selectedCode={selectedCandidateRoomCode}
                  popoverCode={visibleCandidateRoomPopoverCode}
                  onSelect={handleSelectCandidateRoomSlot}
                  themeID={roomThemeID}
                  windowSceneID={candidateRoomCustomization.windowSceneID || ROOM_DEFAULT_WINDOW_SCENE_ID}
                  customization={candidateRoomCustomization}
                />
              </div>
              <div className="card room-inspector">
                <p className="eyebrow">Candidate inspector</p>
                <RoomInspector
                  item={selectedCandidateRoomItem}
                  slot={selectedCandidateRoomSlot}
                  onSelect={handleSelectCandidateRoomSlot}
                  items={orderedCandidateRoomItems}
                />
              </div>
            </>
          ) : (
            <div className="card empty-panel">
              <h3>Unlock a candidate first</h3>
              <p>Candidate rooms become available after the full profile is unlocked.</p>
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
    return "candidates";
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

async function fetchHRLeaderboard(token) {
  return apiFetch("/v1/hr/leaderboard", { token });
}

async function fetchCandidateDetail(token, userID) {
  return apiFetch(`/v1/hr/candidates/${userID}`, { token });
}

async function fetchCosmeticInventory(token) {
  return apiFetch("/v1/dev/cosmetics/inventory", { token });
}

async function unlockCandidate(token, userID) {
  return apiFetch(`/v1/hr/candidates/${userID}/unlock`, {
    method: "POST",
    token,
  });
}

async function inviteCandidate(token, userID) {
  return apiFetch(`/v1/hr/candidates/${userID}/invite`, {
    method: "POST",
    token,
  });
}

async function equipCosmetic(token, cosmeticCode) {
  return apiFetch("/v1/dev/cosmetics/equip", {
    method: "POST",
    token,
    body: {
      cosmetic_code: cosmeticCode,
    },
  });
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

function sanitizeClass(value) {
  return String(value || "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function candidateAccessLabel(access) {
  if (!access) {
    return "Locked";
  }
  if (access.is_invited) {
    return "Invited";
  }
  if (access.is_unlocked) {
    return "Unlocked";
  }
  return `${access.remaining_unlocks || 0} unlocks left`;
}

function unlockActionLabel(access) {
  if (!access) {
    return "Unlock candidate";
  }
  if (access.is_unlocked) {
    return "Unlocked";
  }
  if (!access.can_unlock) {
    return "Unlock limit reached";
  }
  return `Unlock candidate (${access.remaining_unlocks || 0} left)`;
}

function inviteActionLabel(access) {
  if (!access) {
    return "Invite candidate";
  }
  if (access.is_invited) {
    return "Invited";
  }
  if (!access.is_unlocked) {
    return "Unlock before invite";
  }
  if (!access.can_invite) {
    return "Invite limit reached";
  }
  return `Invite candidate (${access.remaining_invites || 0} left)`;
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

function RoomStage({
  items,
  compact = false,
  selectedCode = "",
  popoverCode = "",
  onSelect,
  themeID = ROOM_DEFAULT_THEME_ID,
  windowSceneID = ROOM_DEFAULT_WINDOW_SCENE_ID,
  customization,
}) {
  const indexed = {};
  for (const item of items || []) {
    indexed[item.room_item_code] = item;
  }
  const interactive = typeof onSelect === "function" && !compact;
  const theme = ROOM_THEMES[normalizeRoomTheme(themeID)];
  const appearance = customization || resolveRoomCustomization([]);
  const windowScene = ROOM_WINDOW_SCENES[normalizeWindowScene(appearance.windowSceneID || windowSceneID)];
  const wallClassName = appearance.wallStyle?.className || ROOM_WALL_STYLES[ROOM_DEFAULT_WALL_STYLE].className;
  const floorClassName = appearance.floorStyle?.className || ROOM_FLOOR_STYLES[ROOM_DEFAULT_FLOOR_STYLE].className;

  return (
    <div className={compact ? `room-stage room-stage--compact room-stage--${theme.id}` : `room-stage room-stage--${theme.id}`}>
      <div className="room-stage__canvas">
        <img className="room-stage__window-scene" src={windowScene.asset} alt="" aria-hidden="true" />
        <div className={`room-stage__wall-tint ${wallClassName}`} aria-hidden="true" />
        <div className={`room-stage__floor-tint ${floorClassName}`} aria-hidden="true" />
        <img className="room-stage__shell" src={theme.shellAsset} alt="" aria-hidden="true" />
        {theme.overlayAsset ? <img className="room-stage__theme-overlay" src={theme.overlayAsset} alt="" aria-hidden="true" /> : null}
        {(appearance.decor || []).map((decor) => (
          <div key={decor.code} className="room-decor" style={decor.style} aria-hidden="true">
            <div className={`room-decor__asset ${decor.className || ""}`} />
          </div>
        ))}
        {ROOM_SCENE_ORDER.map((code) => (
          <RoomSlot
            key={code}
            slot={ROOM_SCENE_SLOTS[code]}
            item={indexed[code]}
            interactive={interactive}
            selected={selectedCode === code}
            popoverVisible={popoverCode === code}
            onSelect={onSelect}
          />
        ))}
      </div>
    </div>
  );
}

function RoomSlot({ slot, item, interactive, selected, popoverVisible, onSelect }) {
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
          </div>
        )}
        {showLevelBadge ? <span className={`room-slot__level-badge room-slot__level-badge--${level}`}>{level}</span> : null}
        {popoverVisible ? <RoomSlotPopover slot={slot} item={item} /> : null}
      </button>
    </div>
  );
}

function RoomSlotPopover({ slot, item }) {
  const presentationMode = roomPresentationMode(slot, item);
  const explanation = excerpt(readState(item).explanation || "");
  const achievements = roomAchievementEntries(item).slice(0, 3);
  return (
    <div className={`room-slot__popover room-slot__popover--${slot.code}`}>
      <p className="room-slot__popover-skill">{slot.skill}</p>
      <strong>{slot.title}</strong>
      <span className="room-slot__popover-meta">{roomMetaLabel(slot, item)}</span>
      {presentationMode === "achievement_case" ? (
        achievements.length > 0 ? (
          <ul className="plain-list">
            {achievements.map((achievement) => (
              <li key={achievement.code || achievement.title}>
                <strong>{achievement.title}</strong>
                {achievement.description ? ` — ${achievement.description}` : ""}
              </li>
            ))}
          </ul>
        ) : (
          <p className="muted-copy">No trophies yet.</p>
        )
      ) : (
        <p className="muted-copy">{explanation || "Open the inspector for linked tasks and full evidence."}</p>
      )}
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

function RoomCustomizationPanel({ inventory, monetization, onEquip, busyAction, open, onToggle }) {
  const catalog = inventory?.catalog || [];
  const owned = new Set((inventory?.owned || []).map((item) => item.cosmetic_code));
  const equipped = Object.fromEntries((inventory?.equipped || []).map((item) => [item.slot_code, item.cosmetic_code]));
  const premiumEnabled = Boolean(monetization?.entitlements?.premium_cosmetics);
  const categories = [
    { key: "window_scene", label: "Window", subtitle: "Change the scene behind the window." },
    { key: "wall_style", label: "Walls", subtitle: "Swap the room wall finish without touching skill items." },
    { key: "floor_style", label: "Floor", subtitle: "Change the floor treatment for the room shell." },
    { key: "decor", label: "Decor", subtitle: "Add optional decorative props. These never affect trust or score." },
  ];

  return (
    <div className="room-customization">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Customize room</p>
          <h3>Visual-only cosmetics</h3>
        </div>
        <div className="room-customization__header-actions">
          <span className="meta-pill">{labelize(monetization?.plan?.name || "developer free")}</span>
          <button type="button" className="secondary-button" onClick={onToggle}>
            {open ? "Hide customization" : "Open customization"}
          </button>
        </div>
      </div>
      <p className="muted-copy room-customization__intro">
        Cosmetics only affect the visual shell. Skill items, score, confidence, and ranking stay unchanged.
      </p>

      {open ? (
        categories.map((category) => {
          const items = catalog.filter((item) => item.category === category.key);
          if (items.length === 0) {
            return null;
          }
          return (
            <section key={category.key} className="room-customization__section">
              <div className="room-customization__section-head">
                <div>
                  <p className="eyebrow">{category.label}</p>
                  <h4>{category.subtitle}</h4>
                </div>
              </div>
              <div className="room-customization__grid">
                {items.map((item) => {
                  const isOwned = owned.has(item.code);
                  const isEquipped = equipped[item.slot_code] === item.code;
                  const isLocked = !isOwned;
                  const actionBusy = busyAction === `equip:${item.code}`;
                  const buttonLabel = isEquipped ? "Equipped" : isLocked ? "Developer Plus" : "Equip";
                  return (
                    <div key={item.code} className={isEquipped ? "cosmetic-card cosmetic-card--active" : "cosmetic-card"}>
                      <div className={`cosmetic-card__preview cosmetic-card__preview--${sanitizeClass(item.code)}`}>
                        <span>{item.category.replaceAll("_", " ")}</span>
                      </div>
                      <div className="cosmetic-card__body">
                        <div className="cosmetic-card__meta">
                          <strong>{item.name}</strong>
                          <span className={item.premium ? "meta-pill meta-pill--premium" : "meta-pill"}>{item.premium ? "Plus" : "Included"}</span>
                        </div>
                        <p>{item.description}</p>
                        {isLocked && !premiumEnabled ? <p className="muted-copy">Upgrade to Developer Plus to unlock this cosmetic.</p> : null}
                        <button
                          type="button"
                          className={isEquipped ? "secondary-button" : "primary-button"}
                          onClick={() => onEquip(item.code)}
                          disabled={isEquipped || isLocked || actionBusy}
                        >
                          {actionBusy ? "Saving..." : buttonLabel}
                        </button>
                      </div>
                    </div>
                  );
                })}
              </div>
            </section>
          );
        })
      ) : (
        <div className="room-customization__collapsed">
          <p className="muted-copy">Customization is hidden by default. Open it when you want to switch the room shell, window scene, or decor.</p>
        </div>
      )}
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
