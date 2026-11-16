const dgram=require("dgram");const host=process.env.BMC_HOST;
function req(cmd,data,seq){const msg=[0x20,0x18,0xc8,0x81,(seq<<2)&0xff,cmd,...data];let s=0;for(const x of msg.slice(3))s+=x;msg.push((0x100-(s&0xff))&0xff);
 return Buffer.from([0x06,0x00,0xff,0x07,0x00,0,0,0,0,0,0,0,0,msg.length,...msg]);}
function send(b){return new Promise(r=>{const s=dgram.createSocket("udp4");const t=setTimeout(()=>{s.close();r(null)},3000);
 s.on("message",m=>{clearTimeout(t);s.close();r(m)}); s.send(b,623,host);});}
(async()=>{let all=[];for(let i=0;i<8;i++){const m=await send(req(0x54,[0x0e,0x00,0x80|i],i+1)); if(!m){console.log("timeout",i);break;}
 const h=m.toString("hex"); const data=[...m.slice(20,m.length-1)]; console.log(i,h,"cc",data[0]); const recs=data.slice(2); all.push(...recs); await new Promise(r=>setTimeout(r,250)); if(recs.length<16)break;}
 console.log("records hex",Buffer.from(all).toString("hex"));
 // parse
 let i=0,out=[];while(i<all.length){if(all[i]==0xc0){const id=all[i+1];i+=2;const algs=[];while(i<all.length&&all[i]!=0xc0&&all[i]!=0xc1){algs.push(all[i]);i++;}out.push({id,algs:algs.map(a=>a.toString(16))});}else i++;}
 console.log(JSON.stringify(out));})();
