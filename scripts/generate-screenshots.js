#!/usr/bin/env node
// Generate screenshots of the ScanFlow Web UI for documentation.
// Usage: node scripts/generate-screenshots.js
//
// Starts a mock server that serves the Web UI with realistic sample data,
// then captures screenshots using Playwright.

const http = require("http");
const fs = require("fs");
const path = require("path");

const PORT = 0; // auto-assign
const WEB_DIR = path.join(__dirname, "..", "server", "web");
const OUT_DIR = path.join(__dirname, "..", "docs", "screenshots");

// MIME types for static files
const MIME = {
  ".html": "text/html",
  ".css": "text/css",
  ".js": "application/javascript",
  ".png": "image/png",
  ".svg": "image/svg+xml",
};

// Mock API responses that populate the UI with realistic data
const MOCK_ROUTES = {
  "GET /api/v1/status": {
    status: "ok",
    version: "1.0.0",
    scanner: true,
    devices: 1,
    active_jobs: 1,
    total_jobs: 3,
  },
  "GET /api/v1/scanner/devices": {
    devices: [
      {
        name: "fujitsu:ScanSnap iX1600:12345",
        vendor: "FUJITSU",
        model: "ScanSnap iX1600",
        type: "scanner",
      },
    ],
  },
  "GET /api/v1/profiles": {
    profiles: [
      {
        profile: { name: "Standard", description: "Color 300 DPI duplex" },
      },
      {
        profile: { name: "Oversize", description: "Unlimited page height" },
      },
      { profile: { name: "Photo", description: "600 DPI high quality" } },
    ],
  },
  "GET /api/v1/settings": {
    ocr_enabled: true,
    ocr_language: "deu+eng",
  },
};

function serveMockAPI(req, res) {
  const key = req.method + " " + req.url.split("?")[0];
  const mock = MOCK_ROUTES[key];
  if (mock) {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify(mock));
    return true;
  }
  return false;
}

function serveStatic(req, res) {
  let urlPath = req.url.split("?")[0];
  if (urlPath === "/") urlPath = "/index.html";

  let filePath;
  if (urlPath === "/index.html") {
    filePath = path.join(WEB_DIR, "templates", "index.html");
  } else if (urlPath.startsWith("/static/")) {
    filePath = path.join(WEB_DIR, urlPath);
  } else {
    res.writeHead(404);
    res.end("Not found");
    return;
  }

  // Prevent path traversal
  const resolved = path.resolve(filePath);
  if (!resolved.startsWith(path.resolve(WEB_DIR))) {
    res.writeHead(403);
    res.end("Forbidden");
    return;
  }

  if (!fs.existsSync(resolved)) {
    res.writeHead(404);
    res.end("Not found");
    return;
  }

  const ext = path.extname(resolved);
  res.writeHead(200, { "Content-Type": MIME[ext] || "application/octet-stream" });
  fs.createReadStream(resolved).pipe(res);
}

async function captureScreenshots(baseURL) {
  // Dynamic import for ES module
  const { chromium } = require("playwright");

  fs.mkdirSync(OUT_DIR, { recursive: true });

  const browser = await chromium.launch();
  const context = await browser.newContext({
    viewport: { width: 1280, height: 900 },
    deviceScaleFactor: 2,
  });
  const page = await context.newPage();

  await page.goto(baseURL, { waitUntil: "networkidle" });

  // Wait a moment for any JS rendering to settle
  await page.waitForTimeout(500);

  // Inject a mock job card so the Jobs section looks realistic
  await page.evaluate(() => {
    /* global addJobCard */
    if (typeof addJobCard === "function") {
      addJobCard({
        id: "550e8400-e29b-41d4-a716-446655440000",
        status: "scanning",
        profile: "Standard",
        pages: [{ number: 1 }, { number: 2 }],
        progress: 65,
      });
      addJobCard({
        id: "7c9e6679-7425-40de-944b-e07fc1f90ae7",
        status: "completed",
        profile: "Standard",
        pages: [{ number: 1 }, { number: 2 }, { number: 3 }],
        progress: 100,
      });
    }
  });

  await page.waitForTimeout(300);

  // Full-page screenshot
  await page.screenshot({
    path: path.join(OUT_DIR, "web-ui.png"),
    fullPage: true,
  });
  console.log("Saved docs/screenshots/web-ui.png");

  // Scan section screenshot
  const scanSection = page.locator("#scan-section");
  if (await scanSection.isVisible()) {
    await scanSection.screenshot({
      path: path.join(OUT_DIR, "web-ui-scan.png"),
    });
    console.log("Saved docs/screenshots/web-ui-scan.png");
  }

  // Jobs section screenshot
  const jobsSection = page.locator("#jobs-section");
  if (await jobsSection.isVisible()) {
    await jobsSection.screenshot({
      path: path.join(OUT_DIR, "web-ui-jobs.png"),
    });
    console.log("Saved docs/screenshots/web-ui-jobs.png");
  }

  await browser.close();
}

async function main() {
  const server = http.createServer((req, res) => {
    if (req.url.startsWith("/api/")) {
      if (!serveMockAPI(req, res)) {
        res.writeHead(404, { "Content-Type": "application/json" });
        res.end(JSON.stringify({ error: "not found" }));
      }
    } else {
      serveStatic(req, res);
    }
  });

  await new Promise((resolve) => server.listen(PORT, "127.0.0.1", resolve));
  const addr = server.address();
  const baseURL = `http://127.0.0.1:${addr.port}`;
  console.log(`Mock server listening on ${baseURL}`);

  try {
    await captureScreenshots(baseURL);
    console.log("Screenshots generated successfully.");
  } finally {
    server.close();
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
