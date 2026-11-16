import json,sys,base64
h=json.load(open(sys.argv[1]))
idx=[int(x) for x in sys.argv[2].split(',')]
lim=int(sys.argv[3]) if len(sys.argv)>3 else 3000
for i in idx:
    e=h['log']['entries'][i]
    r=e['request'];s=e['response']
    print('='*20,i,r['method'],r['url'])
    print('REQ HEADERS');[print('  ',x['name'],':',x['value']) for x in r['headers']]
    print('REQ COOKIES',r.get('cookies'))
    print('QS',r.get('queryString'))
    print('POST',r.get('postData'))
    print('STATUS',s['status'],s['statusText'])
    print('RESP HEADERS');[print('  ',x['name'],':',x['value']) for x in s['headers']]
    print('RESP COOKIES',s.get('cookies'))
    print('TIMINGS',e['timings'],'serverIP',e.get('serverIPAddress'),'conn',e.get('connection'))
    c=s['content'];t=c.get('text','')
    if c.get('encoding')=='base64': t=base64.b64decode(t).decode('utf-8','replace')
    print('BODY',t[:lim])
