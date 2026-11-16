import json,sys
from urllib.parse import urlsplit
h=json.load(open(sys.argv[1]))
log=h['log']
print('version',log.get('version'),'creator',log.get('creator'),'browser',log.get('browser'))
print('pages',[(p.get('id'),p.get('title'),p.get('startedDateTime')) for p in log.get('pages',[])])
E=log['entries']
print('entries',len(E))
for i,e in enumerate(E):
    rq=e['request'];rs=e['response'];u=urlsplit(rq['url'])
    t=e.get('timings',{})
    print(i,e['startedDateTime'][11:23],rq['method'],u.netloc if not u.netloc.startswith('10.') else '',u.path,('?'+u.query) if u.query else '',rs['status'],rs['httpVersion'],rq['httpVersion'],(rs['content'].get('mimeType') or '')[:24],rs['content'].get('size'),'w=%d'%t.get('wait',-1),'tot=%d'%e.get('time',-1),'conn',e.get('connection'),e.get('_resourceType',''),'cookies',len(rq.get('cookies',[])),len(rs.get('cookies',[])))
