#!/usr/bin/env python3
"""Real CPython/zlib integration/configuration case, not an upstream bug reproduction."""
import json
import os
import pathlib
import re
import subprocess
import sys
import zlib

binary = str(pathlib.Path(sys.argv[1]).resolve())
out = pathlib.Path(sys.argv[2]).resolve()
out.mkdir(parents=True, exist_ok=True)
ext = getattr(zlib, '__file__', None)
if not ext:
    raise SystemExit('Use a CPython with a shared zlib extension, not a built-in module')
ext = str(pathlib.Path(ext).resolve())
p = subprocess.run(['ldd', ext], check=True, text=True, capture_output=True)
m = re.search(r'libz\.so\.1\s+=>\s+(\S+)', p.stdout)
assert m, p.stdout
libz = str(pathlib.Path(m[1]).resolve())
payload = b'BindWitness: short known native payload\x00\x01' * 4
code = ('import json,sys,zlib; p=' + repr(payload) + '; c=zlib.compress(p); '
        'assert zlib.decompress(c)==p; print(json.dumps({"python":sys.version,'
        '"zlib_compile":zlib.ZLIB_VERSION,"zlib_runtime":zlib.ZLIB_RUNTIME_VERSION,'
        '"payload_hex":p.hex(),"round_trip":True}))')
c = {
    'schema_version': 1, 'command': [sys.executable, '-c', code],
    'working_directory': str(out), 'environment': {}, 'deadline': '10s',
    'limits': {'stdout_bytes': 16384, 'stderr_bytes': 16384, 'trace_bytes': 2097152},
    'roots': {'system': '/'},
    'objects': {'python-zlib': {'root': 'system', 'path': ext.lstrip('/')},
                'zlib': {'root': 'system', 'path': libz.lstrip('/')},
                'python': {'root': 'system', 'path': str(pathlib.Path(sys.executable).resolve()).lstrip('/')}},
    'selectors': [{'reference': 'python-zlib', 'symbol': s, 'providers': ['zlib'], 'required': True}
                  for s in ('deflate', 'inflate')],
}
# Preserve the explicit CPython profile used by the disposable validation image.
for k in ('PYTHONHOME', 'LD_LIBRARY_PATH'):
    if k in os.environ:
        c['environment'][k] = os.environ[k]

def invoke(name, config, expected, operation='check', extra=()):
    path = out / (name + '.config.json')
    path.write_text(json.dumps(config, indent=2) + '\n')
    report = out / (name + '.json')
    p = subprocess.run([binary, operation, '--config', str(path), '--report', str(report), *extra], check=False)
    assert p.returncode == expected, (name, p.returncode)
    return report

positive = invoke('zlib-positive', c, 0)
r = json.loads(positive.read_text())
assert json.loads(r['observation']['workload']['stdout'])['round_trip']
wrong = json.loads(json.dumps(c))
for s in wrong['selectors']:
    s['providers'] = ['python']
invoke('zlib-wrong-provider', wrong, 1, extra=['--observation', str(positive)])
missing = json.loads(json.dumps(c))
missing['selectors'].append({'reference': 'python-zlib', 'symbol': 'inflateBack', 'providers': ['zlib'], 'required': True})
invoke('zlib-uncovered', missing, 2, extra=['--observation', str(positive)])

# Optional real upstream zlib installations in two roots (same source, different
# optimization): check explicit LD_LIBRARY_PATH profiles and compare logical IDs.
if len(sys.argv) == 5:
    roots = [pathlib.Path(x).resolve() for x in sys.argv[3:5]]
    reports = []
    for i, root in enumerate(roots):
        x = json.loads(json.dumps(c))
        x['roots']['z1'] = str(roots[0])
        x['roots']['z2'] = str(roots[1])
        # Pin actual artifact paths rather than symlink names.
        for j, zroot in enumerate(roots):
            relative = (zroot / 'lib/libz.so.1').resolve().relative_to(zroot)
            x['objects']['z' + str(j + 1)] = {'root': 'z' + str(j + 1), 'path': str(relative)}
        x['environment']['LD_LIBRARY_PATH'] = str(root / 'lib') + ':' + os.environ.get('LD_LIBRARY_PATH', '')
        for s in x['selectors']:
            s['providers'] = ['z' + str(i + 1)]
        reports.append(invoke('zlib-root-' + str(i + 1), x, 0))
    cc = {'schema_version': 1, 'left_roots': x['roots'], 'right_roots': x['roots'],
          'objects': x['objects'], 'selectors': x['selectors']}
    comparison = invoke('zlib-provider-change', cc, 1, 'compare', ['--left', str(reports[0]), '--right', str(reports[1])])
    assert any(f['id'] == 'PROVIDER_CHANGED' for f in json.loads(comparison.read_text())['findings'])
    # Explicitly map the two rebuilt artifacts to ONE logical provider. Hash
    # change is now an artifact change, and the binding comparison must pass.
    paired_objects = {k: v for k, v in c['objects'].items() if k != 'zlib'}
    paired_objects['built-zlib'] = {'root': 'built', 'path': 'lib/libz.so.1.3.1'}
    paired_selectors = json.loads(json.dumps(c['selectors']))
    for selector in paired_selectors:
        selector['providers'] = ['built-zlib']
    paired = {'schema_version': 1,
              'left_roots': {'system': '/', 'built': str(roots[0])},
              'right_roots': {'system': '/', 'built': str(roots[1])},
              'objects': paired_objects, 'selectors': paired_selectors}
    same = invoke('zlib-artifact-change', paired, 0, 'compare', ['--left', str(reports[0]), '--right', str(reports[1])])
    assert any(a['logical_id'] == 'built-zlib' for a in json.loads(same.read_text())['artifact_changes'])

print('CPython/zlib integration validation passed')
