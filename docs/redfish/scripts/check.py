import json,re,sys,collections
sys.argv=[sys.argv[0]]
e=json.load(open('out.json'))
src=open('gen.py').read()
ns={'json':json,'re':re}
exec(src.split('# ---------- route sets ----------')[1].split('SPECIAL =')[0].replace('load(P702), load(P905), load(PX470)',"load('p702.json'), load('p905.json'), load('px470.json')"),ns)
S905=ns['S905']
cov={re.sub(r'\{[^}]+\}','*',x['uri']) for x in e if x['firmware']['b650_905'] is True}
print('9.05 firmware shapes not in JSON:',sorted(s for s in S905 if s not in cov))
print(collections.Counter(json.dumps(x['firmware'],sort_keys=True) for x in e))
print('with actions',sum(1 for x in e if x['actions']),'oem',sum(1 for x in e if x['oem']),'schema null',[x['uri'] for x in e if not x['schema_type'] and '/Actions/' not in x['uri']][:40])
for i in (0,3,175): print(json.dumps(e[i],indent=1)[:900])
