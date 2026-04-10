import assert from "node:assert/strict";
import { chromium } from "@playwright/test";

const baseURL = process.env.PLAYWRIGHT_BASE_URL || "http://127.0.0.1:3000";

const resizeOldBlock = `  useEffect(() => {
    window.addEventListener("resize", () => {
      setWidth(window.innerWidth);
    });
  }, [width]);
`;

const resizeNewBlock = `  useEffect(() => {
    function handleResize() {
      setWidth(window.innerWidth);
    }

    window.addEventListener("resize", handleResize);
    return () => {
      window.removeEventListener("resize", handleResize);
    };
  }, []);
`;

async function run() {
  const browser = await chromium.launch({ headless: true });
  try {
    const unique = Date.now();
    const seededCandidates = await Promise.all(
      [0, 1, 2, 3, 4, 5, 6, 7, 8].map((index) => registerCandidateByAPI(unique, index)),
    );
    await candidateFlow(browser, seededCandidates[0]);
    await refreshFailureFlow(browser);
    await recruiterFlow(browser, unique + 1000, seededCandidates);
    await workspaceDegradedFlow(browser, seededCandidates[1]);
    await runnerDegradedFlow(browser, seededCandidates[2]);
    console.log("browser smoke passed");
  } finally {
    await browser.close();
  }
}

async function candidateFlow(browser, candidate) {
  const page = await browser.newPage({ baseURL });
  await signInCandidateViaUI(page, candidate.email);

  await page.waitForURL("**/workspace");
  await waitForWorkspaceReady(page, page.getByTestId("skill-score"));
  await page.getByTestId("nav-challenges").click({ force: true });
  await page.getByText("Task bank", { exact: true }).first().waitFor({ state: "visible", timeout: 30_000 });
  await waitForWorkspaceIdle(page);
  await page.locator('[data-testid^="template-"]').first().waitFor({ state: "visible", timeout: 30_000 });
  await page.getByTestId("template-react_debug_resize_cleanup").click();

  const editor = page.getByTestId("challenge-editor");
  await editor.waitFor({ state: "visible", timeout: 30_000 });
  const starter = await editor.inputValue();
  const patched = starter.replace(resizeOldBlock, resizeNewBlock);
  assert.notEqual(patched, starter, "expected to patch resize challenge starter code");
  await editor.fill(patched);

  await page.getByTestId("run-checks").click();
  await page.getByText("Cost ms").waitFor({ state: "visible", timeout: 90_000 });

  await page.getByTestId("submit-solution").click();
  await page.getByText("Final").waitFor({ state: "visible", timeout: 90_000 });

  await page.getByTestId("nav-room").click();
  await page.getByTestId("room-slot-monitor").click();
  const inspectorText = await page.getByTestId("room-inspector").innerText({ timeout: 30_000 });
  assert.match(inspectorText, /Fix Resize Listener Cleanup/i, "expected room inspector to include linked challenge evidence");

  await page.close();
}

async function recruiterFlow(browser, unique, seededCandidates) {
  const page = await browser.newPage({ baseURL });
  const authForm = page.locator("form.auth-form");
  await page.goto("/");
  await page.getByRole("button", { name: "Recruiter" }).click();
  await page.getByLabel("Username").fill(`hr-core-${unique}`);
  await page.getByLabel("Email").fill(`hr-core-${unique}@skillroom.dev`);
  await page.getByLabel("Password").fill("password123");
  await authForm.getByRole("button", { name: "Create account" }).click();

  await page.waitForURL("**/workspace");
  await waitForWorkspaceReady(page, page.getByRole("button", { name: "Apply filters" }));
  await page.getByTestId("nav-leaderboard").click({ force: true });
  await page.getByText("Candidate leaderboard", { exact: true }).first().waitFor({ state: "visible", timeout: 30_000 });
  await waitForWorkspaceIdle(page);
  await openLeaderboardCandidateByIndex(page, 0);

  await page.getByTestId("candidate-detail").waitFor({ state: "visible", timeout: 30_000 });
  await page.getByTestId("candidate-unlock").click();
  await page.getByTestId("candidate-open-room").waitFor({ state: "visible", timeout: 30_000 });

  await page.getByTestId("candidate-open-room").click();
  await page.getByTestId("room-inspector").waitFor({ state: "visible", timeout: 30_000 });

  await page.getByTestId("candidate-invite").click();
  await page.waitForFunction(() => {
    const action = document.querySelector('[data-testid="candidate-invite"]');
    return Boolean(action && /Invited/i.test(action.textContent || ""));
  }, { timeout: 30_000 });
  const inviteText = await page.getByTestId("candidate-invite").innerText();
  assert.match(inviteText, /Invited/i, "expected candidate invite action to complete");
  await page.getByRole("button", { name: "Back to leaderboard" }).click();
  await page.getByText("Candidate leaderboard", { exact: true }).first().waitFor({ state: "visible", timeout: 30_000 });

  await openLeaderboardCandidateByIndex(page, 1);
  await page.getByTestId("candidate-unlock").click();
  await page.getByTestId("candidate-invite").waitFor({ state: "visible", timeout: 30_000 });
  await page.getByTestId("candidate-invite").click();
  await page.waitForFunction(() => {
    const action = document.querySelector('[data-testid="candidate-invite"]');
    return Boolean(action && /Invited/i.test(action.textContent || ""));
  }, { timeout: 30_000 });

  await openLeaderboardCandidateByIndex(page, 2);
  await page.getByTestId("candidate-unlock").click();
  await page.getByTestId("candidate-invite").waitFor({ state: "visible", timeout: 30_000 });
  await expectButtonState(page.getByTestId("candidate-invite"), /Invite limit reached/i, true);

  await openLeaderboardCandidateByIndex(page, 3);
  await page.getByTestId("candidate-unlock").waitFor({ state: "visible", timeout: 30_000 });
  await expectButtonState(page.getByTestId("candidate-unlock"), /Unlock limit reached/i, true);

  await page.close();
}

async function refreshFailureFlow(browser) {
  const page = await browser.newPage({ baseURL });
  await page.context().addCookies([
    {
      name: "skillroom_refresh",
      value: "rfr_invalid",
      url: baseURL,
      httpOnly: true,
      sameSite: "Lax",
    },
  ]);

  await page.goto("/workspace");
  await page.getByRole("heading", { name: "No active workspace" }).waitFor({ state: "visible", timeout: 30_000 });
  const bodyText = await page.locator("main").innerText();
  assert.match(bodyText, /sign in/i, "expected refresh failure to fall back to the signed-out workspace state");
  await page.close();
}

async function workspaceDegradedFlow(browser, candidate) {
  const page = await browser.newPage({ baseURL });
  await signInCandidateViaUI(page, candidate.email);
  await page.waitForURL("**/workspace");
  await waitForWorkspaceReady(page, page.getByTestId("skill-score"));

  await page.route("**/backend/v1/me", async (route) => {
    await route.fulfill({
      status: 503,
      contentType: "application/json",
      body: JSON.stringify({ error: "workspace bootstrap unavailable" }),
    });
  });

  await page.goto("/workspace");
  await page.getByText(/workspace bootstrap unavailable/i).waitFor({ state: "visible", timeout: 30_000 });
  await page.close();
}

async function runnerDegradedFlow(browser, candidate) {
  const page = await browser.newPage({ baseURL });
  await signInCandidateViaUI(page, candidate.email);
  await page.waitForURL("**/workspace");
  await waitForWorkspaceReady(page, page.getByTestId("skill-score"));
  await page.getByTestId("nav-challenges").click({ force: true });
  await page.getByText("Task bank", { exact: true }).first().waitFor({ state: "visible", timeout: 30_000 });
  await waitForWorkspaceIdle(page);
  await page.locator('[data-testid^="template-"]').first().waitFor({ state: "visible", timeout: 30_000 });
  await page.getByTestId("template-react_debug_resize_cleanup").click();

  const editor = page.getByTestId("challenge-editor");
  await editor.waitFor({ state: "visible", timeout: 30_000 });
  const starter = await editor.inputValue();
  const patched = starter.replace(resizeOldBlock, resizeNewBlock);
  assert.notEqual(patched, starter, "expected to patch resize challenge starter code");
  await editor.fill(patched);

  await page.route("**/backend/v1/challenges/instances/**/runs", async (route) => {
    await route.fulfill({
      status: 503,
      contentType: "application/json",
      body: JSON.stringify({ error: "runner unavailable: smoke" }),
    });
  });

  await page.getByTestId("run-checks").click();
  await page.getByText(/runner unavailable/i).waitFor({ state: "visible", timeout: 30_000 });
  await page.close();
}

async function registerCandidateByAPI(unique, index) {
  const candidateEmail = `candidate-hr-${unique}-${index}@skillroom.dev`;
  const candidateUsername = `candidate-hr-${unique}-${index}`;
  const registerResponse = await fetch(`${baseURL}/backend/v1/auth/register`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      email: candidateEmail,
      username: candidateUsername,
      password: "password123",
      country: "US",
      role: "user",
    }),
  });
  assert.equal(registerResponse.status, 201, "expected candidate registration for recruiter smoke");
  const candidatePayload = await registerResponse.json();
  return { email: candidateEmail, username: candidateUsername, userID: candidatePayload.user.id };
}

async function registerCandidateViaUI(page, username, email) {
  const authForm = page.locator("form.auth-form");
  await page.goto("/");
  await page.getByLabel("Username").fill(username);
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill("password123");
  await authForm.getByRole("button", { name: "Create account" }).click();
}

async function signInCandidateViaUI(page, email) {
  const authForm = page.locator("form.auth-form");
  await page.goto("/");
  await page.getByRole("button", { name: "Sign in" }).first().click();
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill("password123");
  await authForm.getByRole("button", { name: "Sign in" }).click();
}

async function openLeaderboardCandidateByIndex(page, targetIndex) {
  await page.getByTestId("nav-leaderboard").click({ force: true });
  await page.getByText("Candidate leaderboard", { exact: true }).first().waitFor({ state: "visible", timeout: 30_000 });
  await waitForWorkspaceIdle(page);
  const limit = 8;
  const targetPageOffset = Math.floor(targetIndex / limit) * limit;
  for (;;) {
    const currentPageLabel = await page.locator(".pagination-row p").first().textContent().catch(() => "");
    const match = currentPageLabel && currentPageLabel.match(/Showing\s+(\d+)-(\d+)\s+of\s+(\d+)/i);
    const start = match ? Number(match[1]) - 1 : 0;
    if (start === targetPageOffset || !match) {
      break;
    }
    const previousPage = page.getByTestId("leaderboard-pagination-prev");
    if (!(await previousPage.count()) || await previousPage.isDisabled()) {
      break;
    }
    await previousPage.click();
    await waitForWorkspaceIdle(page);
  }

  const rowIndex = targetIndex % limit;
  const openCandidate = page.locator('[data-testid^="leaderboard-open-"]').nth(rowIndex);
  await openCandidate.waitFor({ state: "visible", timeout: 30_000 });
  await openCandidate.click();
  const detail = page.getByTestId("candidate-detail");
  await detail.waitFor({ state: "visible", timeout: 30_000 });
  await detail.locator("h3").first().waitFor({ state: "visible", timeout: 30_000 });
}

async function expectButtonState(locator, label, disabled) {
  await locator.waitFor({ state: "visible", timeout: 30_000 });
  const text = await locator.innerText();
  assert.match(text, label);
  assert.equal(await locator.isDisabled(), disabled, `expected ${text} disabled=${disabled}`);
}

async function waitForWorkspaceIdle(page) {
  const loadingBar = page.locator(".loading-bar");
  if (await loadingBar.count()) {
    await loadingBar.waitFor({ state: "detached", timeout: 30_000 }).catch(() => {});
  }
}

async function waitForWorkspaceReady(page, readyLocator) {
  await readyLocator.waitFor({ state: "visible", timeout: 30_000 });
  await waitForWorkspaceIdle(page);
}

run().catch((error) => {
  console.error(error);
  process.exit(1);
});
