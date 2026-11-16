import sys
def pretty(s):
    out=[];ind=0;i=0;n=len(s);q=None
    while i<n:
        c=s[i]
        if q:
            out.append(c)
            if c=='\\' and i+1<n: out.append(s[i+1]); i+=2; continue
            if c==q: q=None
            i+=1; continue
        if c in '"\'': q=c; out.append(c); i+=1; continue
        if c=='{':
            ind+=1; out.append('{\n'+'  '*ind)
        elif c=='}':
            ind=max(0,ind-1); out.append('\n'+'  '*ind+'}')
        elif c==';':
            out.append(';\n'+'  '*ind)
        else: out.append(c)
        i+=1
    return ''.join(out)
for p in sys.argv[1:]:
    s=open(p,encoding='utf-8',errors='replace').read()
    open(p[:-3]+'.pretty.js','w').write(pretty(s))
