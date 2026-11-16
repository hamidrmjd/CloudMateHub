const http = require('node:http');
const { chromium } = require('playwright-core');
const sessions = new Map();
let browserPromise;
function getBrowser() {
  if (!browserPromise) {
    browserPromise = chromium.launch({executablePath:'/usr/bin/chromium', args:['--disable-background-networking','--disable-quic','--renderer-process-limit=8']})
      .then(browser => { browser.on('disconnected', () => { browserPromise=undefined; sessions.clear(); }); return browser; })
      .catch(error => { browserPromise=undefined; throw error; });
  }
  return browserPromise;
}
const MAX_SESSIONS = 4;
const IDLE_MS = 5 * 60 * 1000;
const MAX_AGE_MS = 60 * 60 * 1000;

async function destroy(id) {
  const session = sessions.get(id);
  if (!session) return;
  sessions.delete(id);
  await session.context?.close().catch(() => {});
}
setInterval(() => {
  for (const [id, s] of sessions) {
    if (Date.now() - s.last > IDLE_MS || Date.now() - s.created > MAX_AGE_MS) void destroy(id);
  }
}, 30000).unref();

function fetchResource(request) {
  return new Promise((resolve, reject) => {
    const headers = request.headers;
    const allowed = Object.fromEntries(Object.entries(headers).map(([key,value]) => [key.toLowerCase(),value]).filter(([key]) =>
      ['cookie', 'content-type', 'accept', 'accept-language', 'origin', 'referer'].includes(key)));
    const req = http.request({socketPath:'/run/website-renderer/fetch.sock',
      path:'/?url='+encodeURIComponent(request.url), method:request.method, headers:allowed, timeout:60000}, res => {
      const chunks=[];
      res.on('data', chunk => chunks.push(chunk));
      res.on('error', reject);
      res.on('end', () => resolve({status:res.statusCode, headers:res.headers, body:Buffer.concat(chunks)}));
    });
    req.on('error', reject);
    req.on('timeout', () => req.destroy(new Error('Resource timed out')));
    req.end(request.postData);
  });
}

async function start(id, url) {
  if (sessions.size >= MAX_SESSIONS) throw new Error('Browser capacity reached. Close another website tab and retry.');
  const session = {last:Date.now(), created:Date.now(), busy:true, message:'', requests:0, bytes:0};
  sessions.set(id, session);
  try {
    const browser = await getBrowser();
    const context = await browser.newContext({viewport:{width:1280,height:800}, acceptDownloads:false, serviceWorkers:'block'});
    session.context = context;
    if (sessions.get(id) !== session) { await context.close(); throw new Error('Browser session closed'); }
    await context.routeWebSocket('**/*', ws => ws.close());
    const setupPage = page => {
      page.on('download', download => { session.message='File downloads are disabled.'; void download.cancel(); });
      page.on('filechooser', () => { session.message='File uploads are disabled.'; });
      page.on('dialog', dialog => void dialog.dismiss());
    };
    session.page = await context.newPage();
    setupPage(session.page);
    const network = await context.newCDPSession(session.page);
    await network.send('Fetch.enable', {patterns:[{urlPattern:'*',requestStage:'Request'}]});
    network.on('Fetch.requestPaused', async ({requestId,request}) => {
      const fail = () => network.send('Fetch.failRequest', {requestId,errorReason:'BlockedByClient'}).catch(() => {});
      if (!['GET','HEAD','POST'].includes(request.method) || !/^https?:\/\//i.test(request.url) ||
          Buffer.byteLength(request.postData || '') > 1024*1024 || ++session.requests > 5000 || session.bytes > 256*1024*1024) {
        session.message = 'This request was blocked by the browsing policy.';
        await fail(); return;
      }
      try {
        const result = await fetchResource(request);
        session.bytes += result.body.length;
        if (result.status === 415) session.message = 'File downloads are disabled.';
        const responseHeaders = Object.entries(result.headers).filter(([key]) =>
          ['content-type','location','access-control-allow-origin','access-control-allow-credentials','content-security-policy','set-cookie'].includes(key))
          .flatMap(([name,value]) => (Array.isArray(value) ? value : [value]).map(value => ({name,value})));
        // Chromium pauses every redirect hop, keeping URL, cookie and POST semantics intact.
        await network.send('Fetch.fulfillRequest', {requestId,responseCode:result.status,responseHeaders,body:result.body.toString('base64')});
      } catch { await fail(); }
    });
    // Popup destinations open in the existing remote viewport. They never use the client network.
    context.on('request', request => {
      if (!request.isNavigationRequest()) return;
      let popup;
      try { popup = request.frame().page(); } catch { return; }
      if (popup === session.page) return;
      if (/^https?:\/\//i.test(request.url())) void session.page.goto(request.url(), {waitUntil:'commit',timeout:30000}).catch(() => {});
      void popup.close().catch(() => {});
    });
    await session.page.goto(url, {waitUntil:'domcontentloaded',timeout:60000}).catch(() => {
      session.message='The page is still loading or the destination could not be reached.';
    });
    session.busy = false;
    return session;
  } catch (error) { await destroy(id); throw error; }
}

module.exports = async function handleSession(req,res) {
  const reply = (status, body) => res.writeHead(status, {'Content-Type':'application/json','Cache-Control':'no-store'}).end(JSON.stringify(body));
  let raw='';
  try {
    for await (const chunk of req) { raw+=chunk; if(raw.length>16384) return reply(400,{error:'Request too large'}); }
    const input=JSON.parse(raw);
    const {session:id, action, text=''}=input;
    if (!/^[a-f0-9]{64}$/.test(id || '') || typeof text!=='string' || text.length>4096) return reply(400,{error:'Invalid request'});
    if(action==='close') { await destroy(id); return reply(200,{}); }
    let s=sessions.get(id);
    if(!s && action==='start') s=await start(id,input.url);
    if(!s) return reply(410,{error:'Browser session expired. Reopen the website to reconnect.'});
    if(s.busy) return reply(409,{error:'Browser is busy'});
    s.busy=true;
    try {
      if(action!=='frame') s.last=Date.now();
      const page=s.page;
      const x=Math.max(0,Math.min(1279,Number(input.x)||0));
      const y=Math.max(0,Math.min(799,Number(input.y)||0));
      switch(action) {
        case 'start': case 'frame': break;
        case 'click': await page.mouse.click(x,y); break;
        case 'move': await page.mouse.move(x,y); break;
        case 'wheel': await page.mouse.wheel(Math.max(-1600,Math.min(1600,Number(input.x)||0)),Math.max(-1600,Math.min(1600,Number(input.y)||0))); break;
        case 'text': await page.keyboard.insertText(text); break;
        case 'key':
          if(!/^(?:(?:Control|Alt|Shift|Meta)\+)*(?:[a-zA-Z0-9]|Enter|Tab|Backspace|Delete|Escape|ArrowUp|ArrowDown|ArrowLeft|ArrowRight|Home|End|PageUp|PageDown|Space)$/.test(text)) return reply(400,{error:'Unsupported key'});
          await page.keyboard.press(text); break;
        case 'back': await page.goBack({waitUntil:'commit',timeout:15000}).catch(()=>{}); break;
        case 'forward': await page.goForward({waitUntil:'commit',timeout:15000}).catch(()=>{}); break;
        case 'reload': await page.reload({waitUntil:'commit',timeout:15000}).catch(()=>{}); break;
        default: return reply(400,{error:'Unsupported action'});
      }
      let sessionLoading = false;
      // Navigation can replace the compositor during a frame. Keep the last
      // frame and retry on the next poll instead of disconnecting the session.
      try {
        const image = await s.page.screenshot({type:'jpeg',quality:75,timeout:2000});
        s.lastFrame = 'data:image/jpeg;base64,' + image.toString('base64');
      } catch {
        sessionLoading = true;
      }
      reply(200,{image:s.lastFrame,url:s.page.url(),message:s.message || (sessionLoading ? 'Page is loading…' : '')});
      s.message='';
    } finally { s.busy=false; }
  } catch (error) { console.error('Browser operation failed:', error.message.split('\n')[0]); reply(502,{error:'The remote browser could not complete this request. Reconnect and try again.'}); }
};
