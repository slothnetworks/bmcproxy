import json,sys,collections
h=json.load(open(sys.argv[1]))
log=h['log']
print(log.get('version'),log.get('creator'),log.get('browser'))
print('pages',[ (p.get('id'),p.get('title'),p.get('startedDateTime')) for p in log.get('pages',[])])
es=log['entries']
print(len(es))
c=collections.Counter()
for i,e in enumerate(es):
    r=e['request'];s=e['response']
    u=r['url']
    print(i,e['startedDateTime'],r['method'],u[:120],s['status'],s['content'].get('mimeType'),s['content'].get('size'),round(e['time']),r.get('httpVersion'),s.get('httpVersion'))
