import json,sys,base64
h=json.load(open(sys.argv[1]))
for i,e in enumerate(h['log']['entries']):
    p=e['request']['url']
    if any(x in p for x in sys.argv[2].split(',')):
        print(i,e['startedDateTime'][11:23],p.split('/api/')[1],e['response']['content'].get('text','')[:200])
