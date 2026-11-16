import json,sys,collections
s=json.load(open(sys.argv[1]))
print(len(s)); print(json.dumps(s[0],indent=1))
keys=collections.OrderedDict()
for x in s:
    for k,v in x.items(): keys.setdefault(k,set()).add(type(v).__name__)
print(keys)
for x in s:
    print(x['id'],x['sensor_number'],repr(x['name']),x['type'],x['type_number'],x.get('unit'),x.get('unit_type') if 'unit_type' in x else '',x['reading'],x['raw_reading'],'st',x.get('sensor_state'),x.get('discrete_state'),'acc',x.get('accessible'),
      'thr',[x.get(k) for k in ('lower_non_recoverable_threshold','lower_critical_threshold','lower_non_critical_threshold','higher_non_critical_threshold','higher_critical_threshold','higher_non_recoverable_threshold')])
