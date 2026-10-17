import zipfile, hashlib, json
p = r'D:\dian115-SL\dian115-autosignin\releases\community.dian115-autosignin-1.0.0.d115p'
z = zipfile.ZipFile(p)
names = z.namelist()
print('entries:', len(names))
for n in names:
    print('  ', n, z.getinfo(n).file_size)
integrity = json.loads(z.read('integrity.json'))
manifest = json.loads(z.read('manifest.json'))
signature = json.loads(z.read('signature.json'))
ok = True
for f in integrity['files']:
    data = z.read(f['path'])
    if hashlib.sha256(data).hexdigest() != f['sha256']:
        print('MISMATCH', f['path']); ok = False
print('integrity files check:', 'PASS' if ok else 'FAIL')
print('manifest publisher key_id:', manifest['publisher']['key_id'])
print('signature key_id:', signature['key_id'], 'alg:', signature['algorithm'], 'domain:', signature['domain'])
print('key match:', manifest['publisher']['key_id'] == signature['key_id'])
