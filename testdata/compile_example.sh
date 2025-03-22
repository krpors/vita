#!/usr/bin/env bash

json=$(python3 -c 'import json; f = open("entry.json"); x = json.load(f); print(json.dumps(x))')
# echo $json
typst compile ./template1/example.typ --input data="${json}"
