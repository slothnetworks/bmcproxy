import sys,re
p=sys.argv[1]; s=open(p).read()
def rep(old,new):
    global s
    assert s.count(old)==1, old[:80]
    s=s.replace(old,new)
rep('| 1 | `/redfish` | GET | `redfish-handler.lua / odata-service-root (inferred)` |  | Version document {"v1": "/redfish/v1/"}; resource_002 is in PRIVILEGE_CHECKING_WHITELIST (key `resource_002 (inferred)`) |',
    '| 1 | `/redfish` | GET | `redfish-handler.lua` |  | Version document {"v1": "/redfish/v1/"}; literal pattern `^/redfish$`; key is in PRIVILEGE_CHECKING_WHITELIST (key `resource_127`) |')
rep('| 3 | `/redfish/... trailing "/"` | GET POST PATCH PUT DELETE | `redirect-handler.lua` |  | Strips trailing slashes and redirects (Location header); route_options for resource_001 sets redirection_location = SERVICE_PREFIX (key `resource_001 (inferred)`) |',
    '| 3 | `/redfish/... trailing "/"` | GET POST PATCH PUT DELETE | `redirect-handler.lua` |  | Pattern `^/redfish(.*)/+$`; strips trailing slashes and redirects (Location header); key is in PRIVILEGE_CHECKING_WHITELIST (key `resource_002`) |')
rep('| 4 | `/redfish/v1/` | GET |',
    '| 4 | `/redfish/v1` | GET POST PATCH PUT DELETE | `redirect-handler.lua` |  | No trailing slash; pattern built as SERVICE_PREFIX .. "$"; route_options sets redirection_location from SERVICE_PREFIX, so it redirects to the service root (methods share the redirect list; inferred) (key `resource_001`) |\n| 4 | `/redfish/v1/` | GET |')
rep('| 166 | `/redfish/v1/Registries/Redfish_1.3.0_PrivilegeRegistry.json` | GET | `redfish/registry.lua` |  | Privilege registry file; resource_127 is in PRIVILEGE_CHECKING_WHITELIST; exact pattern not recoverable from strings (key `resource_127 (inferred)`) |',
    '| 166 | `/redfish/v1/PrivilegeRegistry/Redfish_1.3.0_PrivilegeRegistry.json` |  | none registered |  | Pattern built as SERVICE_PREFIX .. "/PrivilegeRegistry/" .. PRIVILEGE_REGISTRY_JSON_FILE, but the key has no handler or allow entry, so the URI is probably unreachable (falls through to 404). The privilege registries are served under `/Registries/{RegistryFile}` instead (key `resource_130`) |')
# section 3.3 whitelist table
rep("  | `resource_002` | `/redfish` |", "  | `resource_002` | trailing-slash redirect `^/redfish(.*)/+$` |")
rep("  | `resource_127` | most likely the privilege-registry file |", "  | `resource_127` | `/redfish` (version document) |")
# section 9 path
rep("Location `fw/x470/root/usr/local/redfish/`.", "Location `firmware-dumped/X470D4U_3.02.00/rootfs/usr/local/redfish/`.")
# renumber section 2 rows
head,rest=s.split('## 2. URI inventory (B650 9.05)',1)
tbl,tail=rest.split('### 2.1',1)
n=[0]
def rn(m):
    n[0]+=1; return f'| {n[0]} |'
tbl=re.sub(r'^\| \d+ \|',rn,tbl,flags=re.M)
s=head+'## 2. URI inventory (B650 9.05)'+tbl+'### 2.1'+tail
open(p,'w').write(s); print('rows',n[0])
