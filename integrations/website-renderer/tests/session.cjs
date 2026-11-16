const http=require('node:http');
const assert=require('node:assert/strict');
const handle=require('/renderer/session.cjs');
const requests=[];
const deadline = setTimeout(() => { console.error("Worker test timed out"); process.exit(1); }, 90000);
const html=`<!doctype html><style>body{margin:0;font:20px sans-serif}a{position:absolute;left:20px;top:20px}form{position:absolute;left:20px;top:80px}input{width:200px;height:30px}#download{top:160px}#post{top:220px}#blob{top:300px}</style><a href="/second">Second page</a><form action="/search"><input name="q" autofocus></form><a id="download" href="/download">Download</a><form id="post" method="post" action="/submit"><input name="value" value="test-only"><button>Submit</button></form><a id="blob" download="test.txt" href="data:text/plain,test-only">Blob download</a>`;
const broker=http.createServer((req,res)=>{
 const target=new URL(new URL(req.url,'http://broker').searchParams.get('url'));
 let body='';req.on('data',c=>body+=c);req.on('end',()=>{
  requests.push({path:target.pathname,query:target.search,cookie:req.headers.cookie,method:req.method,body});
  if(target.pathname==='/redirect') return res.writeHead(302,{'Location':'https://fixture.example/cookie-a'}).end();
  if(target.pathname==='/submit') return res.writeHead(307,{'Location':'/submitted'}).end();
  if(target.pathname==='/download') return res.writeHead(415).end();
  res.writeHead(200,{'Content-Type':'text/html',...(target.pathname==='/cookie-a'?{'Set-Cookie':'isolated=alice; Path=/'}:{})});
  res.end(html);
 });
});
const api=http.createServer(handle);
(async()=>{
 await new Promise(r=>broker.listen('/run/website-renderer/fetch.sock',r));
 await new Promise(r=>api.listen('/run/website-renderer/test.sock',r));
 async function call(id,action,extra={}){
  return new Promise((resolve,reject)=>{
   const request=http.request({socketPath:'/run/website-renderer/test.sock',path:'/session',method:'POST'},response=>{
    let body='';response.on('data',c=>body+=c);response.on('end',()=>{ if(response.statusCode>=400 && response.statusCode!==410) return reject(new Error(body)); resolve({status:response.statusCode,...JSON.parse(body)}); });
   });request.on('error',reject);request.end(JSON.stringify({session:id,action,url:'https://fixture.example/',...extra}));
  });
 }
 const a='a'.repeat(64), b='b'.repeat(64);
 try {
  assert.equal((await call(a,'start',{url:'https://fixture.example/redirect'})).status,200);
  assert.equal((await call(a,'frame')).url,'https://fixture.example/cookie-a');
  assert.equal((await call(a,'click',{x:50,y:30})).status,200);
  let next=await call(a,'frame');assert.equal(next.url,'https://fixture.example/second');
  assert(requests.some(r=>r.path==='/second' && r.cookie==='isolated=alice'));
  await call(a,'click',{x:60,y:95});await call(a,'text',{text:'browser-test'});await call(a,'key',{text:'Enter'});
  next=await call(a,'frame');assert(next.url.includes('/search?q=browser-test'));
  await call(a,'back');assert.equal((await call(a,'frame')).url,'https://fixture.example/second');
  await call(a,'forward');assert((await call(a,'frame')).url.includes('/search?q=browser-test'));
  await call(a,'click',{x:280,y:235});await call(a,'frame');
  assert(requests.some(r=>r.path==='/submitted' && r.method==='POST' && r.body==='value=test-only'));
  await call(b,'start');await call(b,'click',{x:50,y:30});await call(b,'frame');
  const visits=requests.filter(r=>r.path==='/second');assert(!visits[visits.length-1].cookie);
  const download=await call(b,'click',{x:60,y:170});const after=await call(b,'frame');
  assert((download.message+after.message).includes('downloads are disabled'));
  await call(a,'reload');
  const blob=await call(a,'click',{x:60,y:310});const blobAfter=await call(a,'frame');
  assert((blob.message+blobAfter.message).includes('downloads are disabled'));
  await call(a,'close');assert.equal((await call(a,'frame')).status,410);
  console.log('PASS navigation, typing, GET search, POST forms, back/forward, cookie isolation, download refusal, close');
 } finally {await call(a,'close');await call(b,'close');api.close();broker.close();clearTimeout(deadline);}
})().then(()=>process.exit(0)).catch(error=>{console.error(error);process.exit(1)});
