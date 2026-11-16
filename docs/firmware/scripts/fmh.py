import sys, struct
d=open(sys.argv[1],'rb').read()
print("size",len(d), hex(len(d)))
i=0
while True:
    i=d.find(b'$MODULE$',i)
    if i<0: break
    h=d[i:i+64]
    vmaj,vmin,fsize,alloc,loc=struct.unpack_from('<BBHII',h,8)
    chk=h[23]
    name=h[24:32].split(b'\0')[0]
    mmaj,mmin,mtype,mloc,msize,mflags,mload,mchk=struct.unpack_from('<BBHIIHII',h,32)
    aux=h[56:62]
    end=struct.unpack_from('<H',h,62)[0] if len(h)>=64 else 0
    print(f"@{i:#010x} fmh={vmaj}.{vmin} alloc={alloc:#x} loc={loc:#x} name={name!r} ver={mmaj}.{mmin:02d} type={mtype:#06x} mloc={mloc:#x} msize={msize:#x} flags={mflags:#x} load={mload:#x} aux={aux!r} end={end:#x}")
    i+=8
print("tail", d[0x4000000:0x4000000+64].hex())
