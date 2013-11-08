#! /usr/bin/env python

import datetime
import json

strptime = datetime.datetime.strptime

def parse_record(line):
    doc = json.loads(line)
    if not doc:
    	return

    doc["Cmdline"] = doc["Cmdline"].split("\x00")[:-1]
    if "When" in doc:
	    doc["When"] = strptime(doc["When"][:-4], "%Y-%m-%dT%H:%M:%S.%f")
    return doc

with open("paccountant.log") as fd:
    for line in fd:
        record = parse_record(line)
        pp(record)
