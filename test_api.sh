#!/bin/bash
curl -s -X POST http://ts-panel-controller:8080/api/instances/checkout \
  -H 'Content-Type: application/json' \
  -H 'X-Admin-Token: test-token-456' \
  -d '{"platform": "test", "platform_user": "tester", "slots": 10}'
