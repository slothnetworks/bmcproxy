import os, sys, hashlib
def walk(root):
    out={}
    for dp,dn,fn in os.walk(root, followlinks=False):
        for n in fn+[d for d in dn if os.path.islink(os.path.join(dp,d))]:
            p=os.path.join(dp,n); r=os.path.relpath(p,root)
            if os.path.islink(p): out[r]='L:'+os.readlink(p)
            elif os.path.isfile(p):
                h=hashlib.md5()
                with open(p,'rb') as f:
                    for b in iter(lambda:f.read(1<<20),b''): h.update(b)
                out[r]=h.hexdigest()
    return out
a=walk(sys.argv[1]); b=walk(sys.argv[2])
for k in sorted(set(a)|set(b)):
    if k not in a: print("ADDED\t"+k)
    elif k not in b: print("REMOVED\t"+k)
    elif a[k]!=b[k]: print("CHANGED\t"+k)
