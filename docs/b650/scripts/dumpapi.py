import json,sys
from urllib.parse import urlsplit
h=json.load(open(sys.argv[1]))
E=h['log']['entries']
for i,e in enumerate(E):
    rq=e['request'];rs=e['response'];u=urlsplit(rq['url'])
    if not (u.path.startswith('/api') or rq['method'] not in('GET',) or rs['status']>=400 or u.path=='/' or 'bios' in u.path): continue
    print('='*20,i,e['startedDateTime'],rq['method'],u.path,u.query,'->',rs['status'],rs.get('statusText'))
    print('timings',e['timings'],'time',e['time'],'conn',e.get('connection'),'serverIP',e.get('serverIPAddress'))
    print('REQ HDRS:'); [print('  ',x['name'],':',x['value']) for x in rq['headers']]
    if rq.get('postData'): print('POSTDATA:',json.dumps(rq['postData']))
    print('RESP HDRS:'); [print('  ',x['name'],':',x['value']) for x in rs['headers']]
    c=rs['content']; t=c.get('text','')
    print('BODY(%s,%s,enc=%s,len=%d):'%(c.get('size'),c.get('mimeType'),c.get('encoding'),len(t or '')))
    if u.path in('/api/sensors','/api/sdr','/api/logs/eventlog','/api/settings/users','/api/logs/audit','/api/logs/dashboardevent','/api/dashboardsensors') : print((t or '')[:1500])
    elif u.path=='/' : print((t or '')[:2000])
    else: print(t)
