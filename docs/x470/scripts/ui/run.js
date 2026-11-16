const { chromium } = require('playwright');
const fs = require('fs');
const P = '/tmp/claude-1000/-opt-sparked-asrockrack-api/4c303595-4244-4c22-8dc9-6968bfcb1de5/scratchpad/probe';
const H = process.env.BMC_HOST, B = `https://${H}`;
const log = []; const blocked = []; const wsinfo = []; const consoleLog = [];
const t0 = Date.now(); const ts = () => ((Date.now() - t0) / 1000).toFixed(1);
const routes = ['dashboard','sensors','sysinfo','sysinfo/fru','sysinfo/power_source','sysinfo/system_inventory','logs/ipmi-event-log','logs/video-log',
 'settings','settings/network','settings/network/ip','settings/network/bond','settings/network/link','settings/network/ncsi','settings/network/dns','settings/date_time','settings/users','settings/services','settings/service_sessions',
 'settings/fan','settings/media','settings/media/general','settings/ssl','settings/ssl/view_ssl','settings/log','settings/pef','settings/smtp','settings/prochot','settings/bsod','settings/keep_ncsi_link_up',
 'settings/sensor_thresholds','settings/video','settings/ext_users','settings/firewall','settings/pamorder','settings/web_server_instances','settings/mouse',
 'remote-control','power-control','miscellaneous','miscellaneous/uidcontrol','miscellaneous/postsnoop','image_redirection',
 'maintenance','maintenance/fwinfo','maintenance/firmware_update','maintenance/bios_update','maintenance/dual_image_config','maintenance/backup_config','maintenance/preserve_configuration','maintenance/system_admin','maintenance/fwimage_location','maintenance/cold_reset'];
function allowed(req) {
  const u = new URL(req.url()); const m = req.method();
  if (u.pathname !== '/api/session' && /help-data|nm_|restore|flash|upgrade|reset|delete|recovery|verification/i.test(u.pathname)) return false;
  if (m === 'GET' || m === 'HEAD') return true;
  if (u.pathname === '/api/session' && (m === 'POST' || m === 'DELETE')) return true;
  if (u.pathname === '/api/viewerlogin' && m === 'POST') return true;
  return false;
}
const frame = p => { const b = Buffer.isBuffer(p) ? p : Buffer.from(String(p)); return { len: b.length, type: Buffer.isBuffer(p) ? 'bin' : 'text', head: b.slice(0, 48).toString('hex') }; };
(async () => {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ ignoreHTTPSErrors: true, viewport: { width: 1400, height: 900 },
    recordHar: { path: `${P}/ui/ui.har`, content: 'embed', mode: 'full' } });
  await context.addInitScript(() => {
    const W = window.WebSocket;
    const Wrap = function (url, protos) {
      const ws = protos ? new W(url, protos) : new W(url);
      console.log('WSLOG new ' + JSON.stringify({ url: String(url), protos }));
      ws.addEventListener('open', () => console.log('WSLOG open ' + JSON.stringify({ url: String(url), protocol: ws.protocol, extensions: ws.extensions, binaryType: ws.binaryType })));
      ws.addEventListener('close', e => console.log('WSLOG close ' + JSON.stringify({ url: String(url), code: e.code, reason: e.reason })));
      ws.addEventListener('error', () => console.log('WSLOG error ' + String(url)));
      return ws;
    };
    Wrap.prototype = W.prototype; Wrap.CONNECTING = 0; Wrap.OPEN = 1; Wrap.CLOSING = 2; Wrap.CLOSED = 3;
    window.WebSocket = Wrap;
  });
  await context.route('**/*', route => { const r = route.request(); if (allowed(r)) return route.continue(); blocked.push({ t: ts(), m: r.method(), u: r.url() }); return route.abort(); });
  const attach = (pg, tag) => {
    pg.on('response', async res => { const r = res.request(); if (!r.url().startsWith(B)) return; const u = new URL(r.url());
      if (!u.pathname.startsWith('/api') && !u.pathname.startsWith('/redfish') && !/viewer|kvm|cd-server/.test(u.pathname)) return;
      log.push({ t: ts(), tag, m: r.method(), p: u.pathname + u.search, s: res.status(), ms: Math.round(r.timing().responseEnd) }); });
    pg.on('console', msg => { consoleLog.push({ t: ts(), tag, text: msg.text().slice(0, 500) }); });
    pg.on('websocket', ws => { const w = { tag, url: ws.url(), opened: ts(), sent: [], recv: [], nsent: 0, nrecv: 0 }; wsinfo.push(w);
      ws.on('framesent', f => { w.nsent++; if (w.sent.length < 15) w.sent.push(frame(f.payload)); });
      ws.on('framereceived', f => { w.nrecv++; if (w.recv.length < 15) w.recv.push(frame(f.payload)); });
      ws.on('close', () => { w.closed = ts(); }); });
  };
  const page = await context.newPage(); attach(page, 'main');
  context.on('page', pg => { if (pg !== page) attach(pg, 'popup'); });
  await page.goto(B + '/', { waitUntil: 'networkidle' });
  await page.screenshot({ path: `${P}/ui/shots/00-login.png` });
  await page.fill('#userid', process.env.BMC_USER); await page.fill('#password', process.env.BMC_PASS);
  await page.click('#btn-login');
  await page.waitForTimeout(6000);
  await page.screenshot({ path: `${P}/ui/shots/01-after-login.png` });
  const cookies = (await context.cookies()).map(c => ({ name: c.name, valueLen: c.value.length, domain: c.domain, path: c.path, expires: c.expires, httpOnly: c.httpOnly, secure: c.secure, sameSite: c.sameSite }));
  const storage = await page.evaluate(() => ({ ls: Object.keys(localStorage), ss: Object.keys(sessionStorage) }));
  let i = 2;
  for (const r of routes) {
    try { await page.goto(`${B}/#${r}`); await page.waitForTimeout(r === 'logs/ipmi-event-log' ? 9000 : 3500);
      await page.screenshot({ path: `${P}/ui/shots/${String(i).padStart(2, '0')}-${r.replace(/\//g, '_')}.png` }); } catch (e) { log.push({ t: ts(), err: r + ' ' + e.message }); }
    i++;
  }
  const vm = {};
  try {
    await page.goto(`${B}/#remote-control`); await page.waitForTimeout(4000);
    const [popup] = await Promise.all([context.waitForEvent('page', { timeout: 20000 }), page.click('#download')]);
    await popup.waitForTimeout(15000);
    await popup.screenshot({ path: `${P}/ui/shots/90-h5viewer.png` });
    vm.title = await popup.title();
    vm.menus = await popup.evaluate(() => Array.from(document.querySelectorAll('a,button,li')).map(e => (e.innerText || '').trim()).filter(t => t && t.length < 40).slice(0, 120));
    const cand = await popup.$$('text=/Virtual Media|Media/i');
    vm.mediaCandidates = cand.length;
    if (cand.length) {
      try { await cand[0].click({ timeout: 5000 }); await popup.waitForTimeout(2500); await popup.screenshot({ path: `${P}/ui/shots/91-h5viewer-media-menu.png` });
        vm.menuAfter = await popup.evaluate(() => Array.from(document.querySelectorAll('a,button,li')).filter(e => e.offsetParent).map(e => (e.innerText || '').trim()).filter(t => t && t.length < 40).slice(0, 80));
      } catch (e) { vm.mediaErr = e.message; }
    }
    vm.storage = await popup.evaluate(() => ({ ls: Object.keys(localStorage), ss: Object.keys(sessionStorage) }));
    fs.writeFileSync(`${P}/ui/vm-pre.json`, JSON.stringify(vm, null, 1));
    await popup.waitForTimeout(2000);
    await popup.close(); await page.waitForTimeout(3000);
  } catch (e) { vm.err = e.message; }
  try { await page.goto(`${B}/#dashboard`); await page.waitForTimeout(3000);
    const lo = await page.$('a[href="#logout"]');
    vm.logoutLink = !!lo;
    if (lo) { try { await lo.click({ timeout: 3000 }); } catch (e) { await page.goto(`${B}/#logout`); } } else await page.goto(`${B}/#logout`);
    await page.waitForTimeout(5000); await page.screenshot({ path: `${P}/ui/shots/99-after-logout.png` }); } catch (e) { log.push({ t: ts(), err: 'logout ' + e.message }); }
  const cookiesAfter = (await context.cookies()).map(c => ({ name: c.name, valueLen: c.value.length }));
  await context.close(); await browser.close();
  fs.writeFileSync(`${P}/ui/result.json`, JSON.stringify({ log, blocked, wsinfo, consoleLog: consoleLog.filter(c => /WSLOG|rror/.test(c.text)).slice(0, 200), cookies, cookiesAfter, storage, vm }, null, 1));
  console.log('done', log.length, 'requests', blocked.length, 'blocked', wsinfo.length, 'ws');
})();
