import sys,glob,re
d=sys.argv[1]; pats=sys.argv[2:]
files=[f for f in glob.glob(d+'/*.js') if not f.endswith('.pretty.js')]
for p in pats:
    print('===',p)
    n=0
    for f in sorted(files):
        s=open(f,encoding='utf-8',errors='replace').read()
        for m in re.finditer(re.escape(p),s):
            print('  [%s]'%f.split('/')[-1][:-3], s[max(0,m.start()-150):m.start()+350].replace('\n',' '))
            n+=1
            if n>=2: break
        if n>=2: break
