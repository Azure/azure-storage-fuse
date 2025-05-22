#!/usr/bin/env python3
import re

test_file = 'cmd/cleanup_test.go'

with open(test_file, 'r') as f:
    content = f.read()

# Replace all calls to cleanupCachePath with CleanupCachePath
pattern = r'cleanupCachePath\('
replacement = 'CleanupCachePath('
modified_content = re.sub(pattern, replacement, content)

with open(test_file, 'w') as f:
    f.write(modified_content)

print("Updated test file successfully")
