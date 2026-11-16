import sys, struct
f=sys.argv[1]; off=int(sys.argv[2],0); maxdepth=int(sys.argv[3]) if len(sys.argv)>3 else 99
d=open(f,'rb').read()
magic,totalsize,off_struct,off_strings,off_rsv,ver,lcv,bcpu,size_str,size_struct=struct.unpack_from('>10I',d,off)
assert magic==0xd00dfeed, hex(magic)
print(f"# FDT @{off:#x} totalsize={totalsize:#x} version={ver}")
b=d[off:off+totalsize]
strs=b[off_strings:off_strings+size_str]
def s(o): return strs[o:strs.index(b'\0',o)].decode()
p=off_struct; depth=0
def fmt(name,val):
    if len(val)>64 and name in('data',): return f"<{len(val)} bytes>"
    if len(val)>256: return f"<{len(val)} bytes>"
    if val and val[-1]==0 and all(32<=c<127 or c==0 for c in val[:-1]) and val[0]!=0:
        return repr([x.decode() for x in val[:-1].split(b'\0')])
    if len(val)%4==0 and val: return ' '.join(hex(x) for x in struct.unpack(f'>{len(val)//4}I',val))
    return val.hex()
while True:
    tok=struct.unpack_from('>I',b,p)[0]; p+=4
    if tok==1:
        e=b.index(b'\0',p); name=b[p:e].decode(); p=(e+4)&~3
        if depth<=maxdepth: print('  '*depth+(name or '/')+' {')
        depth+=1
    elif tok==2:
        depth-=1
        if depth<=maxdepth: print('  '*depth+'}')
    elif tok==3:
        ln,no=struct.unpack_from('>II',b,p); p+=8
        val=b[p:p+ln]; p=(p+ln+3)&~3
        if depth<=maxdepth: print('  '*depth+f"{s(no)} = {fmt(s(no),val)}")
    elif tok==4: continue
    elif tok==9: break
