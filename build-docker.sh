#!/bin/bash

docker build --platform linux/arm/v7 -t matlockx/tasmota .
docker push matlockx/tasmota

