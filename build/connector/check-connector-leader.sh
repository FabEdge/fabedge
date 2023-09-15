#!/usr/bin/env sh
is_leader=`curl -s http://127.0.0.1:30306/is-leader`
exit_code=$?

if [ $exit_code != 0 ]; then
  exit $exit_code
fi

if [ $is_leader == "true" ]; then
    exit 0
else
    exit 1
fi
