# Length-aware `strings`: finds runs of printable bytes preceded by a ULEB128 length (n+5), as LuaJIT stores string constants.
import sys
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
for f in sys.argv[1:]:
    for s in scan(open(f,'rb').read()): print(s.replace('\n','\\n'))
