import { promises as fs } from "node:fs";
import path from "node:path";
import { spawn } from "node:child_process";

const workspace = process.env.SKILLROOM_WORKSPACE || "/workspace";
const reportDir = path.join(workspace, ".skillroom");
const manifestPath = path.join(workspace, "skillroom-run.json");
const vitestReportPath = path.join(reportDir, "vitest-report.json");
const eslintReportPath = path.join(reportDir, "eslint-report.json");

await fs.mkdir(reportDir, { recursive: true });
await ensureNodeModulesSymlink(workspace);

const manifest = JSON.parse(await fs.readFile(manifestPath, "utf8"));
const errors = [];

const lint = await runLint(manifest.lint_files || [], eslintReportPath, errors);
const { report, durationMs } = await runVitest(vitestReportPath, errors);

const testResults = flattenVitestResults(report);
const correctnessResults = testResults.filter((result) => result.kind !== "quality");
const qualityResults = testResults.filter((result) => result.kind === "quality");
const output = {
  test_results: testResults,
  passed: correctnessResults.filter((result) => result.passed).length,
  failed: correctnessResults.filter((result) => !result.passed).length,
  visible_passed: correctnessResults.filter((result) => result.passed && !result.hidden).length,
  visible_failed: correctnessResults.filter((result) => !result.passed && !result.hidden).length,
  hidden_passed: correctnessResults.filter((result) => result.passed && result.hidden).length,
  hidden_failed: correctnessResults.filter((result) => !result.passed && result.hidden).length,
  quality_passed: qualityResults.filter((result) => result.passed).length,
  quality_failed: qualityResults.filter((result) => !result.passed).length,
  execution_cost_ms: durationMs,
  execution_time_ms: durationMs,
  errors,
  lint,
};

process.stdout.write(`${JSON.stringify(output)}\n`);

async function ensureNodeModulesSymlink(root) {
  const target = path.join(root, "node_modules");
  try {
    await fs.lstat(target);
  } catch {
    await fs.symlink("/opt/skillroom-runtime/node_modules", target, "dir");
  }
}

async function runLint(files, reportPath, errors) {
  if (files.length === 0) {
    return { error_count: 0, warning_count: 0, messages: [] };
  }

  const eslintBin = path.join("/opt/skillroom-runtime/node_modules", ".bin", "eslint");
  const args = [...files, "--format", "json", "--output-file", reportPath, "--no-warn-ignored"];
  const { code, stderr } = await runProcess(eslintBin, args, { cwd: workspace });
  if (stderr.trim() !== "") {
    errors.push(stderr.trim());
  }
  if (code > 1) {
    errors.push(`eslint exited with code ${code}`);
  }

  let report = [];
  try {
    report = JSON.parse(await fs.readFile(reportPath, "utf8"));
  } catch {
    report = [];
  }

  const messages = [];
  let errorCount = 0;
  let warningCount = 0;
  for (const entry of report) {
    errorCount += entry.errorCount || 0;
    warningCount += entry.warningCount || 0;
    for (const message of entry.messages || []) {
      messages.push({
        file: relativePath(entry.filePath || ""),
        rule_id: message.ruleId || "",
        severity: message.severity === 2 ? "error" : "warning",
        message: message.message || "",
      });
    }
  }

  return {
    error_count: errorCount,
    warning_count: warningCount,
    messages,
  };
}

async function runVitest(reportPath, errors) {
  const vitestBin = path.join("/opt/skillroom-runtime/node_modules", ".bin", "vitest");
  const startedAt = Date.now();
  const { code, stderr } = await runProcess(vitestBin, ["run", "--reporter=json", "--outputFile", reportPath], { cwd: workspace });
  const durationMs = Date.now() - startedAt;

  if (stderr.trim() !== "") {
    errors.push(stderr.trim());
  }
  if (code > 1) {
    errors.push(`vitest exited with code ${code}`);
  }

  try {
    const report = JSON.parse(await fs.readFile(reportPath, "utf8"));
    return { report, durationMs };
  } catch (error) {
    errors.push(`unable to read vitest report: ${error.message}`);
    return { report: { testResults: [] }, durationMs };
  }
}

function flattenVitestResults(report) {
  const results = [];
  for (const suite of report.testResults || []) {
    const hidden = `${suite.name || ""}`.includes("hidden.spec.");
    for (const assertion of suite.assertionResults || []) {
      const metadata = parseAssertionMetadata(assertion.fullName || assertion.title || "unnamed test");
      results.push({
        file: relativePath(suite.name || ""),
        name: metadata.name,
        check_id: metadata.checkId,
        kind: metadata.kind,
        passed: assertion.status === "passed",
        hidden,
        duration_ms: assertion.duration || 0,
        error: assertion.status === "passed" ? "" : formatFailure(assertion.failureMessages || []),
      });
    }
  }
  return results;
}

function parseAssertionMetadata(name) {
  const raw = String(name || "unnamed test");
  const match = raw.match(/\[(quality)(?::([a-z0-9_-]+))?\]/i);
  if (!match) {
    return {
      kind: "correctness",
      checkId: "",
      name: raw.trim(),
    };
  }

  const cleaned = raw.replace(match[0], "").replace(/\s+/g, " ").trim();
  return {
    kind: "quality",
    checkId: match[2] || normalizeCheckId(cleaned),
    name: cleaned,
  };
}

function normalizeCheckId(name) {
  return String(name || "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function formatFailure(messages) {
  if (!Array.isArray(messages) || messages.length === 0) {
    return "";
  }
  return String(messages[0]).trim();
}

function relativePath(filePath) {
  if (!filePath) {
    return "";
  }
  return filePath.startsWith(workspace) ? path.relative(workspace, filePath) : filePath;
}

function runProcess(command, args, options) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      ...options,
      env: {
        ...process.env,
        NODE_ENV: "test",
      },
      stdio: ["ignore", "ignore", "pipe"],
    });

    let stderr = "";
    child.stderr.on("data", (chunk) => {
      stderr += chunk.toString();
    });
    child.on("error", reject);
    child.on("close", (code) => {
      resolve({
        code: code ?? 1,
        stderr,
      });
    });
  });
}
