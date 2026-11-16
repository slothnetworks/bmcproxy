import json,glob,sys
R={}
for f in sorted(glob.glob('out*.jsonl')):
    for l in open(f):
        r=json.loads(l)
        if r.get('id','').startswith('_'): continue
        k=(r['method'],r['path'],json.dumps(r.get('params'),sort_keys=True) if r.get('params') else '', r.get('noauth',False))
        old=R.get(k)
        # prefer a non-401 result
        if old is None or old.get('status')==401 or r.get('status')!=401:
            r['_file']=f; R[k]=r
json.dump(list(R.values()),open('results.json','w'),indent=0)
print(len(R))
