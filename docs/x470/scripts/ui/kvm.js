const { chromium } = require('playwright');
const fs = require('fs');
const P = '/tmp/claude-1000/-opt-sparked-asrockrack-api/4c303595-4244-4c22-8dc9-6968bfcb1de5/scratchpad/probe';
const H = process.env.BMC_HOST, B = `https://${H}`;
const log = [], blocked = [], wsinfo = [], cons = [], wsHandshake = [];
const t0 = Date.now(); const ts = () => ((Date.now() - t0) / 1000).toFixed(1);
function allowed(req) {
  const u = new URL(req.url()); const m = req.method();
  if (u.pathname !== '/api/session' && /help-data|nm_|restore|flash|upgrade|reset|delete|recovery|verification|start-media|stop-media/i.test(u.pathname)) return false;
  if (m === 'GET' || m === 'HEAD') return true;
  if (u.pathname === '/api/session' && (m === 'POST' || m === 'DELETE')) return true;
  if (u.pathname === '/api/viewerlogin' && m === 'POST') return true;
  return false;
}
const frame = p => { const b = Buffer.isBuffer(p) ? p : Buffer.from(String(p)); return { len: b.length, type: Buffer.isBuffer(p) ? 'bin' : 'text', head: b.slice(0, 40).toString('hex') }; };
(async () => {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ ignoreHTTPSErrors: true, viewport: { width: 1400, height: 900 },
    recordHar: { path: `${P}/ui/kvm.har`, content: 'embed', mode: 'full' } });
  await context.addInitScript(() => {
    const W = window.WebSocket;
    const Wrap = function (url, protos) {
      const ws = protos ? new W(url, protos) : new W(url);
      console.log('WSLOG new ' + JSON.stringify({ url: String(url).replace(/([?&][^=]+=)[^&]+/g, '$1<R>'), protos }));
      ws.addEventListener('open', () => console.log('WSLOG open ' + JSON.stringify({ protocol: ws.protocol, extensions: ws.extensions })));
      ws.addEventListener('close', e => console.log('WSLOG close ' + JSON.stringify({ code: e.code, reason: e.reason })));
      return ws;
    };
    Wrap.prototype = W.prototype; Wrap.CONNECTING = 0; Wrap.OPEN = 1; Wrap.CLOSING = 2; Wrap.CLOSED = 3;
    window.WebSocket = Wrap;
  });
  await context.route('**/*', route => { const r = route.request(); if (allowed(r)) return route.continue(); blocked.push({ t: ts(), m: r.method(), u: new URL(r.url()).pathname }); return route.abort(); });
  const attach = async (pg, tag) => {
    pg.on('response', res => { const r = res.request(); if (!r.url().startsWith(B)) return; const u = new URL(r.url());
      if (!u.pathname.startsWith('/api') && !/viewer|kvm|cd-server/.test(u.pathname)) return;
      log.push({ t: ts(), tag, m: r.method(), p: u.pathname, q: u.search ? 'yes' : '', s: res.status() }); });
    pg.on('console', msg => cons.push({ t: ts(), tag, text: msg.text().slice(0, 400) }));
    pg.on('websocket', ws => { const w = { tag, url: ws.url().replace(/([?&][^=]+=)[^&]+/g, '$1<R>'), opened: ts(), sent: [], recv: [], nsent: 0, nrecv: 0 }; wsinfo.push(w);
      ws.on('framesent', f => { w.nsent++; if (w.sent.length < 12) w.sent.push(frame(f.payload)); });
      ws.on('framereceived', f => { w.nrecv++; if (w.recv.length < 12) w.recv.push(frame(f.payload)); });
      ws.on('close', () => { w.closed = ts(); }); });
    try { const cdp = await context.newCDPSession(pg); await cdp.send('Network.enable');
      cdp.on('Network.webSocketWillSendHandshakeRequest', e => wsHandshake.push({ tag, dir: 'req', url: e.request.url ? e.request.url.replace(/([?&][^=]+=)[^&]+/g, '$1<R>') : '', headers: Object.fromEntries(Object.entries(e.request.headers).map(([k, v]) => [k, /cookie|csrf|token/i.test(k) ? `<REDACTED len ${String(v).length}>` : v])) }));
      cdp.on('Network.webSocketHandshakeResponseReceived', e => wsHandshake.push({ tag, dir: 'resp', status: e.response.status, headers: Object.fromEntries(Object.entries(e.response.headers).map(([k, v]) => [k, /cookie|csrf|token/i.test(k) ? '<REDACTED>' : v])) }));
    } catch (e) { cons.push({ tag, text: 'cdp ' + e.message }); }
  };
  const page = await context.newPage(); await attach(page, 'main');
  page.on('dialog', d => { cons.push({ tag: 'main', text: 'DIALOG ' + d.message() }); d.accept(); });
  context.on('page', pg => { if (pg !== page) attach(pg, 'popup'); });
  await page.goto(B + '/', { waitUntil: 'networkidle' });
  await page.fill('#userid', process.env.BMC_USER); await page.fill('#password', process.env.BMC_PASS);
  await page.click('#btn-login'); await page.waitForTimeout(6000);
  const vm = {};
  try {
    await page.click('text=Remote Control'); await page.waitForTimeout(6000);
    await page.screenshot({ path: `${P}/ui/shots/89-remote-control-via-menu.png` });
    vm.buttons = await page.evaluate(() => Array.from(document.querySelectorAll('button,a.btn,input[type=button]')).map(e => (e.id || '') + ':' + (e.innerText || e.value || '').trim()).slice(0, 30));
    const [popup] = await Promise.all([context.waitForEvent('page', { timeout: 20000 }), page.click('#download', { timeout: 10000 })]);
    await popup.waitForLoadState('domcontentloaded'); await popup.setViewportSize({ width: 1280, height: 900 });
    await popup.waitForTimeout(15000);
    await popup.screenshot({ path: `${P}/ui/shots/90-h5viewer.png` });
    vm.title = await popup.title(); vm.url = popup.url();
    vm.menus = await popup.evaluate(() => Array.from(document.querySelectorAll('a,button,li,span')).filter(e => e.offsetParent).map(e => (e.innerText || '').trim()).filter(t => t && t.length < 40 && !t.includes('\n'))).then ? null : null;
    vm.menus = await popup.evaluate(() => [...new Set(Array.from(document.querySelectorAll('a,button,li')).filter(e => e.offsetParent).map(e => (e.innerText || '').trim()).filter(t => t && t.length < 40 && !t.includes('\n')))].slice(0, 80));
    const media = popup.locator('text=/^\\s*(Media|Virtual Media)\\s*$/i').first();
    if (await media.count()) {
      await media.click({ timeout: 5000 }); await popup.waitForTimeout(2000);
      await popup.screenshot({ path: `${P}/ui/shots/91-h5viewer-media-menu.png` });
      vm.mediaMenu = await popup.evaluate(() => [...new Set(Array.from(document.querySelectorAll('a,button,li')).filter(e => e.offsetParent).map(e => (e.innerText || '').trim()).filter(t => t && t.length < 40 && !t.includes('\n')))].slice(0, 80));
      const vs = popup.locator('text=/Virtual Media Wizard|Virtual Storage|Virtual Media$/i').first();
      if (await vs.count()) { await vs.click({ timeout: 5000 }); await popup.waitForTimeout(3000); await popup.screenshot({ path: `${P}/ui/shots/92-h5viewer-media-dialog.png` });
        vm.dialogText = (await popup.evaluate(() => { const d = Array.from(document.querySelectorAll('.modal, .ui-dialog, [role=dialog]')).filter(e => e.offsetParent); return d.map(e => e.innerText.slice(0, 600)).join(' | '); }));
        await popup.keyboard.press('Escape'); await popup.waitForTimeout(1000); }
    }
    vm.storage = await popup.evaluate(() => ({ ls: Object.keys(localStorage), ss: Object.keys(sessionStorage) }));
    await popup.close(); await page.waitForTimeout(4000);
  } catch (e) { vm.err = e.message; await page.screenshot({ path: `${P}/ui/shots/89b-err.png` }).catch(() => {}); }
  try { await page.bringToFront(); await page.click('text=Sign out', { timeout: 10000 }); await page.waitForTimeout(2000);
    const ok = page.locator('button:has-text("OK"), button:has-text("Yes")').first(); if (await ok.count()) { await ok.click(); }
    await page.waitForTimeout(5000); await page.screenshot({ path: `${P}/ui/shots/99b-after-signout.png` }); } catch (e) { vm.logoutErr = e.message; }
  page.on('dialog', d => d.accept());
  await context.close(); await browser.close();
  fs.writeFileSync(`${P}/ui/kvm-result.json`, JSON.stringify({ log, blocked, wsinfo, wsHandshake, cons: cons.filter(c => !/OTS/.test(c.text)).slice(0, 150), vm }, null, 1));
  console.log('done', log.length, blocked.length, wsinfo.length, JSON.stringify(vm).slice(0, 300));
})();
