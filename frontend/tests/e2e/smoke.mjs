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
    await candidateFlow(browser);
    await recruiterFlow(browser);
    console.log("browser smoke passed");
  } finally {
    await browser.close();
  }
}

async function candidateFlow(browser) {
  const page = await browser.newPage({ baseURL });
  const unique = Date.now();
  const authForm = page.locator("form.auth-form");

  await page.goto("/");
  await page.getByLabel("Username").fill(`candidate-core-${unique}`);
  await page.getByLabel("Email").fill(`candidate-core-${unique}@skillroom.dev`);
  await page.getByLabel("Password").fill("password123");
  await authForm.getByRole("button", { name: "Create account" }).click();

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

async function recruiterFlow(browser) {
  const unique = Date.now() + 1000;
  const candidateEmail = `candidate-hr-${unique}@skillroom.dev`;
  const candidateUsername = `candidate-hr-${unique}`;

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
  const candidateUserID = candidatePayload.user.id;

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
  await page.getByTestId(`leaderboard-open-${candidateUserID}`).waitFor({ state: "visible", timeout: 30_000 });
  await page.getByTestId(`leaderboard-open-${candidateUserID}`).click();

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

  await page.close();
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
