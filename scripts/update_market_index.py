# -*- coding: utf-8 -*-
import json

ROOT = r'D:\dian115-SL\dian115-autosignin'
index_p = ROOT + r'\plugin-market\index.json'
gen_p = ROOT + r'\releases\market-entry.generated.json'

index = json.load(open(index_p, encoding='utf-8'))
gen = json.load(open(gen_p, encoding='utf-8'))

entry = index['plugins'][0]
entry['version'] = gen['version']
entry['description'] = gen['description']
entry['package_url'] = gen['package_url']
entry['sha256'] = gen['sha256']
entry['permissions'] = gen['permissions']

with open(index_p, 'w', encoding='utf-8', newline='\n') as f:
    json.dump(index, f, ensure_ascii=False, indent=2)
    f.write('\n')

# 校验权限逐键一致
manifest = json.load(open(ROOT + r'\manifest.template.json', encoding='utf-8'))
assert entry['permissions'] == manifest['permissions'], 'permissions mismatch vs manifest'
print('index.json updated to', gen['version'], 'permissions aligned; sha256', gen['sha256'][:16])
