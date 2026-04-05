"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { apiFetch, loadAuth, loadRegionId, saveAuth, saveRegionId, setPreview } from "@/lib/client";
import { REGIONS } from "@/lib/preview-data";

const emptyRegister = {
  email: "",
  username: "",
  password: "",
  role: "user",
};

const emptyLogin = {
  email: "",
  password: "",
};

export default function LandingClient() {
  const router = useRouter();
  const [mode, setMode] = useState("register");
  const [regionId, setRegionId] = useState("americas");
  const [registerForm, setRegisterForm] = useState(emptyRegister);
  const [loginForm, setLoginForm] = useState(emptyLogin);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [hasSession, setHasSession] = useState(false);

  useEffect(() => {
    setRegionId(loadRegionId());
    setHasSession(Boolean(loadAuth()));
  }, []);

  const region = useMemo(
    () => REGIONS.find((item) => item.id === regionId) || REGIONS[0],
    [regionId],
  );

  async function handleSubmit(event) {
    event.preventDefault();
    setError("");
    setSubmitting(true);

    try {
      if (mode === "register") {
        const payload = await apiFetch("/v1/auth/register", {
          method: "POST",
          body: {
            email: registerForm.email,
            username: registerForm.username,
            password: registerForm.password,
            role: registerForm.role,
            country: region.country,
          },
        });
        saveAuth(payload);
      } else {
        const payload = await apiFetch("/v1/auth/login", {
          method: "POST",
          body: loginForm,
        });
        saveAuth(payload);
      }
      saveRegionId(region.id);
      setPreview(false);
      router.push("/workspace");
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : "Unable to continue");
    } finally {
      setSubmitting(false);
    }
  }

  function launchPreview() {
    saveRegionId(region.id);
    setPreview(true);
    router.push("/workspace");
  }

  return (
    <main className="landing-shell">
      <section className="landing-scene">
        <div className="brand-chip">SkillRoom</div>
        <div className="scene-copy">
          <p className="eyebrow">Room-linked skill signals</p>
          <h1>See the stack you actually built.</h1>
          <p>
            Real React challenges, deterministic variants, runner-backed execution, and a room that changes with real skill data.
          </p>
        </div>
        <div className="region-picker">
          <span>Region</span>
          <div className="region-grid">
            {REGIONS.map((item) => (
              <button
                key={item.id}
                type="button"
                className={item.id === region.id ? "region-pill active" : "region-pill"}
                onClick={() => {
                  setRegionId(item.id);
                  saveRegionId(item.id);
                }}
              >
                {item.label}
              </button>
            ))}
          </div>
        </div>
        <div className="room-stage room-stage--landing">
          <div className="room-wall room-wall--left" />
          <div className="room-wall room-wall--right" />
          <div className="room-floor" />
          <div className="room-window">
            <span />
          </div>
          <div className="room-shelf">
            <span />
            <span />
            <span />
          </div>
          <div className="room-desk">
            <div className="room-monitor">
              <span />
            </div>
          </div>
          <div className="room-plant">
            <span />
          </div>
          <div className="room-chair" />
          <div className="room-rug" />
          <div className="room-trophy" />
        </div>
      </section>

      <section className="landing-panel">
        <div className="landing-header">
          <p className="eyebrow">Launch workspace</p>
          <h2>{mode === "register" ? "Create your account" : "Welcome back"}</h2>
          <p>
            {mode === "register"
              ? "Join as a candidate or recruiter. Region sets your onboarding locale and leaderboard context."
              : "Sign in to continue with live challenge execution and room updates."}
          </p>
        </div>

        <div className="mode-switch">
          <button type="button" className={mode === "register" ? "mode-button active" : "mode-button"} onClick={() => setMode("register")}>
            Create account
          </button>
          <button type="button" className={mode === "login" ? "mode-button active" : "mode-button"} onClick={() => setMode("login")}>
            Sign in
          </button>
        </div>

        <form className="auth-form" onSubmit={handleSubmit}>
          {mode === "register" ? (
            <>
              <div className="segmented-row">
                <button
                  type="button"
                  className={registerForm.role === "user" ? "mode-button active" : "mode-button"}
                  onClick={() => setRegisterForm((current) => ({ ...current, role: "user" }))}
                >
                  Candidate
                </button>
                <button
                  type="button"
                  className={registerForm.role === "hr" ? "mode-button active" : "mode-button"}
                  onClick={() => setRegisterForm((current) => ({ ...current, role: "hr" }))}
                >
                  Recruiter
                </button>
              </div>
              <label className="form-row">
                <span>Username</span>
                <input
                  type="text"
                  value={registerForm.username}
                  onChange={(event) => setRegisterForm((current) => ({ ...current, username: event.target.value }))}
                  placeholder="fletcher-room"
                  required
                />
              </label>
              <label className="form-row">
                <span>Email</span>
                <input
                  type="email"
                  value={registerForm.email}
                  onChange={(event) => setRegisterForm((current) => ({ ...current, email: event.target.value }))}
                  placeholder="you@skillroom.dev"
                  required
                />
              </label>
              <label className="form-row">
                <span>Password</span>
                <input
                  type="password"
                  value={registerForm.password}
                  onChange={(event) => setRegisterForm((current) => ({ ...current, password: event.target.value }))}
                  placeholder="Create a password"
                  required
                />
              </label>
            </>
          ) : (
            <>
              <label className="form-row">
                <span>Email</span>
                <input
                  type="email"
                  value={loginForm.email}
                  onChange={(event) => setLoginForm((current) => ({ ...current, email: event.target.value }))}
                  placeholder="you@skillroom.dev"
                  required
                />
              </label>
              <label className="form-row">
                <span>Password</span>
                <input
                  type="password"
                  value={loginForm.password}
                  onChange={(event) => setLoginForm((current) => ({ ...current, password: event.target.value }))}
                  placeholder="Enter your password"
                  required
                />
              </label>
            </>
          )}

          {error ? <p className="inline-error">{error}</p> : null}

          <button type="submit" className="primary-button" disabled={submitting}>
            {submitting ? "Opening workspace..." : mode === "register" ? "Create account" : "Sign in"}
          </button>
        </form>

        <div className="preview-panel">
          <div>
            <strong>Explore without signing in</strong>
            <p>Open a guided preview of the challenge room, rankings, and recruiter view before creating an account.</p>
          </div>
          <button type="button" className="secondary-button" onClick={launchPreview}>
            Explore product preview
          </button>
        </div>

        {hasSession ? (
          <div className="session-banner">
            <span>Existing session found.</span>
            <Link href="/workspace">Open workspace</Link>
          </div>
        ) : null}
      </section>
    </main>
  );
}
