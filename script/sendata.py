import json
import sys
import requests

if __name__ == '__main__':
    data = {}
    with open('out.json') as data_file:
        data = json.load(data_file)

    url = 'http://localhost:8080/transactions'
    for d in data:

        # print json.dumps(d)
        headers = {'Content-Type': 'application/json'}
        response = requests.post(url, data=json.dumps(d), headers=headers)
        print response
