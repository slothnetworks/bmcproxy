import json,sys
from urllib.parse import urlsplit
h=json.load(open(sys.argv[1])); idx=int(sys.argv[2])
e=h['log']['entries'][idx]
t=e['response']['content'].get('text') or ''
d=json.loads(t)
mode=sys.argv[3] if len(sys.argv)>3 else 'pretty'
if mode=='pretty': print(json.dumps(d,indent=1,ensure_ascii=False))
