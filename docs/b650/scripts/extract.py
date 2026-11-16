import json,sys,base64
from urllib.parse import urlsplit
h=json.load(open(sys.argv[1]))
for i,e in enumerate(h['log']['entries']):
    p=urlsplit(e['request']['url']).path
    if p=='/source.min.js':
        c=e['response']['content'];t=c.get('text','')
        b=base64.b64decode(t) if c.get('encoding')=='base64' else t.encode()
        open(sys.argv[2],'wb').write(b); print(i,len(b),c.get('encoding'))
