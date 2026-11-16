const dgram=require("dgram");const host=process.env.BMC_HOST;
function authcap(d0,d1){const msg=[0x20,0x18,0xc8,0x81,0x00,0x38,d0,d1];let s=0;for(const x of msg.slice(3))s+=x;msg.push((0x100-(s&0xff))&0xff);
 return Buffer.from([0x06,0x00,0xff,0x07,0x00,0,0,0,0,0,0,0,0,msg.length,...msg]);}
const pkts={authcap_v2ext:authcap(0x8e,0x04),authcap_v15:authcap(0x0e,0x04)};
async function send(name){return new Promise(r=>{const s=dgram.createSocket("udp4");const t=setTimeout(()=>{console.log(name,"timeout");s.close();r()},3000);
 s.on("message",m=>{clearTimeout(t);console.log(name,"reply",m.toString("hex"));s.close();r()}); s.send(pkts[name],623,host);});}
(async()=>{for(const n of Object.keys(pkts)){console.log(n,"sent",pkts[n].toString("hex"));await send(n);await new Promise(r=>setTimeout(r,300));}})();
