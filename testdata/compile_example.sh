#!/usr/bin/env bash

json=$(python3 -c 'import json; f = open("entry.json"); x = json.load(f); print(json.dumps(x))')
# echo $json
typst compile example.typ --input data="${json}"
