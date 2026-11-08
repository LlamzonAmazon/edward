import { chromium } from "playwright-core";
import { mkdir } from "node:fs/promises";

const out = new URL("../../media/mock/", import.meta.url).pathname;
await mkdir(out, { recursive: true });

const browser = await chromium.launch({
  executablePath: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
  headless: true,
});
const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 2 });
const open = ".dialog[data-open='true']";
const settle = () => page.waitForTimeout(600);
const shot = async (name) => {
  await page.mouse.move(2, 2);
  await settle();
  await page.screenshot({ path: `${out}${name}.png` });
  console.log("saved", name);
};

await page.goto("http://127.0.0.1:5173/");
await page.waitForTimeout(1000);
await shot("readme-first-run");

await page.locator(`${open} .wipe`, { hasText: "Skip" }).click();
await page.waitForTimeout(800);
await shot("readme-graph");

await page.locator(".node--hub[aria-label='Network']").click();
await settle();
await shot("readme-shelf");

await page.locator(".sheet .wipe", { hasText: "Open finding" }).first().click();
await settle();
await shot("readme-finding");

await browser.close();
