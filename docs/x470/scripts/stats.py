import json,sys,base64,collections,datetime,statistics,hashlib,urllib.parse
h=json.load(open(sys.argv[1]));out=sys.argv[2]
g=collections.OrderedDict()
for i,e in enumerate(h['log']['entries']):
    u=urllib.parse.urlsplit(e['request']['url'])
    if '/api/' not in u.path: continue
    k=(e['request']['method'],u.path)
    g.setdefault(k,[]).append((i,e))
def ts(e): return datetime.datetime.fromisoformat(e['startedDateTime'].replace('Z','+00:00')).timestamp()
for k,v in g.items():
    t=[ts(e) for _,e in v]; iv=[round(b-a,2) for a,b in zip(t,t[1:])]
    w=[e['timings']['wait'] for _,e in v]; tot=[e['time'] for _,e in v]
    bodies=collections.Counter()
    for i,e in v:
        c=e['response']['content'];x=c.get('text','')
        if c.get('encoding')=='base64': x=base64.b64decode(x).decode()
        bodies[hashlib.md5(x.encode()).hexdigest()[:8]]+=1
    st=collections.Counter(e['response']['status'] for _,e in v)
    print(k[0],k[1],'n=',len(v),'idx',[i for i,_ in v][:50])
    print('   status',dict(st),'bodies',dict(bodies))
    print('   wait ms min/med/max %.0f/%.0f/%.0f  total min/med/max %.0f/%.0f/%.0f'%(min(w),statistics.median(w),max(w),min(tot),statistics.median(tot),max(tot)))
    if iv: print('   intervals',iv)
    i,e=v[0];c=e['response']['content'];x=c.get('text','')
    fn=out+'/'+k[0]+'_'+k[1].replace('/api/','').replace('/','__')+'.json'
    open(fn,'w').write(x)
