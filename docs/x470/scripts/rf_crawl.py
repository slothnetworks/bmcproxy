import subprocess,sys,time,json,os,re
host=os.environ['BMC_HOST']; auth=os.environ.get('RF_AUTH'); out=sys.argv[1]; seeds=sys.argv[2:]
cap=600; seen=set(); queue=list(seeds); res=[]
def get(u,method='GET'):
    cmd=['curl','-sk','-m','30','-X',method,'-D','-','-o','/tmp/claude-1000/-opt-sparked-asrockrack-api/152d9ce5-3665-4f40-8521-6ab266196a9e/scratchpad/probe-x470/_b','-w','\n%{http_code} %{time_total}']
    if auth: cmd+=['-H','@'+auth]
    if method=='HEAD': cmd[cmd.index('-X')+1:cmd.index('-X')+2]=['HEAD']; cmd.insert(1,'-I')
    r=subprocess.run(cmd+['https://%s%s'%(host,u)],capture_output=True,text=True)
    lines=r.stdout.rstrip().split('\n'); code,t=lines[-1].split(); hdr='\n'.join(lines[:-1])
    try: body=open('/tmp/claude-1000/-opt-sparked-asrockrack-api/152d9ce5-3665-4f40-8521-6ab266196a9e/scratchpad/probe-x470/_b','rb').read()
    except: body=b''
    return int(code),float(t),hdr,body
def links(o,acc):
    if isinstance(o,dict):
        for k,v in o.items():
            if k in('@odata.id','Members@odata.nextLink','@Redfish.ActionInfo','target') and isinstance(v,str) and v.startswith('/redfish'):
                if k!='target': acc.add(v.split('#')[0])
            else: links(v,acc)
    elif isinstance(o,list):
        for v in o: links(v,acc)
def actions(o,path='',acc=None):
    acc={} if acc is None else acc
    if isinstance(o,dict):
        for k,v in o.items():
            if k=='Actions' and isinstance(v,dict):
                for an,av in v.items():
                    if isinstance(av,dict) and an!='Oem': acc[an]={kk:vv for kk,vv in av.items()}
                    elif an=='Oem' and isinstance(av,dict):
                        for on,ov in av.items(): acc['Oem/'+on]=ov
            elif k!='Members': actions(v,path+'/'+k,acc)
    return acc
while queue and len(res)<cap:
    u=queue.pop(0)
    if u in seen: continue
    seen.add(u)
    code,t,hdr,body=get(u)
    H={}
    for l in hdr.splitlines():
        if ':' in l: k,v=l.split(':',1); H.setdefault(k.strip().lower(),[]).append(v.strip())
    rec=dict(uri=u,status=code,ms=round(t*1000),allow=(H.get('allow') or [None])[0],etag=(H.get('etag') or [None])[0],
             ctype=(H.get('content-type') or [None])[0],bytes=len(body))
    try: j=json.loads(body)
    except Exception: j=None
    if isinstance(j,dict):
        rec['odata_type']=j.get('@odata.type'); a=actions(j)
        if a: rec['actions']=a
        if 'Members@odata.count' in j: rec['members_count']=j['Members@odata.count']; rec['members_returned']=len(j.get('Members',[]))
        if 'Members@odata.nextLink' in j: rec['nextLink']=j['Members@odata.nextLink']
        if 'error' in j: rec['error']=j['error'].get('code')
        fn=os.path.join(out,(u.strip('/').replace('/','__').replace('$','S') or 'root')[:200]+'.json')
        os.makedirs(out,exist_ok=True); json.dump(dict(headers=H,body=j),open(fn,'w'),indent=1)
        acc=set(); links(j,acc)
        big=re.search(r'/LogServices/[^/]+/Entries$',u) is not None
        for l in sorted(acc):
            if l in seen: continue
            if re.search(r'/LogServices/[^/]+/Entries/[^/]+$',l) and sum(1 for s in seen if '/Entries/' in s)>=3: continue
            if re.search(r'/JsonSchemas/.+',l) and sum(1 for s in seen if '/JsonSchemas/' in s)>=5: continue
            queue.append(l)
    res.append(rec)
    print(code,rec['ms'],u,rec.get('odata_type'),rec.get('allow'),flush=True)
    if code in(403,503): time.sleep(3)
    time.sleep(0.25)
json.dump(res,open(out+'.json','w'),indent=1)
print('done',len(res),'remaining',len(queue))
