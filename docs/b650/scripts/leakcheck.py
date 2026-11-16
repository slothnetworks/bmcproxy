import json,re,sys
H='/opt/sparked/asrockrack-api/harfiles/'
pats=set()
for f in ['b650d4u.har','b650d4u-2t.har']:
    E=json.load(open(H+f))['log']['entries']
    for e in E:
        pd=e['request'].get('postData') or {}
        for p in pd.get('params') or []:
            if p['name']=='password': pats.add(p['value'])
        t=e['response']['content'].get('text') or ''
        if '/api/' in e['request']['url'] and t[:1] in '[{':
            pats.update(m.group() for m in re.finditer(r'\b[0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}\b',t))
            pats.update(re.findall(r'AMI[0-9A-F]{12}',t)); pats.update(re.findall(r'tos_\d+_\d+',t))
            pats.update(re.findall(r'"SerialNumber": "([0-9A-Za-z]{6,})"',t))
            pats.update(re.findall(r'"CSRFToken": "([^"]+)"',t))
pats.add('82072141930'); pats.add('PRIVATE KEY-----MII'); pats.add('MIIEogIBAAKCAQEA')
pats.discard('')
docs=[open('/opt/sparked/asrockrack-api/docs/b650/'+n).read() for n in ['har-analysis.md','har-endpoints.json']]
hits=[(p,i) for p in pats for i,d in enumerate(docs) if p in d]
print(len(pats),'patterns; hits:',hits)
for i,d in enumerate(docs):
    print(i,'MAC-like',re.findall(r'\b[0-9A-Fa-f]{2}(?::[0-9A-Fa-f]{2}){5}\b',d)[:5],'racsession numeric',re.findall(r'"racsession_id": \d+',d))
