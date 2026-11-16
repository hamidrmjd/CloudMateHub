const http = require("node:http");
const fs = require("node:fs");
const { chromium } = require("playwright-core");

const handleSession = require("./session.cjs");

const mount = "/run/website-renderer";
const socket = mount + "/render.sock";
const broker = mount + "/fetch.sock";
let rendering = false;

function fetchResource(url) {
  return new Promise((resolve, reject) => {
    const req = http.get({ socketPath: broker, path: "/?url=" + encodeURIComponent(url), timeout: 65000 }, (res) => {
      const chunks = [];
      res.on("data", (part) => chunks.push(part));
      res.on("error", reject);
      res.on("end", () => resolve({ status: res.statusCode, headers: res.headers, body: Buffer.concat(chunks) }));
    });
    req.on("timeout", () => req.destroy(new Error("Resource timed out")));
    req.on("error", reject);
  });
}

async function capture(url) {
  const browser = await chromium.launch({ executablePath: "/usr/bin/chromium", args: ["--disable-background-networking", "--disable-quic"] });
  const deadline = setTimeout(() => void browser.close(), 90000);
  try {
    const context = await browser.newContext({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: 1,
      acceptDownloads: false, serviceWorkers: "block" });
    let requests = 0;
    let bytes = 0;
    await context.routeWebSocket("**/*", (ws) => ws.close());
    await context.route("**/*", async (route) => {
      const req = route.request();
      if (req.method() !== "GET" || !/^https?:\/\//i.test(req.url()) || ++requests > 160 || bytes > 32 * 1024 * 1024) {
        return route.abort();
      }
      try {
        const result = await fetchResource(req.url());
        bytes += result.body.length;
        if (bytes > 32 * 1024 * 1024) return route.abort();
        const headers = Object.fromEntries(Object.entries(result.headers).filter(([key]) =>
          ["content-type", "location", "access-control-allow-origin", "content-security-policy"].includes(key)));
        await route.fulfill({ status: result.status, headers, body: result.body });
      } catch {
        await route.abort().catch(() => {});
      }
    });
    const page = await context.newPage();
    const failures = [];
    page.on("response", (response) => {
      if (response.status() >= 400 && failures.length < 10) {
        failures.push(`${response.request().resourceType()} HTTP ${response.status()} ${new URL(response.url()).pathname}`);
      }
    });
    page.on("pageerror", (error) => {
      if (failures.length < 10) failures.push(error.name + ": " + error.message.slice(0, 150));
    });
    try {
      const response = await page.goto(url, { waitUntil: "commit", timeout: 25000 });
      if (!response?.ok()) throw new Error("Website navigation failed");
      await page.waitForFunction(() => document.body?.innerText.trim().length > 0, undefined, { timeout: 60000 });
      await page.waitForLoadState("networkidle", { timeout: 6000 }).catch(() => {});
    } catch (error) {
      console.error("Page did not finish rendering:", failures.join("; "));
      throw error;
    }
    return await page.screenshot({ type: "jpeg", quality: 80, fullPage: false, animations: "disabled", timeout: 5000 });
  } finally {
    clearTimeout(deadline);
    await browser.close();
  }
}

try { fs.unlinkSync(socket); } catch (err) { if (err.code !== "ENOENT") throw err; }
http.createServer(async (req, res) => {
  if (req.method === "POST" && req.url === "/session") return handleSession(req, res);
  const url = new URL(req.url, "http://localhost").searchParams.get("url");
  if (!url || !/^https?:\/\//i.test(url) || req.method !== "GET") {
    res.writeHead(400).end();
    return;
  }
  if (rendering) {
    res.writeHead(503).end();
    return;
  }
  rendering = true;
  try {
    const screenshot = await capture(url);
    res.writeHead(200, { "Content-Type": "image/jpeg", "Content-Length": screenshot.length }).end(screenshot);
  } catch (error) {
    console.error(error.message.split("\n", 1)[0]);
    res.writeHead(502).end();
  } finally {
    rendering = false;
  }
}).listen(socket, () => fs.chmodSync(socket, 0o666));
