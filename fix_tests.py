#!/usr/bin/env python3
import re

test_file = 'component/file_cache/file_cache_test.go'

with open(test_file, 'r') as f:
    content = f.read()

# Replace all assertions for cleanupOnStart
pattern = r'suite\.assert\.Equal\(suite\.fileCache\.cleanupOnStart, .*?\)'
replacement = '// Removed assertion for cleanupOnStart as it\'s now handled in mount.go'
modified_content = re.sub(pattern, replacement, content)

# Also update the config string, but keep the cleanup-on-start parameter in the YAML
# since it should be backward compatible
with open(test_file, 'w') as f:
    f.write(modified_content)

print("Updated test file successfully")
