#! /bin/bash

lotus chain list --count 1 --height $1 --format "<blocks>" | sed 's/,/\n/g' | sed 's/^.*bafy/bafy/' | sed 's/:.*$//' | grep -v ' ]'
