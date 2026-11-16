#!/usr/bin/env python3
"""Minimal JFFS2 (little-endian) extractor: dirents + inodes, none/zero/rtime/zlib/lzma."""
import sys, os, struct, zlib, lzma, stat

def rtime_decompress(src, dlen):
    out = bytearray(); pos = [0] * 256; i = 0
    while len(out) < dlen and i + 1 < len(src):
        val = src[i]; rep = src[i + 1]; i += 2
        out.append(val); backoff = pos[val]; pos[val] = len(out)
        for _ in range(rep):
            out.append(out[backoff]); backoff += 1
    return bytes(out[:dlen])

def main(img, outdir):
    d = open(img, 'rb').read()
    dirents = {}   # (pino,name) -> (version, ino, type)
    inodes = {}    # ino -> list of (version, mode, isize, offset, data)
    i = 0; bad = 0
    while i < len(d) - 12:
        if d[i:i+2] != b'\x85\x19':
            i += 4; continue
        nt, ln = struct.unpack_from('<HI', d, i + 2)
        if ln < 12 or i + ln > len(d):
            i += 4; continue
        if nt == 0xe001:
            pino, ver, ino, mct, nsize, typ = struct.unpack_from('<IIIIBB', d, i + 12)
            name = d[i+40:i+40+nsize].decode('utf-8', 'replace')
            k = (pino, name)
            if k not in dirents or dirents[k][0] < ver:
                dirents[k] = (ver, ino, typ)
        elif nt == 0xe002:
            (ino, ver, mode, uid, gid, isize, at, ct, mt, off, csize, dsize,
             compr, ucompr, flags) = struct.unpack_from('<IIIHHIIIIIIIBBH', d, i + 12)
            raw = d[i+68:i+68+csize]
            try:
                if compr == 0: data = raw
                elif compr == 1: data = b'\0' * dsize
                elif compr == 2: data = rtime_decompress(raw, dsize)
                elif compr == 6: data = zlib.decompress(raw)
                elif compr == 8: data = lzma.decompress(raw, format=lzma.FORMAT_RAW,
                                                       filters=[{'id': lzma.FILTER_LZMA1}])
                else: data = None; bad += 1
            except Exception:
                data = None; bad += 1
            inodes.setdefault(ino, []).append((ver, mode, isize, off, data, mt))
        i += (ln + 3) & ~3
    # build paths
    children = {}
    for (pino, name), (ver, ino, typ) in dirents.items():
        if ino: children.setdefault(pino, []).append((name, ino))
    def meta(ino):
        nodes = sorted(inodes.get(ino, []), key=lambda n: n[0])
        return nodes
    count = {'files': 0, 'dirs': 0, 'links': 0, 'other': 0}
    def walk(pino, path, depth=0):
        if depth > 64: return
        for name, ino in children.get(pino, []):
            if '/' in name or name in ('.', '..'): continue
            p = os.path.join(path, name)
            nodes = meta(ino)
            mode = nodes[-1][1] if nodes else stat.S_IFDIR | 0o755
            if stat.S_ISDIR(mode):
                os.makedirs(p, exist_ok=True); count['dirs'] += 1; walk(ino, p, depth + 1)
            elif stat.S_ISLNK(mode):
                tgt = b''.join(n[4] or b'' for n in nodes[-1:]).decode('utf-8', 'replace')
                if os.path.lexists(p): os.remove(p)
                os.symlink(tgt, p); count['links'] += 1
            elif stat.S_ISREG(mode):
                isize = nodes[-1][2]; buf = bytearray(isize)
                for ver, m, sz, off, data, mt in nodes:
                    if data is not None:
                        buf[off:off+len(data)] = data[:max(0, isize - off)]
                with open(p, 'wb') as f: f.write(bytes(buf[:isize]))
                os.chmod(p, stat.S_IMODE(mode) | 0o600)
                os.utime(p, (nodes[-1][5], nodes[-1][5])); count['files'] += 1
            else:
                count['other'] += 1
    os.makedirs(outdir, exist_ok=True)
    walk(1, outdir)
    print(img, count, 'undecodable nodes:', bad)

if __name__ == '__main__':
    main(sys.argv[1], sys.argv[2])
