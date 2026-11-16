import json,sys,collections
h=json.load(open(sys.argv[1]))
c=collections.Counter();cc=collections.Counter();rq=collections.Counter();conn=collections.Counter()
for i,e in enumerate(h['log']['entries']):
    if '10.0.0.87' not in e['request']['url']: continue
    api='/api/' in e['request']['url']
    for x in e['response']['headers']:
        n=x['name'].lower(); v=x['value'] if n not in ('date','content-length','etag','last-modified') else '*'
        c[(api,n,v)]+=1
    for x in e['request']['headers']:
        if api: rq[x['name']]+=1
    conn[(api,e.get('connection'))]+=1
    t=e['timings']
    if t['connect']>0: cc[(i,round(t['connect']),round(t['ssl']))]+=1
for k,v in sorted(c.items()): print(v,k)
print('req hdrs',rq)
print('conn ids',conn)
print('new connections (idx,connect,ssl)',list(cc))
