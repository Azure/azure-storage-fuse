import json
import requests
import sys
from xml.dom import minidom

sasUrl = sys.argv[1]
releaseVersion = sys.argv[2].split(' ')[2]
print('Release Version: ' + releaseVersion)
if(len(releaseVersion)==0):
    print('Incorrect Release Version')
    sys.exit(1)

containerUrl = sasUrl.split('?')[0]
sasToken = sasUrl.split('?')[1]

# list latest version file in the container
listUrl = sasUrl + '&restype=container&comp=list&prefix=latest/'
resp = requests.get(listUrl)
sys.exit(1) if(resp.status_code<200 or resp.status_code>202) else print('Listed latest version container')
listData = minidom.parseString(resp.content)
name = listData.getElementsByTagName('Name')
if(len(name)!=1):
    print('Latest version container is empty')
    sys.exit(1)
latestVersion = name[0].firstChild.data
print('Last release version: ' + latestVersion)

# delete latest version file in the container
deleteUrl = containerUrl + '/' + latestVersion + '?' + sasToken
resp = requests.delete(deleteUrl)
sys.exit(1) if(resp.status_code<200 or resp.status_code>202) else print('Deleted last release file')

# create release version file in the container
createUrl = containerUrl + '/latest/' + releaseVersion + '?' + sasToken
resp = requests.put(createUrl, headers={'x-ms-blob-type': 'BlockBlob'})
sys.exit(1) if(resp.status_code<200 or resp.status_code>202) else print('Created new release version file')