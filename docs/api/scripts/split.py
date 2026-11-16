import re, sys, os, json
src, out = sys.argv[1], sys.argv[2]
s = open(src, encoding='utf-8', errors='replace').read()
os.makedirs(out, exist_ok=True)
# module definitions: <ident>("name",[  or <ident>("name",function
pat = re.compile(r'(?<![A-Za-z0-9_$.])([A-Za-z_$][A-Za-z0-9_$]?)\("([a-zA-Z0-9_.!:-]+(?:/[a-zA-Z0-9_.!:-]*)*)",(\[|function)')
ms = list(pat.finditer(s))
names = []
for i, m in enumerate(ms):
    end = ms[i+1].start() if i+1 < len(ms) else len(s)
    name = m.group(2)
    fn = name.replace('/', '__').replace('!', '_')
    open(os.path.join(out, fn + '.js'), 'w').write(s[m.start():end])
    names.append((name, m.start(), end-m.start()))
json.dump(names, open(os.path.join(out, '_index.json'), 'w'), indent=0)
print(len(names))
