import re, sys, os, json, glob
STR = re.compile(r'"(?:[^"\\]|\\.)*"|\'(?:[^\'\\]|\\.)*\'')
def scan_strings(s):
    spans=[];i=0;n=len(s);last=''
    while i<n:
        c=s[i]
        if c in '"\'':
            j=i+1
            while j<n and s[j]!=c:
                if s[j]=='\\': j+=1
                j+=1
            spans.append((i,j+1)); i=j+1; last='"'; continue
        if c=='/' and (last in '(,=:[!&|?{};+-*%<>~^' or s[max(0,i-6):i].endswith('return')):
            if i+1<n and s[i+1] in '/*':
                pass
            else:
                j=i+1;cls=False
                while j<n:
                    if s[j]=='\\': j+=2; continue
                    if s[j]=='[': cls=True
                    elif s[j]==']': cls=False
                    elif s[j]=='/' and not cls: break
                    elif s[j]=='\n': break
                    j+=1
                spans.append((i,j+1)); i=j+1; last='r'; continue
        if not c.isspace(): last=c
        i+=1
    return spans
def mask(s):
    # replace string contents with X to allow brace matching
    out=list(s)
    for a,b in scan_strings(s):
        if s[a]=='/': continue
        for k in range(a+1,b-1): out[k]='X'
    return ''.join(out)
def enclosing(m, pos):
    depth=0;i=pos
    while i>=0:
        c=m[i]
        if c=='}': depth+=1
        elif c=='{':
            if depth==0: break
            depth-=1
        i-=1
    if i<0: return None
    j=pos;depth=0
    while j<len(m):
        c=m[j]
        if c=='{': depth+=1
        elif c=='}':
            if depth==0: break
            depth-=1
        j+=1
    return i,j+1
def topkeys(m, s, a, b):
    """top-level keys of object literal s[a:b] (a at '{')"""
    keys=[];depth=0;i=a+1;expect=True
    while i<b-1:
        c=m[i]
        if c in '{[(': depth+=1
        elif c in '}])': depth-=1
        elif depth==0 and c==',': expect=True
        elif depth==0 and expect:
            mm=re.match(r'\s*("?[A-Za-z0-9_$\-]+"?)\s*:', s[i:])
            if mm: keys.append(mm.group(1).strip('"')); i+=mm.end(); expect=False; continue
            expect=False
        i+=1
    return keys
if __name__=='__main__':
    d = sys.argv[1]
    res=[]
    for f in sorted(glob.glob(os.path.join(d,'*.js'))):
        if f.endswith('.pretty.js'): continue
        name=os.path.basename(f)[:-3]
        if name.startswith('text_') or name.startswith('bower'): continue
        if False and not (name.startswith('views') or name.startswith('models') or name.startswith('collection') or name in ('app','router','utils','auto_router','HTTPErrorHandler')): continue
        s=open(f,encoding='utf-8',errors='replace').read()
        # cut trailing i18n junk: stop at first '}),c("i18n' or similar? keep
        SP=scan_strings(s)
        m=mask(s)
        for a,b in SP:
            lit=s[a+1:b-1]
            if not re.search(r'(^|/)api/', lit): continue
            # build url expression: following concatenations
            k=b; expr=s[a:b]
            mm=re.match(r'(\s*\+\s*(?:"(?:[^"\\]|\\.)*"|[A-Za-z0-9_$.()\[\]"]+))+', s[k:k+200])
            if mm: expr+=mm.group(0)
            enc=enclosing(m,a)
            info={'module':name,'url':lit,'expr':expr[:200]}
            if enc:
                ea,eb=enc
                obj=s[ea:eb]; mo=m[ea:eb]
                info['keys']=topkeys(m,s,ea,eb)
                t=re.search(r'\btype:"([A-Z]+)"',mo.replace('X','') and obj)
                tt=re.findall(r'(?:type|method):"(GET|POST|PUT|DELETE|PATCH)"',obj)
                info['type']=tt
                # data object
                dm=re.search(r'\bdata:',mo)
                if dm:
                    p=ea+dm.end()
                    if s[p]=='{':
                        e2=enclosing(m,p+1)
                        info['data']=topkeys(m,s,e2[0],e2[1]) if e2 else []
                    else:
                        info['data_expr']=s[p:p+150]
                        im=re.match(r'([A-Za-z_$][A-Za-z0-9_$]*)\s*[,}]',s[p:p+40])
                        if im:
                            v=im.group(1); ve=re.escape(v)
                            pre=s[max(0,ea-2500):ea]
                            keys=[]
                            js=list(re.finditer(r'(?<![A-Za-z0-9_$.])'+ve+r'=JSON\.stringify\(([A-Za-z_$][A-Za-z0-9_$]*|\{)',pre))
                            tgt=v
                            if js:
                                g=js[-1].group(1)
                                if g=='{':
                                    keys+=re.findall(r'(?:\{|,)"?([A-Za-z0-9_]+)"?:',pre[js[-1].end()-1:js[-1].end()+600].split('})')[0])
                                else: tgt=g
                            te=re.escape(tgt)
                            keys+=re.findall(r'(?<![A-Za-z0-9_$.])'+te+r'\.([A-Za-z0-9_]+)=(?!=)',pre[-1500:])
                            keys+=re.findall(r'(?<![A-Za-z0-9_$.])'+te+r'\["([A-Za-z0-9_]+)"\]=(?!=)',pre[-1500:])
                            keys+=['multipart:'+x for x in re.findall(r'(?<![A-Za-z0-9_$.])'+te+r'\.append\("([A-Za-z0-9_\-]+)"',pre[-1500:])]
                            ob=list(re.finditer(r'(?<![A-Za-z0-9_$.])'+te+r'=\{',pre))
                            if ob:
                                seg=pre[ob[-1].end()-1:]
                                mseg=mask(seg)
                                e3=enclosing(mseg,1)
                                if e3: keys+=topkeys(mseg,seg,e3[0],e3[1])
                            if keys: info['data']=sorted(set(keys))
                for kw in ('contentType','processData','dataType'):
                    km=re.search(kw+r':([^,}]+)',obj)
                    if km: info[kw]=km.group(1)[:60]
                if len(obj)<3000: info['obj']=obj
                else: info['obj']=obj[:1500]+' ... '+obj[-500:]
            res.append(info)
    json.dump(res,open(sys.argv[2],'w'),indent=1)
    print(len(res))
    