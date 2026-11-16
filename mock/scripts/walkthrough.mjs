import { chromium } from "playwright-core";
const url = process.env.MOCK_URL ?? "http://127.0.0.1:5173/";

const dir = process.argv[2] || "/tmp/video";
const size = { width: 1440, height: 900 };
const browser = await chromium.launch({ executablePath: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" });
const context = await browser.newContext({ viewport: size, recordVideo: { dir, size } });
// Fixture data only: a running EDwaRd answering /api would put this Mac in the shots.
await context.route("**/api/**", (r) => r.abort());
await context.addInitScript(() => {
  window.addEventListener("DOMContentLoaded", () => {
    const c = document.createElement("div");
    c.style.cssText =
      "position:fixed;left:0;top:0;width:22px;height:22px;margin:-11px 0 0 -11px;border-radius:50%;background:rgb(18 19 24 / .35);box-shadow:0 0 0 3px #fff;z-index:9999;pointer-events:none;transition:transform 120ms";
    document.body.appendChild(c);
    addEventListener("mousemove", (e) => (c.style.left = e.clientX + "px", c.style.top = e.clientY + "px"));
    addEventListener("mousedown", () => (c.style.transform = "scale(.7)"));
    addEventListener("mouseup", () => (c.style.transform = "none"));
  });
});
const page = await context.newPage();
const wait = (ms) => page.waitForTimeout(ms);
let pos = { x: 720, y: 450 };
const go = async (loc, { click = true, pause = 500 } = {}) => {
  const b = await loc.first().boundingBox();
  const x = b.x + b.width / 2, y = b.y + b.height / 2;
  const steps = Math.max(12, Math.round(Math.hypot(x - pos.x, y - pos.y) / 18));
  await page.mouse.move(x, y, { steps });
  pos = { x, y };
  await wait(pause);
  if (click) {
    await page.mouse.down();
    await wait(90);
    await page.mouse.up();
  }
};

const open = ".dialog[data-open='true']";
await page.goto(url);
await page.mouse.move(pos.x, pos.y);
await wait(1800);

// First run starts with no agent picked.
await go(page.locator(`${open} .toggle[aria-label='Claude Code']`));
await go(page.locator(`${open} .toggle[aria-label='Codex']`));
await wait(600);
await go(page.locator(`${open} .wipe`, { hasText: "Next" }));
await wait(1400);
await go(page.locator(`${open} .wipe`, { hasText: "Open System Settings" }));
await wait(1200);
await go(page.locator(`${open} .toggle[aria-label='New files']`));
await wait(1400);
await go(page.locator(`${open} .wipe`, { hasText: "Next" }));
await wait(1800);
await page.mouse.wheel(0, 300);
await wait(1000);
await go(page.locator(`${open} .wipe`, { hasText: "Write to" }));
await wait(2000);
await go(page.locator(`${open} .field .wipe`).first());
await wait(1000);
await go(page.locator(`${open} .wipe`, { hasText: "Done" }));
await wait(1600);

await go(page.locator(".flag-label .wipe", { hasText: "com.fluxo.sync" }), { click: false, pause: 1800 });
await go(page.locator(".node--hub[aria-label='Network']"));
await wait(1600);
await go(page.locator(".menu__trigger"));
await wait(900);
await go(page.locator(".menu__item", { hasText: "Raised first" }));
await wait(700);
await go(page.locator(".sheet .wipe", { hasText: "Open finding" }));
await wait(2000);
await go(page.locator(`${open} .wipe`, { hasText: "That’s my dev server" }));
await wait(1400);
await go(page.locator(`${open} .dialog__foot .wipe`, { hasText: "Clear" }));
await wait(1600);
await go(page.locator(".sheet .icon-btn"));
await wait(1800);

await go(page.locator(".flag-label .wipe", { hasText: "invoice_0923" }));
await wait(2000);
await go(page.locator(`${open} .wipe`, { hasText: "Ask your agent what this is" }));
await wait(2400);
await go(page.locator(`${open} .dialog__head .icon-btn`));
await wait(1600);

await go(page.locator(".nav__foot .icon-btn"));
await wait(1200);
await go(page.locator(`${open} .row__head`, { hasText: "Notes for your agent" }));
await wait(1000);
await go(page.locator("#context"));
await page.keyboard.type("Port 3000 is always my dev server.", { delay: 35 });
await wait(500);
await go(page.locator(`${open} .wipe`, { hasText: "Save for" }));
await wait(1400);
await go(page.locator(`${open} [aria-label='Back to settings']`));
await wait(1000);
await go(page.locator(`${open} .row__head`, { hasText: "Notes on findings" }));
await wait(2000);
await go(page.locator(`${open} [aria-label='Back to settings']`));
await wait(1000);
await go(page.locator(`${open} .dialog__head .icon-btn`));
await wait(1200);

await go(page.locator(".node--hub[aria-label='Firewall']"));
await wait(2400);
await go(page.locator(".sheet .icon-btn"));
await wait(1500);

const video = page.video();
await context.close();
console.log(await video.path());
await browser.close();
