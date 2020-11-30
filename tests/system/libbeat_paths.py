import os.path
import subprocess
import sys

# Add libbeat/tests/system to the import path.
output = subprocess.check_output(["go", "list", "-m", "-f", "{{.Path}} {{.Dir}}", "all"]).decode("utf-8")
beats_line = [line for line in output.splitlines() if line.startswith("github.com/elastic/beats/")][0]
beats_dir = beats_line.split(" ", 2)[1]
sys.path.append(os.path.join(beats_dir, 'libbeat', 'tests', 'system'))
