import sys
s=open(sys.argv[1],encoding='utf-8',errors='replace').read()
pat=sys.argv[2];before=int(sys.argv[3]);after=int(sys.argv[4]);maxn=int(sys.argv[5]) if len(sys.argv)>5 else 3
i=0;n=0
while n<maxn:
    j=s.find(pat,i)
    if j<0: break
    print('@@',j,s[max(0,j-before):j+after].replace('\\n','\n'));print()
    i=j+1;n+=1
