import { expect, test } from "@playwright/test";

test("candidate session survives reload without localStorage auth", async ({ page }) => {
  const unique = Date.now();

  await page.goto("/");
  await page.getByLabel("Username").fill(`candidate-${unique}`);
  await page.getByLabel("Email").fill(`candidate-${unique}@skillroom.dev`);
  await page.getByLabel("Password").fill("password123");
  await page.getByRole("button", { name: "Create account" }).click();

  await page.waitForURL("**/workspace");
  await expect(page.getByRole("button", { name: "Challenges" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Ranking" })).toBeVisible();

  const storedAuth = await page.evaluate(() => window.localStorage.getItem("skillroom.auth"));
  expect(storedAuth).toBeNull();

  await page.reload();
  await page.waitForURL("**/workspace");
  await expect(page.getByRole("button", { name: "Challenges" })).toBeVisible();

  await page.getByRole("button", { name: "Sign out" }).click();
  await page.waitForURL("**/");
});

test("recruiter workspace stays role-scoped after auth bootstrap", async ({ page }) => {
  const unique = Date.now() + 1;

  await page.goto("/");
  await page.getByRole("button", { name: "Recruiter" }).click();
  await page.getByLabel("Username").fill(`hr-${unique}`);
  await page.getByLabel("Email").fill(`hr-${unique}@skillroom.dev`);
  await page.getByLabel("Password").fill("password123");
  await page.getByRole("button", { name: "Create account" }).click();

  await page.waitForURL("**/workspace");
  await expect(page.getByRole("button", { name: "Candidates" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Leaderboard" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Challenges" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Room" })).toHaveCount(0);

  await page.reload();
  await page.waitForURL("**/workspace");
  await expect(page.getByRole("button", { name: "Candidates" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Leaderboard" })).toBeVisible();
});
