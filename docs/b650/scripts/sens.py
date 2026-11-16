import json, re, collections, sys
from datetime import datetime, timezone
from urllib.parse import urlsplit
H='/opt/sparked/asrockrack-api/harfiles/'
for f in ['b650d4u.har','b650d4u-2t.har']:
    E=json.load(open(H+f))['log']['entries']
    ips=collections.Counter(); macs=collections.Counter(); hosts=collections.Counter()
    for i,e in enumerate(E):
        u=urlsplit(e['request']['url'])
        if not u.path.startswith('/api') and u.path!='/': continue
        txt=(e['response']['content'].get('text') or '')+' '+json.dumps(e['request'])+' '+json.dumps(e['response']['headers'])
        for m in re.findall(r'\b\d{1,3}(?:\.\d{1,3}){3}\b',txt): ips[m]+=1
        for m in re.findall(r'\b[0-9A-Fa-f]{2}(?::[0-9A-Fa-f]{2}){5}\b',txt): macs[m.upper()]+=1
        for m in re.findall(r'AMI[0-9A-F]{12}',txt): hosts[m]+=1
    print(f,'IPs',dict(ips)); print('  MACs',dict(macs)); print('  hosts',dict(hosts))
    # clock skew
    from email.utils import parsedate_to_datetime
    sk=[]
    for e in E:
        d=[h['value'] for h in e['response']['headers'] if h['name']=='Date']
        if d:
            c=datetime.strptime(e['startedDateTime'].replace('Z','+0000'),'%Y-%m-%dT%H:%M:%S.%f%z')
            sk.append((parsedate_to_datetime(d[0])-c).total_seconds())
    print('  skew s min/max',min(sk),max(sk))
for v in (1794781758,1325394605,1325376099,1766892740,1778524330,1794462404,1794498312):
    print(v, datetime.fromtimestamp(v,timezone.utc))
