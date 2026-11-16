const har=require("/opt/sparked/asrockrack-api/docs/x470/har-endpoints.json");const fs=require("fs");
const P=process.argv[2];
function shape(v,p="",out=new Set()){ if(Array.isArray(v)){out.add(p+"[]"); if(v.length) shape(v[0],p+"[]",out);} else if(v&&typeof v=="object"){for(const k of Object.keys(v)){out.add(p+"."+k+":"+(Array.isArray(v[k])?"array":v[k]===null?"null":typeof v[k])); shape(v[k],p+"."+k,out);} } return out;}
for(const e of har.endpoints){ if(e.method!="GET")continue; const n=e.path.replace(/^\/api\//,"").replace(/[\/:]/g,"_"); const f=`${P}/raw/api/${n}.json`; if(!fs.existsSync(f)){console.log(e.path,"NOFILE");continue;}
 let live; try{live=JSON.parse(fs.readFileSync(f,"latin1"))}catch(x){console.log(e.path,"live not json");continue;}
 const a=shape(e.response_example), b=shape(live); const onlyHar=[...a].filter(x=>!b.has(x)), onlyLive=[...b].filter(x=>!a.has(x));
 console.log(e.path, "har_status",JSON.stringify(e.status),"har_wait_med",e.timing_ms.wait_median, onlyHar.length||onlyLive.length?("DIFF har-only="+JSON.stringify(onlyHar.slice(0,8))+" live-only="+JSON.stringify(onlyLive.slice(0,8))):"same-shape");}
