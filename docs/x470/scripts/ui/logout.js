const { chromium } = require('playwright');
const fs = require('fs');
const P = '/tmp/claude-1000/-opt-sparked-asrockrack-api/4c303595-4244-4c22-8dc9-6968bfcb1de5/scratchpad/probe';
const B = `https://${process.env.BMC_HOST}`;
const log = []; const t0 = Date.now(); const ts = () => ((Date.now() - t0) / 1000).toFixed(1);
(async () => {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ ignoreHTTPSErrors: true, viewport: { width: 1400, height: 900 }, recordHar: { path: `${P}/ui/logout.har`, content: 'embed' } });
  await context.route('**/*', route => { const r = route.request(); const u = new URL(r.url());
    if (r.method() === 'GET' || u.pathname === '/api/session') return route.continue(); log.push({ blocked: r.method() + ' ' + u.pathname }); return route.abort(); });
  const page = await context.newPage();
  page.on('dialog', d => { log.push({ dialog: d.message() }); d.accept(); });
  page.on('request', r => { const u = new URL(r.url()); if (u.pathname.startsWith('/api')) log.push({ t: ts(), m: r.method(), p: u.pathname, hdr: Object.keys(r.headers()).filter(h => /csrf|cookie|x-req/i.test(h)) }); });
  page.on('response', r => { const u = new URL(r.url()); if (u.pathname === '/api/session') log.push({ t: ts(), resp: r.request().method() + ' ' + r.status(), setcookie: (r.headers()['set-cookie'] || '').replace(/=[^;]{6,}/g, '=<R>') }); });
  await page.goto(B + '/', { waitUntil: 'networkidle' });
  await page.fill('#userid', process.env.BMC_USER); await page.fill('#password', process.env.BMC_PASS);
  await page.click('#btn-login'); await page.waitForTimeout(6000);
  const so = page.locator('a[href="#logout"]:visible').first();
  log.push({ visibleSignOut: await so.count() });
  if (await so.count()) await so.click(); else { await page.click('text=admin'); await page.waitForTimeout(800); await page.locator('a[href="#logout"]:visible').first().click(); }
  await page.waitForTimeout(6000);
  await page.screenshot({ path: `${P}/ui/shots/99c-after-signout.png` });
  log.push({ urlAfter: page.url(), cookiesAfter: (await context.cookies()).map(c => c.name + ':' + c.value.length) });
  await context.close(); await browser.close();
  fs.writeFileSync(`${P}/ui/logout-result.json`, JSON.stringify(log, null, 1));
  console.log(JSON.stringify(log.filter(l => !l.p || l.p === '/api/session')));
})();
