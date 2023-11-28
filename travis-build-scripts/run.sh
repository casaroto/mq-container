#!/bin/bash

# © Copyright IBM Corporation 2019, 2020
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

if [ "$(uname -m)" = "x86_64" ] ; then export ARCH="amd64" ; else export ARCH=$(uname -m) ; fi

# if DOCKER_USER is set, authenticate with docker.io to mitigate rate limit (https://www.docker.com/increase-rate-limits)
if [ -n "$DOCKER_USER" ] ; then echo 'Authenticating with docker.io...' && docker login -u $DOCKER_USER -p $DOCKER_PASS docker.io ; fi

if [ "$PUSH_MANIFEST_ONLY" = true ] ; then
  echo 'Retrieving remote tagcache' && echo -en 'travis_fold:start:retrieve-tag-cache\\r'
  ./travis-build-scripts/artifact-util.sh -c ${CACHE_PATH} -u ${REPOSITORY_USER} -p ${REPOSITORY_CREDENTIAL} -f cache/${TAGCACHE_FILE} -l ./.tagcache --get
  echo -en 'travis_fold:end:retrieve-tag-cache\\r'
  make push-manifest
  ./travis-build-scripts/cleanup-cache.sh
  exit 0
fi

echo 'Downgrading Docker (if necessary)...' && echo -en 'travis_fold:start:docker-downgrade\\r'
eval "$DOCKER_DOWNGRADE"
echo -en 'travis_fold:end:docker-downgrade\\r'

## Build images
./travis-build-scripts/build.sh

## Test images
./travis-build-scripts/test.sh

## Push images
if [ -z "$BUILD_INTERNAL_LEVEL" ] ; then
  if [ "$BUILD_ALL" = true ] ; then
    ./travis-build-scripts/push.sh developer
    ./travis-build-scripts/push.sh production
  fi
else
  if [[ "$BUILD_INTERNAL_LEVEL" == *".DE"* ]]; then
    ./travis-build-scripts/push.sh developer
  else
    ./travis-build-scripts/push.sh production
  fi
fi

if [ "$LTS" = true ] ; then
    printf '\nIn CD stream but building LTS image. Do not push LTS image to artifactory\n'
fi
