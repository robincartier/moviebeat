#!/bin/bash

make && make update && \
curl -XDELETE 'http://localhost:9200/_template/*' && \
curl -XDELETE 'http://localhost:9200/*' && \
./moviebeat -e -d "*"
