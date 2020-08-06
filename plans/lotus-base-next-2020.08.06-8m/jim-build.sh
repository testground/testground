#! /bin/bash

. ~/.profile

set +x

make clean
FFI_BUILD_FROM_SOURCE=1 make debug bench
