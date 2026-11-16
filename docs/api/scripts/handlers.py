import re,json
doc=open('/opt/sparked/asrockrack-api/docs/firmware/x470d4u.md').read()
i=doc.index('### 5.2 Endpoints: binary strings')
j=doc.index('**In the UI only',i)
blocks=re.findall(r'```\n(.*?)```',doc[i:j],re.S)
txt=' '.join(blocks)
txt=re.sub(r'\(stored without[^)]*\)','',txt)
txt=re.sub(r'\[/\(id\)\]|/\(id\)','',txt)
# join continuation lines inside braces
txt=re.sub(r',\s*\n\s*',',',txt)
txt=re.sub(r'\(\d+ endpoints\)','',txt)
# remove whitespace inside braces
o=[];dep=0
for ch in txt:
    if ch=='{': dep+=1
    if ch=='}': dep-=1
    if dep>0 and ch.isspace(): continue
    o.append(ch)
txt=''.join(o)
def expand(s):
    a=s.find('{')
    if a<0: return [s]
    dep=0
    for k in range(a,len(s)):
        if s[k]=='{': dep+=1
        elif s[k]=='}':
            dep-=1
            if dep==0: break
    inner=s[a+1:k]; parts=[];d=0;st=0
    for q,ch in enumerate(inner):
        if ch=='{': d+=1
        elif ch=='}': d-=1
        elif ch==',' and d==0: parts.append(inner[st:q]); st=q+1
    parts.append(inner[st:])
    out=[]
    for alt in parts: out+=expand(s[:a]+alt+s[k+1:])
    return out
x=set()
for tok in txt.split():
    if not tok.startswith('/'): continue
    for e in expand(tok):
        x.add('api'+e.rstrip('/'))
b9=set()
for line in open('../b650-agent/handlers.txt'):
    if line.startswith('BIN'): continue
    for t in line.split(): b9.add(t.lstrip('/').rstrip('/'))
b7=set(p for p in b9 if not re.match(r'api/asrr/maintenance/(CPLD|FPGA|RETIMER)/',p))|{'api/asrr/PowerSourceID','api/logs/platform'}
json.dump({'x470':sorted(x),'b650_905':sorted(b9),'b650_702':sorted(b7)},open('handlers.json','w'),indent=0)
print(len(x),len(b9),len(b7)); print([p for p in sorted(x) if '{' in p or '}' in p])
