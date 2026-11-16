def pr(c): return 32<=c<127 or c in (9,10,13)
def scan(data):
    i=0;L=len(data);out=[]
    while i<L:
        b=data[i];j=i+1;v=b&0x7f;sh=7
        while b&0x80 and j<L:
            b=data[j];v|=(b&0x7f)<<sh;sh+=7;j+=1
        n=v-5
        if 2<=n<=20000 and j+n<=L and all(pr(c) for c in data[j:j+n]):
            out.append(data[j:j+n].decode());i=j+n
        else:
            i+=1
    return out
def sfile(p): return scan(open(p,'rb').read())
