import time
import sys
for i in xrange(5):
	time.sleep(1)
	print('initial')

sys.stdout.flush()

print(input('Number from stdin: '))
for i in xrange(10):
	time.sleep(1)
	print('ok')
time.sleep(1)
print(input('Number from stdin: '))
