import json,sys,collections
from urllib.parse import urlsplit
h=json.load(open(sys.argv[1]))
E=h['log']['entries']
COMMONQ={'Accept-Encoding','Accept-Language','Connection','Host','Sec-Fetch-Dest','Sec-Fetch-Mode','Sec-Fetch-Site','Sec-GPC','User-Agent','sec-ch-ua','sec-ch-ua-mobile','sec-ch-ua-platform'}
# compute response header sets across api
rh=collections.Counter()
api=[(i,e) for i,e in enumerate(E) if urlsplit(e['request']['url']).path.startswith('/api')]
for i,e in api:
    rh[tuple(sorted(x['name'] for x in e['response']['headers']))]+=1
print('API response header name-sets:'); [print(' ',c,k) for k,c in rh.items()]
qh=collections.Counter()
for i,e in api:
    qh[tuple(sorted(x['name'] for x in e['request']['headers']))]+=1
print('API request header name-sets:'); [print(' ',c,k) for k,c in qh.items()]
full=set(sys.argv[2:])
for i,e in api:
    rq=e['request'];rs=e['response'];u=urlsplit(rq['url'])
    q={x['name']:x['value'] for x in rq['headers'] if x['name'] not in COMMONQ}
    r={x['name']:x['value'] for x in rs['headers'] if x['name'] not in('Cache-Control','Content-Security-Policy','Content-Type','Pragma','Referrer-Policy','Server','Strict-Transport-Security','Transfer-Encoding','X-Content-Type-Options','X-Frame-Options','X-XSS-Protection')}
    t=rs['content'].get('text') or ''
    print('--',i,e['startedDateTime'][11:23],rq['method'],u.path,u.query,rs['status'],'w=%.0f'%e['timings']['wait'],'blk=%.0f'%e['timings']['blocked'],'tot=%.0f'%e['time'],'len',len(t))
    print('   Q',q); print('   R',r)
    if rq.get('postData'): print('   POST',rq['postData'].get('mimeType'),rq['postData'].get('text'))
    print('   B',t if (len(t)<2500 or u.path in full) else t[:600]+' ...')
