#!/usr/bin/env python3
"""Exercise the installed binary, retaining actual reports and source contracts."""
import json
import pathlib
import subprocess
import sys

binary = str(pathlib.Path(sys.argv[1]).resolve())
out = pathlib.Path(sys.argv[2]).resolve()
repo = pathlib.Path(__file__).resolve().parents[1]
out.mkdir(parents=True, exist_ok=True)
fixtures = out / 'fixtures'
subprocess.run(['sh', str(repo / 'scripts/build-fixtures.sh'), str(fixtures)], check=True)
c = json.loads((repo / 'examples/native.json').read_text())
c['working_directory'] = str(fixtures)
c['roots']['fixtures'] = str(fixtures)

def invoke(name, config, expected, operation='check', extra=()):
    path = out / (name + '.config.json')
    path.write_text(json.dumps(config, indent=2) + '\n')
    report = out / (name + '.json')
    p = subprocess.run([binary, operation, '--config', str(path), '--report', str(report), *extra], check=False)
    assert p.returncode == expected, (name, p.returncode)
    return report

positive = invoke('native-positive', c, 0)
wrong = json.loads(json.dumps(c))
wrong['command'][1:3] = reversed(wrong['command'][1:3])
negative = invoke('native-mismatch', wrong, 1)
a = json.loads(positive.read_text())
b = json.loads(negative.read_text())
assert a['observation']['workload']['stdout'] == b['observation']['workload']['stdout'] == 'result=12\n'
assert a['observation']['workload']['exit_code'] == b['observation']['workload']['exit_code'] == 0
assert any(f['id'] == 'PROVIDER_NOT_ALLOWED' for f in b['findings'])
missing = json.loads(json.dumps(c))
missing['selectors'][0]['symbol'] = 'dormant'
invoke('native-uncovered', missing, 2)
capture = invoke('native-capture', c, 0, 'capture')
invoke('native-offline', c, 0, extra=['--observation', str(capture)])
cc = json.loads((repo / 'examples/compare.json').read_text())
cc['left_roots'] = c['roots']
cc['right_roots'] = c['roots']
invoke('native-compare', cc, 1, 'compare', ['--left', str(positive), '--right', str(negative)])
print('Native installed-binary validation passed')
