import subprocess,sys,time,json,os
host=os.environ['BMC_HOST']; out=sys.argv[1]; paths=sys.argv[2:]
os.makedirs(out,exist_ok=True); res=[]
for p in paths:
    fn=os.path.join(out,p.strip('/').replace('/','__') or 'root')
    r=subprocess.run(['curl','-sk','-m','30','-H','@api.cookie','-H','@api.csrf','-H','X-Requested-With: XMLHttpRequest','-H','Accept: application/json',
        '-D',fn+'.hdr','-o',fn+'.body','-w','%{http_code} %{time_total} %{size_download}','https://%s/api/%s'%(host,p)],capture_output=True,text=True)
    code,t,sz=r.stdout.split()
    hdr=open(fn+'.hdr').read() if os.path.exists(fn+'.hdr') else ''
    body=open(fn+'.body','rb').read()[:160].decode('latin1')
    res.append(dict(path=p,status=int(code),ms=round(float(t)*1000),bytes=int(sz),conn_close='Connection: close' in hdr,ctype=next((l.split(':',1)[1].strip() for l in hdr.splitlines() if l.lower().startswith('content-type')),None),head=body))
    print(code,round(float(t)*1000),sz,p,'|',body[:110].replace('\n',' '),flush=True)
    if code in('403','503'): time.sleep(3)
    time.sleep(0.25)
json.dump(res,open(out+'.json','w'),indent=1)
