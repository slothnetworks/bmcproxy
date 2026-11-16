const dgram=require("dgram");const host=process.env.BMC_HOST;
const pkts={asfping:Buffer.from("0600ff06000011be80000000","hex"),
 authcap_v2:Buffer.from("0600ff070000000000000000000920186881000038"+"8e04"+"b5","hex"),
 authcap_v15:Buffer.from("0600ff070000000000000000000920186881000038"+"0e04"+"35","hex")};
// fix: rqAddr 81 checksum header 0xc8 for 20 18
for(const k of ["authcap_v2","authcap_v15"]){const b=pkts[k]; b[16]=0xc8; b[17]=0x81; b[18]=0x00; b[19]=0x38; const d=[b[20],b[21]]; let s=(0x81+0x00+0x38+d[0]+d[1])&0xff; b[22]=(0x100-s)&0xff;}
async function send(name){return new Promise(r=>{const s=dgram.createSocket("udp4");const t=setTimeout(()=>{console.log(name,"timeout");s.close();r()},3000);
 s.on("message",m=>{clearTimeout(t);console.log(name,"reply",m.toString("hex"));s.close();r()}); s.send(pkts[name],623,host);});}
(async()=>{for(const n of Object.keys(pkts)){console.log(n,"sent",pkts[n].toString("hex"));await send(n);await new Promise(r=>setTimeout(r,300));}})();
