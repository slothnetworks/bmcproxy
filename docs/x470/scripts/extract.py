import json,sys,base64
h=json.load(open(sys.argv[1]))
e=h['log']['entries'][int(sys.argv[2])]
c=e['response']['content'];t=c.get('text','')
d=base64.b64decode(t) if c.get('encoding')=='base64' else t.encode()
open(sys.argv[3],'wb').write(d)
