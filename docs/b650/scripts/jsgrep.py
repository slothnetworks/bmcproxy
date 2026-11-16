import re,sys
f=sys.argv[1]; w=int(sys.argv[2]); n=int(sys.argv[3])
s=open(f,encoding='utf-8',errors='replace').read()
for pat in sys.argv[4:]:
    ms=[m.start() for m in re.finditer(pat,s)]
    print('########',pat,len(ms))
    last=-10**9
    k=0
    for p in ms:
        if p-last<w: continue
        last=p;k+=1
        if k>n: break
        t=re.sub(r'[A-Za-z0-9+/=]{60,}','<B64>',s[max(0,p-w):p+w])
        print('--- @',p); print(t)
