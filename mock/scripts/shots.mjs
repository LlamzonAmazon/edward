import { chromium } from "playwright-core";

const out = process.argv[2] || "/tmp/shots";
const browser = await chromium.launch({ executablePath: "/usr/local/bin/google-chrome" });
const page = await browser.newPage({ viewport: { width: 1600, height: 960 }, deviceScaleFactor: 2 });
const open = ".dialog[data-open='true']";
const settle = () => page.waitForTimeout(700);
const shot = async (name) => {
  await page.mouse.move(2, 2);
  await settle();
  await page.screenshot({ path: `${out}/${name}.png` });
  console.log("saved", name);
};

await page.goto("http://127.0.0.1:5173/");
await page.waitForTimeout(900);
await shot("01-first-run");

await page.locator(`${open} .wipe`, { hasText: "Next" }).click();
await page.locator(`${open} .wipe`, { hasText: "Open System Settings" }).click();
await page.locator(`${open} .toggle[aria-label='New files']`).click();
await shot("12-onboarding-access");
await page.locator(`${open} .wipe`, { hasText: "Next" }).click();
await shot("13-onboarding-skill");
await page.locator(`${open} .wipe`, { hasText: "Write to" }).click();
await page.waitForTimeout(1400);
await shot("14-onboarding-ask");
await page.locator(`${open} .wipe`, { hasText: "Done" }).click();
await page.waitForTimeout(4600);
await shot("02-attack-surface");

await page.locator(".node--hub[aria-label='Network']").click();
await shot("03-node-open");

await page.locator(".sheet .wipe", { hasText: "Open finding" }).first().click();
await settle();
await page.locator(`${open} .wipe`, { hasText: "That’s my dev server" }).click();
await shot("04-finding-open");

await page.locator(`${open} .dialog__foot .wipe`, { hasText: "Clear" }).click();
await page.waitForTimeout(300);
await page.keyboard.press("Escape");
await page.locator(".nav__heading", { hasText: "Cleared" }).click();
await shot("05-cleared");

await page.waitForTimeout(4000);
await page.locator(".flag-label .wipe", { hasText: "invoice_0923" }).click();
await settle();
await page.locator(`${open} .wipe`, { hasText: "Ask your agent what this is" }).click();
await page.waitForTimeout(900);
await shot("06-with-your-agent");

await page.keyboard.press("Escape");
await page.waitForTimeout(4500);
await page.locator(".node--hub[aria-label='Firewall']").click();
await shot("07-uncollected-shelf");

await page.keyboard.press("Escape");
await settle();
await page.locator(".node--hub[aria-label='Running programs']").click();
await settle();
await page.locator(".menu__trigger").click();
await page.waitForTimeout(500);
await page.screenshot({ path: `${out}/08-menu-and-row.png` });
console.log("saved", "08-menu-and-row");

await page.keyboard.press("Escape");
await page.keyboard.press("Escape");
await settle();
await page.locator(".flag-label .wipe", { hasText: "com.fluxo.sync" }).hover();
await page.waitForTimeout(500);
await page.screenshot({ path: `${out}/09-graph-hover.png` });
console.log("saved", "09-graph-hover");

await page.keyboard.press("Escape");
await page.locator(".nav__foot .icon-btn").click();
await settle();
await shot("15-settings");
await page.locator(`${open} .row__head`, { hasText: "Notes for your agent" }).click();
await settle();
await page.locator("#context").fill("Port 3000 is always my dev server.\nIgnore anything in ~/scratch.");
await page.locator(`${open} .wipe`, { hasText: "Save for" }).click();
await shot("17-settings-agent-notes");
await page.locator(`${open} [aria-label='Back to settings']`).click();
await settle();
await page.locator(`${open} .row__head`, { hasText: "Notes on findings" }).click();
await shot("18-settings-finding-notes");
await page.locator(`${open} [aria-label='Back to settings']`).click();
await settle();
await page.locator(".dialog[data-open='true'] .toggle[aria-label='New files']").click();
await page.keyboard.press("Escape");
await settle();
await page.locator(".node--hub[aria-label='New files']").click();
await shot("16-new-files-off");

await page.goto("http://127.0.0.1:5173/?busy");
await page.waitForTimeout(900);
await page.keyboard.press("Escape");
await shot("10-many-raised");

await page.locator(".nav__more").click();
await shot("11-all-raised");

await browser.close();
