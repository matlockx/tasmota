#!/bin/bash

docker build --platform linux/arm/v8 -t matlockx/tasmota .
docker push matlockx/tasmota

